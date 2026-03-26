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
 Users who sign in through OAuth are automatically created with the `user` role on first login. Subsequent logins match by provider + provider user ID and reuse the existing account.

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
| `APPLE_CLIENT_ID` | No | _(disabled)_ | Apple Services ID (e.g. `com.example.land-of-stamp`) |
| `APPLE_CLIENT_SECRET` | No | _(disabled)_ | Apple client secret JWT (see [Apple Setup](#apple-setup)) |
| `OAUTH_REDIRECT_BASE` | No | `http://localhost:8080` | Base URL for OAuth callback endpoints (must match what's registered with the provider) |
| `FRONTEND_URL` | No | `http://localhost:5173` | Frontend URL to redirect to after successful OAuth |

Providers are **enabled individually** — set the client ID/secret pair for each provider you want to activate. If a provider's `CLIENT_ID` is empty, its routes are not registered.

### Google Setup

1. Go to [Google Cloud Console → Credentials](https://console.cloud.google.com/apis/credentials)
2. Create an **OAuth 2.0 Client ID** (Web application)
3. Add authorized redirect URI: `{OAUTH_REDIRECT_BASE}/auth/google/callback`
   - Local dev: `http://localhost:8080/auth/google/callback`
4. Copy the Client ID and Client Secret into env vars

### GitHub Setup

1. Go to [GitHub → Settings → Developer settings → OAuth Apps](https://github.com/settings/developers)
2. Create a **New OAuth App**
3. Set the Authorization callback URL: `{OAUTH_REDIRECT_BASE}/auth/github/callback`
   - Local dev: `http://localhost:8080/auth/github/callback`
4. Copy the Client ID and Client Secret into env vars

### Apple Setup

Apple's Sign in with Apple is more involved than other providers.

#### Prerequisites
- An [Apple Developer Program](https://developer.apple.com/programs/) membership ($99/year)
- A registered **App ID** with "Sign in with Apple" capability
- A **Services ID** (this is your `APPLE_CLIENT_ID`)

#### Step-by-step

1. Go to [Apple Developer → Certificates, Identifiers & Profiles](https://developer.apple.com/account/resources)
2. Under **Identifiers**, create an **App ID** (if you don't have one):
   - Enable **Sign in with Apple**
3. Create a **Services ID** (type: Services IDs):
   - Set the identifier (e.g. `com.example.land-of-stamp.web`) — this becomes `APPLE_CLIENT_ID`
   - Enable **Sign in with Apple** and click **Configure**:
     - **Primary App ID**: select your App ID from step 2
     - **Domains**: your production domain (e.g. `land-of-stamp.example.com`)
     - **Return URLs**: `{OAUTH_REDIRECT_BASE}/auth/apple/callback`
4. Create a **Key** (type: Keys):
   - Enable **Sign in with Apple** and associate it with your App ID
   - Download the `.p8` private key file — **save it securely, you can only download once**
   - Note the **Key ID**
5. Generate the client secret JWT:

   Apple's client secret is a **short-lived JWT** (max 6 months) signed with your private key. Generate it with:

   ```bash
   # Using Ruby (macOS has it built-in):
   ruby -e '
     require "jwt"
     key = OpenSSL::PKey::EC.new(File.read("AuthKey_XXXXXXXXXX.p8"))
     now = Time.now.to_i
     payload = {
       iss: "YOUR_TEAM_ID",
       iat: now,
       exp: now + 86400 * 180,  # 6 months
       aud: "https://appleid.apple.com",
       sub: "YOUR_SERVICES_ID"
     }
     puts JWT.encode(payload, key, "ES256", { kid: "YOUR_KEY_ID" })
   '
   ```

   Or use the [Apple client secret generator](https://github.com/nicklockwood/iVersion/wiki/Generating-a-client-secret-for-Sign-in-with-Apple) or any JWT library.

   Set the output as `APPLE_CLIENT_SECRET`.

> **⚠️ Important:** Apple client secrets expire (max 6 months). Set a reminder to regenerate before expiry.

#### Apple-specific behavior

- **Callback is POST**: Apple sends the callback as a `POST` with `response_mode=form_post` (form data), not a GET redirect. The backend handles this automatically.
- **User info on first login only**: Apple sends the user's name (`firstName`, `lastName`) in a `user` form parameter only during the **first** authorization. On subsequent logins, only the `sub` (unique ID) and `email` come from the ID token. The backend stores the name from the first login.
- **ID token**: User info (`sub`, `email`) is extracted from the JWT ID token in the token response — no separate userinfo API call is needed.

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

Apple follows the same flow but with two differences:
- The initial redirect includes `response_mode=form_post`
- The callback is a **POST** to `/auth/apple/callback` (form data instead of query params)

### Endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/auth/google` | None | Initiates Google OAuth flow |
| `GET` | `/auth/google/callback` | None | Google redirect callback |
| `GET` | `/auth/github` | None | Initiates GitHub OAuth flow |
| `GET` | `/auth/github/callback` | None | GitHub redirect callback |
| `GET` | `/auth/apple` | None | Initiates Apple OAuth flow |
| `POST` | `/auth/apple/callback` | None | Apple form_post callback |

> **Note:** These are plain HTTP handlers, not ConnectRPC endpoints. OAuth providers require browser redirects that ConnectRPC POST endpoints cannot support.

### User Creation

- **First login:** A new user is created with `role=user`, `oauth_provider={google|github|apple}`, and `oauth_id={provider's user ID}`. The username is taken from the provider profile (Google name, GitHub login, or Apple name/email). If the username is already taken, a suffix is appended (e.g. `john_github_a3f8c0`).
- **Subsequent logins:** The user is looked up by `(oauth_provider, oauth_id)` and logged in directly.
- **No password:** OAuth users have an empty `password_hash` and cannot use the username/password login form.

### Security

- **CSRF protection:** A random `state` parameter is stored in a short-lived `__oauth_state` HttpOnly cookie (5 min TTL) and validated on callback.
- **Token exchange:** The authorization code is exchanged server-side — the client secret never reaches the browser.
- **JWT cookie:** The same `__token` HttpOnly cookie used by password auth is set after OAuth, so all existing auth middleware (ConnectRPC interceptor, etc.) works unchanged.

### Database Schema

The `users` table has two additional columns for OAuth:

| Column | Type | Default | Description |
|---|---|---|---|
| `oauth_provider` | `TEXT NOT NULL` | `''` | Provider name: `"google"`, `"github"`, `"apple"`, or `""` (password user) |
| `oauth_id` | `TEXT NOT NULL` | `''` | Provider's unique user ID |

A **partial unique index** ensures one OAuth identity maps to one user:
```sql
CREATE UNIQUE INDEX idx_oauth_provider_id
ON users (oauth_provider, oauth_id)
WHERE oauth_provider != '' AND deleted_at IS NULL
```

---

## Frontend

The login page shows "Continue with Google", "Continue with GitHub", and "Continue with Apple" buttons below the password form. These are plain `<a href="/auth/{provider}">` links — clicking them triggers a full-page redirect through the OAuth flow.

The buttons are always visible regardless of whether the provider is configured on the backend. If a provider is not configured, the route will 404.

---

## Local Development

```bash
# .env or export in shell
export GOOGLE_CLIENT_ID="your-google-client-id"
export GOOGLE_CLIENT_SECRET="your-google-client-secret"
export GITHUB_CLIENT_ID="your-github-client-id"
export GITHUB_CLIENT_SECRET="your-github-client-secret"
export APPLE_CLIENT_ID="your-apple-services-id"
export APPLE_CLIENT_SECRET="your-apple-client-secret-jwt"
export OAUTH_REDIRECT_BASE="http://localhost:8080"
export FRONTEND_URL="http://localhost:5173"
export JWT_SECRET="dev-secret"

# Start backend
cd backend && go run .

# Start frontend (Vite proxies /auth/* to backend)
cd frontend && npm run dev
```

> **Note:** Apple Sign in requires HTTPS in production and a registered domain. For local development, Apple may not work without additional tunneling (e.g. ngrok) and domain verification.

---

## Production Checklist

- [ ] Set `COOKIE_SECURE=true` (requires HTTPS)
- [ ] Set `OAUTH_REDIRECT_BASE` to your production backend URL
- [ ] Set `FRONTEND_URL` to your production frontend URL
- [ ] Register the production callback URLs with Google, GitHub, and Apple
- [ ] Store client secrets securely (e.g. environment variables, secrets manager)
- [ ] Set a calendar reminder to regenerate the Apple client secret before it expires (max 6 months)
