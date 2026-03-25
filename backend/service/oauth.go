// Package service — OAuth login handlers for Google, GitHub, and Apple.
//
// These are plain HTTP handlers (not ConnectRPC) because OAuth requires
// browser GET redirects that ConnectRPC POST endpoints cannot support.
package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"land-of-stamp-backend/auth"
	"land-of-stamp-backend/constants"
	"land-of-stamp-backend/db"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"gorm.io/gorm"
)

// Sentinel errors for OAuth operations.
var (
	errNoIDToken            = errors.New("no id_token in Apple token response")
	errMalformedIDToken     = errors.New("malformed Apple id_token")
	errUniqueUsernameFailed = errors.New("could not create unique username")
)

// maxAppleFormSize limits the request body size for Apple's form_post callback.
const maxAppleFormSize = 1 << 20 // 1 MB

// ── Provider endpoints ─────────────────────────────────

var googleEndpoint = oauth2.Endpoint{
	AuthURL:  constants.GoogleAuthURL,
	TokenURL: constants.GoogleTokenURL,
}

var appleEndpoint = oauth2.Endpoint{
	AuthURL:  constants.AppleAuthURL,
	TokenURL: constants.AppleTokenURL,
}

// ── OAuthService holds configs for each provider ───────

// OAuthService manages OAuth2 configurations and handlers for supported identity providers.
type OAuthService struct {
	google      *oauth2.Config
	github      *oauth2.Config
	apple       *oauth2.Config
	frontendURL string
}

// NewOAuthService reads env vars and builds OAuth configs.
// If a provider's client ID is empty the provider is disabled.
func NewOAuthService() *OAuthService {
	redirectBase := os.Getenv(constants.EnvOAuthRedirectBase)
	if redirectBase == "" {
		redirectBase = constants.DefaultOAuthRedirect
	}
	frontendURL := os.Getenv(constants.EnvFrontendURL)
	if frontendURL == "" {
		frontendURL = constants.DefaultFrontendURL
	}

	s := &OAuthService{frontendURL: frontendURL}

	if id := os.Getenv(constants.EnvGoogleClientID); id != "" {
		s.google = &oauth2.Config{
			ClientID:     id,
			ClientSecret: os.Getenv(constants.EnvGoogleSecret),
			Endpoint:     googleEndpoint,
			RedirectURL:  redirectBase + constants.GoogleCallbackRoute,
			Scopes:       []string{"openid", "email", "profile"},
		}
		slog.Info("oauth: Google provider enabled")
	}
	if id := os.Getenv(constants.EnvGitHubClientID); id != "" {
		s.github = &oauth2.Config{
			ClientID:     id,
			ClientSecret: os.Getenv(constants.EnvGitHubSecret),
			Endpoint:     github.Endpoint,
			RedirectURL:  redirectBase + constants.GitHubCallbackRoute,
			Scopes:       []string{"read:user", "user:email"},
		}
		slog.Info("oauth: GitHub provider enabled")
	}
	if id := os.Getenv(constants.EnvAppleClientID); id != "" {
		s.apple = &oauth2.Config{
			ClientID:     id,
			ClientSecret: os.Getenv(constants.EnvAppleSecret),
			Endpoint:     appleEndpoint,
			RedirectURL:  redirectBase + constants.AppleCallbackRoute,
			Scopes:       []string{"name", "email"},
		}
		slog.Info("oauth: Apple provider enabled")
	}
	return s
}

// Enabled returns which providers are configured.
func (s *OAuthService) Enabled() (google, gh, apple bool) {
	return s.google != nil, s.github != nil, s.apple != nil
}

// Register mounts the OAuth HTTP routes on the mux.
func (s *OAuthService) Register(mux *http.ServeMux) {
	if s.google != nil {
		mux.HandleFunc(constants.GoogleLoginRoute, s.handleLogin(s.google, nil))
		mux.HandleFunc(constants.GoogleCallbackMux, s.handleCallback(constants.ProviderGoogle, s.google, fetchGoogleUser))
	}
	if s.github != nil {
		mux.HandleFunc(constants.GitHubLoginRoute, s.handleLogin(s.github, nil))
		mux.HandleFunc(constants.GitHubCallbackMux, s.handleCallback(constants.ProviderGitHub, s.github, fetchGitHubUser))
	}
	if s.apple != nil {
		// Apple requires response_mode=form_post and sends callback as POST.
		mux.HandleFunc(constants.AppleLoginRoute, s.handleLogin(s.apple, []oauth2.AuthCodeOption{
			oauth2.SetAuthURLParam("response_mode", "form_post"),
		}))
		mux.HandleFunc(constants.AppleCallbackMux, s.handleAppleCallback())
	}
}

