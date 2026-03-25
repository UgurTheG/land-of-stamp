// Package service — OAuth login handlers for Google and GitHub.
//
// These are plain HTTP handlers (not ConnectRPC) because OAuth requires
// browser GET redirects that ConnectRPC POST endpoints cannot support.
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"land-of-stamp-backend/auth"
	"land-of-stamp-backend/db"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"gorm.io/gorm"
)

// ── Google endpoint (not in x/oauth2 by default) ───────

var googleEndpoint = oauth2.Endpoint{
	AuthURL:  "https://accounts.google.com/o/oauth2/auth",
	TokenURL: "https://oauth2.googleapis.com/token",
}

// ── OAuthService holds configs for each provider ───────

type OAuthService struct {
	google      *oauth2.Config
	github      *oauth2.Config
	frontendURL string
}

// NewOAuthService reads env vars and builds OAuth configs.
// If a provider's client ID is empty the provider is disabled.
func NewOAuthService() *OAuthService {
	redirectBase := os.Getenv("OAUTH_REDIRECT_BASE")
	if redirectBase == "" {
		redirectBase = "http://localhost:8080"
	}
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}

	s := &OAuthService{frontendURL: frontendURL}

	if id := os.Getenv("GOOGLE_CLIENT_ID"); id != "" {
		s.google = &oauth2.Config{
			ClientID:     id,
			ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
			Endpoint:     googleEndpoint,
			RedirectURL:  redirectBase + "/auth/google/callback",
			Scopes:       []string{"openid", "email", "profile"},
		}
		slog.Info("oauth: Google provider enabled")
	}
	if id := os.Getenv("GITHUB_CLIENT_ID"); id != "" {
		s.github = &oauth2.Config{
			ClientID:     id,
			ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
			Endpoint:     github.Endpoint,
			RedirectURL:  redirectBase + "/auth/github/callback",
			Scopes:       []string{"read:user", "user:email"},
		}
		slog.Info("oauth: GitHub provider enabled")
	}
	return s
}

// Enabled returns which providers are configured.
func (s *OAuthService) Enabled() (google, gh bool) {
	return s.google != nil, s.github != nil
}

// Register mounts the OAuth HTTP routes on the mux.
func (s *OAuthService) Register(mux *http.ServeMux) {
	if s.google != nil {
		mux.HandleFunc("GET /auth/google", s.handleLogin(s.google))
		mux.HandleFunc("GET /auth/google/callback", s.handleCallback("google", s.google, fetchGoogleUser))
	}
	if s.github != nil {
		mux.HandleFunc("GET /auth/github", s.handleLogin(s.github))
		mux.HandleFunc("GET /auth/github/callback", s.handleCallback("github", s.github, fetchGitHubUser))
	}
}

// ── Handlers ───────────────────────────────────────────

// handleLogin redirects to the provider's consent screen.
func (s *OAuthService) handleLogin(cfg *oauth2.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state := randomState()
		http.SetCookie(w, &http.Cookie{
			Name:     "__oauth_state",
			Value:    state,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   300, // 5 min
		})
		http.Redirect(w, r, cfg.AuthCodeURL(state), http.StatusTemporaryRedirect)
	}
}

type oauthUserInfo struct {
	ID       string
	Username string
	Email    string
}

type userFetcher func(ctx context.Context, token *oauth2.Token) (*oauthUserInfo, error)

