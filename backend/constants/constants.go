// Package constants defines shared constants used across the backend.
package constants

import "time"

// Roles used for user authorization.
const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

// Cookie names used by the application.
const (
	CookieToken      = "__token"
	CookieOAuthState = "__oauth_state"
)

// Cookie durations in seconds.
const (
	TokenCookieMaxAge      = 3 * 24 * 60 * 60 // 3 days in seconds
	OAuthStateCookieMaxAge = 300              // 5 minutes in seconds
)

// BearerPrefix is the "Bearer " prefix for the Authorization header.
const BearerPrefix = "Bearer "

// StampTokenTTL is the time-to-live for QR stamp tokens.
const StampTokenTTL = 60 * time.Second

// OAuth provider name identifiers.
const (
	ProviderGoogle = "google"
	ProviderGitHub = "github"
	ProviderApple  = "apple"
)

// Frontend paths used after OAuth redirects.
const (
	OAuthCallbackPath = "/login/oauth-callback"
	OAuthErrorPath    = "/login"
)

// Environment variable names read at startup.
const (
	EnvDBPath            = "DB_PATH"
	EnvPort              = "PORT"
	EnvJWTSecret         = "JWT_SECRET"
	EnvCookieSecure      = "COOKIE_SECURE"
	EnvFrontendURL       = "FRONTEND_URL"
	EnvOAuthRedirectBase = "OAUTH_REDIRECT_BASE"
	EnvTestSeed          = "TEST_SEED"
	EnvLogLevel          = "LOG_LEVEL"
	EnvDistDir           = "DIST_DIR"
	EnvGoogleClientID    = "GOOGLE_CLIENT_ID"
	EnvGoogleSecret      = "GOOGLE_CLIENT_SECRET"
	EnvGitHubClientID    = "GITHUB_CLIENT_ID"
	EnvGitHubSecret      = "GITHUB_CLIENT_SECRET" //nolint:gosec // G101: env var name, not a credential.
	EnvAppleClientID     = "APPLE_CLIENT_ID"
	EnvAppleSecret       = "APPLE_CLIENT_SECRET"
)

// Default fallback values when environment variables are not set.
const (
	DefaultPort          = "8080"
	DefaultDBPath        = "land-of-stamp.db"
	DefaultDistDir       = "../frontend/dist"
	DefaultOAuthRedirect = "http://localhost:8080"
	DefaultFrontendURL   = "http://localhost:5173"
)

// Status response string values.
const (
	StatusLoggedOut = "logged out"
	StatusRedeemed  = "redeemed"
)

// Shop default and validation limits.
const (
	DefaultShopColor            = "#6366f1"
	DefaultStampsRequired int32 = 8
	MinStampsRequired     int32 = 2
	MaxStampsRequired     int32 = 20
)

// JWTExpiry is the duration before a JWT token expires.
const JWTExpiry = 72 * time.Hour

// ReadHeaderTimeout is the server timeout for reading request headers.
const ReadHeaderTimeout = 10 * time.Second

// CORSMaxAge is the Access-Control-Max-Age header value in seconds.
const CORSMaxAge = "86400"

// CORS header values.
const (
	CORSAllowMethods  = "GET, POST, PUT, DELETE, OPTIONS"
	CORSAllowHeaders  = "Content-Type, Authorization, Connect-Protocol-Version, Connect-Timeout-Ms, Grpc-Timeout, X-Grpc-Web, X-User-Agent"
	CORSExposeHeaders = "Grpc-Status, Grpc-Message, Grpc-Status-Details-Bin"
)

// HeaderRequestID is the HTTP header used to propagate a unique request ID.
const HeaderRequestID = "X-Request-ID"

// OAuth provider endpoint URLs.
//
//nolint:gosec // G101: these are OAuth endpoint URLs, not hardcoded credentials.
const (
	GoogleAuthURL  = "https://accounts.google.com/o/oauth2/auth"
	GoogleTokenURL = "https://oauth2.googleapis.com/token"
	GoogleUserURL  = "https://www.googleapis.com/oauth2/v2/userinfo"
	AppleAuthURL   = "https://appleid.apple.com/auth/authorize"
	AppleTokenURL  = "https://appleid.apple.com/auth/token"
	GitHubUserURL  = "https://api.github.com/user"
)

// OAuth HTTP route patterns.
const (
	GoogleCallbackRoute = "/auth/google/callback"
	GitHubCallbackRoute = "/auth/github/callback"
	AppleCallbackRoute  = "/auth/apple/callback"
	GoogleLoginRoute    = "GET /auth/google"
	GitHubLoginRoute    = "GET /auth/github"
	AppleLoginRoute     = "GET /auth/apple"
	GoogleCallbackMux   = "GET /auth/google/callback"
	GitHubCallbackMux   = "GET /auth/github/callback"
	AppleCallbackMux    = "POST /auth/apple/callback"
)

// Random byte sizes for token and secret generation.
const (
	RandomTokenBytes = 16
	RandomStateBytes = 16
	RandomShortBytes = 3
	JWTSecretBytes   = 32
)

// MaxUsernameRetries is the maximum number of attempts when resolving username conflicts.
const MaxUsernameRetries = 5

// User-facing stamp claim messages.
const (
	MsgAlreadyScanned = "You already scanned this QR code! ✅"
	MsgCardFull       = "Your card is already full! Redeem your reward first."
	MsgStampCollected = "Stamp collected! 🎉"
	MsgCardComplete   = "Card complete! 🏆 You can now redeem your reward!"
)