// ── Handlers ───────────────────────────────────────────

// handleLogin redirects to the provider's consent screen.
func (s *OAuthService) handleLogin(cfg *oauth2.Config, extraOpts []oauth2.AuthCodeOption) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state := randomState()
		http.SetCookie(w, &http.Cookie{
			Name:     constants.CookieOAuthState,
			Value:    state,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   constants.OAuthStateCookieMaxAge,
		})
		opts := append([]oauth2.AuthCodeOption{}, extraOpts...)
		http.Redirect(w, r, cfg.AuthCodeURL(state, opts...), http.StatusTemporaryRedirect)
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
		stateCookie, err := r.Cookie(constants.CookieOAuthState)
		if err != nil || stateCookie.Value == "" || stateCookie.Value != r.URL.Query().Get("state") {
			slog.Warn("oauth: invalid state", "provider", provider)
			http.Redirect(w, r, s.frontendURL+constants.OAuthErrorPath+"?error=oauth_state", http.StatusTemporaryRedirect)
			return
		}
		// Clear state cookie
		http.SetCookie(w, &http.Cookie{Name: constants.CookieOAuthState, MaxAge: -1, Path: "/"})

		// Exchange code for token
		code := r.URL.Query().Get("code")
		oauthToken, err := cfg.Exchange(r.Context(), code)
		if err != nil {
			slog.Error("oauth: code exchange failed", "provider", provider, "error", err)
			http.Redirect(w, r, s.frontendURL+constants.OAuthErrorPath+"?error=oauth_exchange", http.StatusTemporaryRedirect)
			return
		}

		// Fetch user info from provider
		info, err := fetch(r.Context(), oauthToken)
		if err != nil {
			slog.Error("oauth: user info fetch failed", "provider", provider, "error", err)
			http.Redirect(w, r, s.frontendURL+constants.OAuthErrorPath+"?error=oauth_userinfo", http.StatusTemporaryRedirect)
			return
		}

		// Upsert user in DB
		user, err := upsertOAuthUser(r.Context(), provider, info.ID, info.Username)
		if err != nil {
			slog.Error("oauth: upsert failed", "provider", provider, "error", err)
			http.Redirect(w, r, s.frontendURL+constants.OAuthErrorPath+"?error=oauth_db", http.StatusTemporaryRedirect)
			return
		}

		// Generate JWT
		uid := user.UUID.String()
		jwtToken, err := auth.GenerateToken(uid, user.Username, user.Role)
		if err != nil {
			slog.Error("oauth: token generation failed", "provider", provider, "error", err)
			http.Redirect(w, r, s.frontendURL+constants.OAuthErrorPath+"?error=oauth_token", http.StatusTemporaryRedirect)
			return
		}

		// Set cookie and redirect
		SetTokenCookie(w.Header(), jwtToken)
		slog.Info("oauth: user logged in", "provider", provider, "uuid", uid, "username", user.Username)
		http.Redirect(w, r, s.frontendURL+constants.OAuthCallbackPath, http.StatusTemporaryRedirect)
	}
}

