# OAuth Authentication Guide

## Overview

Länd of Stamp supports social login via **Google** and **GitHub** OAuth 2.0. Users who sign in through OAuth are automatically created on first login. Subsequent logins match by provider + provider user ID and reuse the existing account.

OAuth users do not have a password — they authenticate exclusively through their chosen provider.

---

## Setup

### Environment Variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `GOOGLE_CLIENT_ID` | No | _(disabled)_ | Google OAuth 2.0 client ID |
| `GOOGLE_CLIENT_SECRET` | No | _(disabled)_ | Google OAuth 2.0 client secret |
| `GITHUB_CLIENT_ID` | No | _(disabled)_ | GitHub OAuth App client ID |
| `GITHUB_CLIENT_SECRET` | No | _(disabled)_ | GitHub OAuth App client secret |
| `OAUTH_REDIRECT_BASE` | No | `http://localhost:8080` | Base URL for OAuth callback endpoints (must match what's registered with the provider) |
| `FRONTEND_URL` | No | `http://localhost:5173` | Frontend URL to redirect to after successful OAuth |

Providers are **enabled individually** — set the client ID/secret pair for each provider you want to activate. If a provider's `CLIENT_ID` is empty, its routes are not registered.

### Google Setup

1. Go to [Google Cloud Console → Credentials](https://console.cloud.google.com/apis/credentials)
2. Create an **OAuth 2.0 Client ID** (Web application)
3. Add authorized redirect URI: `{OAUTH_REDIRECT_BASE}/auth/google/callback`
   - Local dev: `http://localhost:8080/auth/google/callback`
4. Copy the Client ID and Client Secret into `.env`

### GitHub Setup

1. Go to [GitHub → Settings → Developer settings → OAuth Apps](https://github.com/settings/developers)
2. Create a **New OAuth App**
3. Set the Authorization callback URL: `{OAUTH_REDIRECT_BASE}/auth/github/callback`
   - Local dev: `http://localhost:8080/auth/github/callback`
4. Copy the Client ID and Client Secret into `.env`

---

## How It Works

### Flow

```
User clicks "Continue with Google"
        │
        ▼
GET /auth/google
   → Sets __oauth_state cookie (CSRF protection)
   → 302 redirect to Google consent screen
        │
        ▼
User authorizes on Google
        │
        ▼
GET /auth/google/callback?code=...&state=...
   → Validates state cookie
   → Exchanges code for access token
   → Fetches user info from Google API
   → Upserts user in DB (find by provider+ID, or create)
   → Generates JWT, sets __token HttpOnly cookie
   → 302 redirect to {FRONTEND_URL}/login/oauth-callback
        │
        ▼
React OAuthCallbackPage
   → Calls GetMe RPC (cookie is already set)
   → Saves user to AuthContext + localStorage
   → Navigates to /dashboard or /admin
```

GitHub follows the same flow with `/auth/github` and `/auth/github/callback`.

### Endpoints

| Method | Path | Description |
|---|---|---|
| `GET` | `/auth/google` | Initiates Google OAuth flow |
| `GET` | `/auth/google/callback` | Google redirect callback |
| `GET` | `/auth/github` | Initiates GitHub OAuth flow |
| `GET` | `/auth/github/callback` | GitHub redirect callback |

> **Note:** These are plain HTTP handlers, not ConnectRPC endpoints. OAuth providers require browser redirects that ConnectRPC POST endpoints cannot support.

### User Creation

- **First login:** A new user is created with `oauth_provider={google|github}` and `oauth_id={provider's user ID}`. The username is taken from the provider profile (Google name, GitHub login). If the username is already taken, a suffix is appended (e.g. `john_github_a3f8c0`).
- **Subsequent logins:** The user is looked up by `(oauth_provider, oauth_id)` and logged in directly.
- **No password:** OAuth users have an empty `password_hash`.

### Security

- **CSRF protection:** A random `state` parameter is stored in a short-lived `__oauth_state` HttpOnly cookie (5 min TTL) and validated on callback.
- **Token exchange:** The authorization code is exchanged server-side — the client secret never reaches the browser.
- **JWT cookie:** The `__token` HttpOnly cookie is set after OAuth, so all existing auth middleware works unchanged.

---

## Local Development

```bash
# .env
GOOGLE_CLIENT_ID=your-google-client-id
GOOGLE_CLIENT_SECRET=your-google-client-secret
GITHUB_CLIENT_ID=your-github-client-id
GITHUB_CLIENT_SECRET=your-github-client-secret
OAUTH_REDIRECT_BASE=http://localhost:8080
FRONTEND_URL=http://localhost:5173
JWT_SECRET=dev-secret

# Start backend
cd backend && go run .

# Start frontend (Vite proxies /auth/* to backend)
cd frontend && npm run dev
```

---

## Production Checklist

- [ ] Set `COOKIE_SECURE=true` (requires HTTPS)
- [ ] Set `OAUTH_REDIRECT_BASE` to your production backend URL
- [ ] Set `FRONTEND_URL` to your production frontend URL
- [ ] Register the production callback URLs with Google and GitHub
- [ ] Store client secrets securely (e.g. environment variables, secrets manager)
