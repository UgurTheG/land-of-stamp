// Package constants defines shared constants used across the backend.
package constants

import "time"

// ── Roles ──────────────────────────────────────────────

const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

// ── Cookie names ───────────────────────────────────────

const (
	CookieToken      = "__token"
	CookieOAuthState = "__oauth_state"
)

// ── Cookie durations ───────────────────────────────────

const (
	TokenCookieMaxAge     = 3 * 24 * 60 * 60 // 3 days in seconds
	OAuthStateCookieMaxAge = 300              // 5 minutes in seconds
)

// ── JWT / Auth ─────────────────────────────────────────

const BearerPrefix = "Bearer "

// ── Token / stamp expiry ───────────────────────────────

const StampTokenTTL = 60 * time.Second

// ── OAuth provider names ───────────────────────────────

const (
	ProviderGoogle = "google"
	ProviderGitHub = "github"
	ProviderApple  = "apple"
)

// ── OAuth frontend redirect paths ──────────────────────

const (
	OAuthCallbackPath = "/login/oauth-callback"
	OAuthErrorPath    = "/login"
)

// ── Environment variable names ─────────────────────────

const (
	EnvDBPath            = "DB_PATH"
	EnvPort              = "PORT"
	EnvJWTSecret         = "JWT_SECRET"
	EnvCookieSecure      = "COOKIE_SECURE"
	EnvFrontendURL       = "FRONTEND_URL"
	EnvOAuthRedirectBase = "OAUTH_REDIRECT_BASE"
	EnvTestSeed          = "TEST_SEED"
	EnvGoogleClientID    = "GOOGLE_CLIENT_ID"
	EnvGoogleSecret      = "GOOGLE_CLIENT_SECRET"
	EnvGitHubClientID    = "GITHUB_CLIENT_ID"
	EnvGitHubSecret      = "GITHUB_CLIENT_SECRET"
	EnvAppleClientID     = "APPLE_CLIENT_ID"
	EnvAppleSecret       = "APPLE_CLIENT_SECRET"
)

// ── Default values ─────────────────────────────────────

const (
	DefaultPort            = "8080"
	DefaultDBPath          = "land-of-stamp.db"
	DefaultOAuthRedirect   = "http://localhost:8080"
	DefaultFrontendURL     = "http://localhost:5173"
)

// ── CORS ───────────────────────────────────────────────

const CORSMaxAge = "86400"