// handleAppleCallback handles Apple's form_post callback (POST with form data).
// Apple sends code, state, and optionally a `user` JSON blob (first login only) as form fields.
func (s *OAuthService) handleAppleCallback() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxAppleFormSize)
		if err := r.ParseForm(); err != nil {
			slog.Error("oauth: apple form parse failed", "error", err)
			http.Redirect(w, r, s.frontendURL+constants.OAuthErrorPath+"?error=oauth_exchange", http.StatusTemporaryRedirect)
			return
		}

		// Validate state
		stateCookie, err := r.Cookie(constants.CookieOAuthState)
		if err != nil || stateCookie.Value == "" || stateCookie.Value != r.FormValue("state") {
			slog.Warn("oauth: invalid state", "provider", constants.ProviderApple)
			http.Redirect(w, r, s.frontendURL+constants.OAuthErrorPath+"?error=oauth_state", http.StatusTemporaryRedirect)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: constants.CookieOAuthState, MaxAge: -1, Path: "/"})

		// Exchange code for token
		code := r.FormValue("code")
		oauthToken, err := s.apple.Exchange(r.Context(), code)
		if err != nil {
			slog.Error("oauth: apple code exchange failed", "error", err)
			http.Redirect(w, r, s.frontendURL+constants.OAuthErrorPath+"?error=oauth_exchange", http.StatusTemporaryRedirect)
			return
		}

		// Extract user info from the ID token (JWT in token response)
		info, err := extractAppleUser(oauthToken, r.FormValue("user"))
		if err != nil {
			slog.Error("oauth: apple user info failed", "error", err)
			http.Redirect(w, r, s.frontendURL+constants.OAuthErrorPath+"?error=oauth_userinfo", http.StatusTemporaryRedirect)
			return
		}

		// Upsert user in DB
		user, err := upsertOAuthUser(r.Context(), constants.ProviderApple, info.ID, info.Username)
		if err != nil {
			slog.Error("oauth: upsert failed", "provider", constants.ProviderApple, "error", err)
			http.Redirect(w, r, s.frontendURL+constants.OAuthErrorPath+"?error=oauth_db", http.StatusTemporaryRedirect)
			return
		}

		// Generate JWT and redirect
		uid := user.UUID.String()
		jwtToken, err := auth.GenerateToken(uid, user.Username, user.Role)
		if err != nil {
			slog.Error("oauth: token generation failed", "provider", constants.ProviderApple, "error", err)
			http.Redirect(w, r, s.frontendURL+constants.OAuthErrorPath+"?error=oauth_token", http.StatusTemporaryRedirect)
			return
		}
		SetTokenCookie(w.Header(), jwtToken)
		slog.Info("oauth: user logged in", "provider", constants.ProviderApple, "uuid", uid, "username", user.Username)
		http.Redirect(w, r, s.frontendURL+constants.OAuthCallbackPath, http.StatusTemporaryRedirect)
	}
}

// ── Provider-specific user info fetchers ───────────────

func fetchGoogleUser(ctx context.Context, token *oauth2.Token) (*oauthUserInfo, error) {
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, constants.GoogleUserURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("google userinfo request create: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google userinfo request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, constants.GitHubUserURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("github user request create: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github user request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var data struct {
		Login string `json:"login"`
		Email string `json:"email"`
		ID    int    `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("github user decode: %w", err)
	}
	return &oauthUserInfo{
		ID:       strconv.Itoa(data.ID),
		Username: data.Login,
		Email:    data.Email,
	}, nil
}

// extractAppleUser parses the ID token from Apple's token response.
// Apple provides user info (name) only on the first authorization via the `user` form param.
// The `sub` (unique user ID) and email always come from the ID token JWT.
func extractAppleUser(token *oauth2.Token, userJSON string) (*oauthUserInfo, error) {
	// The ID token is a JWT in the Extra data
	idTokenRaw, ok := token.Extra("id_token").(string)
	if !ok || idTokenRaw == "" {
		return nil, errNoIDToken
	}

	// Decode JWT payload (second segment) without verification —
	// we just exchanged the code server-side so the token is trustworthy.
	parts := strings.SplitN(idTokenRaw, ".", 3)
	if len(parts) < 2 {
		return nil, errMalformedIDToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode Apple id_token payload: %w", err)
	}

	var claims struct {
		Sub   string `json:"sub"`
		Email string `json:"email"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("parse Apple id_token claims: %w", err)
	}

	// Try to get the user's name from the `user` form param (first login only)
	username := claims.Email
	if userJSON != "" {
		var userData struct {
			Name struct {
				FirstName string `json:"firstName"`
				LastName  string `json:"lastName"`
			} `json:"name"`
		}
		if err := json.Unmarshal([]byte(userJSON), &userData); err == nil {
			name := strings.TrimSpace(userData.Name.FirstName + " " + userData.Name.LastName)
			if name != "" {
				username = name
			}
		}
	}

	return &oauthUserInfo{
		ID:       claims.Sub,
		Username: username,
		Email:    claims.Email,
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
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// New OAuth user — ensure unique username
	candidateName := username
	for range constants.MaxUsernameRetries {
		user = db.User{
			UUID:          uuid.New(),
			Username:      candidateName,
			PasswordHash:  "", // no password for OAuth users
			Role:          constants.RoleUser,
			OAuthProvider: provider,
			OAuthID:       oauthID,
		}
		if err := db.DB.WithContext(ctx).Create(&user).Error; err == nil {
			return &user, nil
		}
		// Username conflict — try with suffix
		candidateName = fmt.Sprintf("%s_%s_%s", username, provider, randomShort())
	}
	return nil, fmt.Errorf("%w for %s/%s", errUniqueUsernameFailed, provider, oauthID)
}

// ── Utilities ──────────────────────────────────────────

func randomState() string {
	b := make([]byte, constants.RandomStateBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func randomShort() string {
	b := make([]byte, constants.RandomShortBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