// handleCallback exchanges the code, upserts the user, sets the JWT cookie, and redirects to the frontend.
func (s *OAuthService) handleCallback(provider string, cfg *oauth2.Config, fetch userFetcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Validate state
		stateCookie, err := r.Cookie("__oauth_state")
		if err != nil || stateCookie.Value == "" || stateCookie.Value != r.URL.Query().Get("state") {
			slog.Warn("oauth: invalid state", "provider", provider)
			http.Redirect(w, r, s.frontendURL+"/login?error=oauth_state", http.StatusTemporaryRedirect)
			return
		}
		// Clear state cookie
		http.SetCookie(w, &http.Cookie{Name: "__oauth_state", MaxAge: -1, Path: "/"})

		// Exchange code for token
		code := r.URL.Query().Get("code")
		oauthToken, err := cfg.Exchange(r.Context(), code)
		if err != nil {
			slog.Error("oauth: code exchange failed", "provider", provider, "error", err)
			http.Redirect(w, r, s.frontendURL+"/login?error=oauth_exchange", http.StatusTemporaryRedirect)
			return
		}

		// Fetch user info from provider
		info, err := fetch(r.Context(), oauthToken)
		if err != nil {
			slog.Error("oauth: user info fetch failed", "provider", provider, "error", err)
			http.Redirect(w, r, s.frontendURL+"/login?error=oauth_userinfo", http.StatusTemporaryRedirect)
			return
		}

		// Upsert user in DB
		user, err := upsertOAuthUser(r.Context(), provider, info.ID, info.Username)
		if err != nil {
			slog.Error("oauth: upsert failed", "provider", provider, "error", err)
			http.Redirect(w, r, s.frontendURL+"/login?error=oauth_db", http.StatusTemporaryRedirect)
			return
		}

		// Generate JWT
		uid := user.UUID.String()
		jwtToken, err := auth.GenerateToken(uid, user.Username, user.Role)
		if err != nil {
			slog.Error("oauth: token generation failed", "provider", provider, "error", err)
			http.Redirect(w, r, s.frontendURL+"/login?error=oauth_token", http.StatusTemporaryRedirect)
			return
		}

		// Set cookie and redirect
		SetTokenCookie(w.Header(), jwtToken)
		slog.Info("oauth: user logged in", "provider", provider, "uuid", uid, "username", user.Username)
		http.Redirect(w, r, s.frontendURL+"/login/oauth-callback", http.StatusTemporaryRedirect)
	}
}

// ── Provider-specific user info fetchers ───────────────

func fetchGoogleUser(ctx context.Context, token *oauth2.Token) (*oauthUserInfo, error) {
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("google userinfo request: %w", err)
	}
	defer resp.Body.Close()

	var data struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("google userinfo decode: %w", err)
	}
	username := data.Name
	if username == "" {
		username = data.Email
	}
	return &oauthUserInfo{ID: data.ID, Username: username, Email: data.Email}, nil
}

func fetchGitHubUser(ctx context.Context, token *oauth2.Token) (*oauthUserInfo, error) {
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, fmt.Errorf("github user request: %w", err)
	}
	defer resp.Body.Close()

	var data struct {
		ID    int    `json:"id"`
		Login string `json:"login"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("github user decode: %w", err)
	}
	return &oauthUserInfo{
		ID:       fmt.Sprintf("%d", data.ID),
		Username: data.Login,
		Email:    data.Email,
	}, nil
}

// ── DB helper ──────────────────────────────────────────

// upsertOAuthUser finds or creates a user by (oauth_provider, oauth_id).
// If the username is already taken (by a password-based user), we append the provider suffix.
func upsertOAuthUser(ctx context.Context, provider, oauthID, username string) (*db.User, error) {
	var user db.User
	err := db.DB.WithContext(ctx).
		Where("oauth_provider = ? AND oauth_id = ?", provider, oauthID).
		First(&user).Error

	if err == nil {
		return &user, nil // existing OAuth user
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	// New OAuth user — ensure unique username
	candidateName := username
	for attempt := 0; attempt < 5; attempt++ {
		user = db.User{
			UUID:          uuid.New(),
			Username:      candidateName,
			PasswordHash:  "", // no password for OAuth users
			Role:          "user",
			OAuthProvider: provider,
			OAuthID:       oauthID,
		}
		if err := db.DB.WithContext(ctx).Create(&user).Error; err == nil {
			return &user, nil
		}
		// Username conflict — try with suffix
		candidateName = fmt.Sprintf("%s_%s_%s", username, provider, randomShort())
	}
	return nil, fmt.Errorf("could not create unique username for %s/%s", provider, oauthID)
}

// ── Utilities ──────────────────────────────────────────

func randomState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func randomShort() string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

