package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"land-of-stamp-backend/auth"

	"github.com/google/uuid"
)

type contextKey string

const UserKey contextKey = "user"

// requestIDKey is used to store a unique request ID in the context.
const requestIDKey contextKey = "request_id"

// RequestLog logs every incoming request with method, path, status, and duration.
// It also injects a unique request_id into the request context for tracing.
func RequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := uuid.New().String()[:8]
		ctx := context.WithValue(r.Context(), requestIDKey, reqID)
		r = r.WithContext(ctx)

		w.Header().Set("X-Request-ID", reqID)

		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		slog.InfoContext(r.Context(), "request",
			"request_id", reqID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration", time.Since(start).String(),
			"remote", r.RemoteAddr,
		)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Auth extracts and validates the JWT from the Authorization header OR the __token HttpOnly cookie.
func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		var tokenStr string

		// 1. Try Authorization: Bearer header
		if header := r.Header.Get("Authorization"); strings.HasPrefix(header, "Bearer ") {
			tokenStr = strings.TrimPrefix(header, "Bearer ")
		}

		// 2. Fall back to HttpOnly cookie
		if tokenStr == "" {
			if c, err := r.Cookie("__token"); err == nil {
				tokenStr = c.Value
			}
		}

		if tokenStr == "" {
			slog.WarnContext(ctx, "auth: no token provided", "path", r.URL.Path)
			http.Error(w, `{"error":"missing or invalid authorization"}`, http.StatusUnauthorized)
			return
		}

		claims, err := auth.ValidateToken(tokenStr)
		if err != nil {
			slog.WarnContext(ctx, "auth: invalid token", "path", r.URL.Path, "error", err)
			http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
			return
		}

		ctx = context.WithValue(ctx, UserKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AdminOnly requires the authenticated user to have role "admin".
func AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		claims := GetUser(r)
		if claims == nil || claims.Role != "admin" {
			role := "none"
			if claims != nil {
				role = claims.Role
			}
			slog.WarnContext(ctx, "auth: admin access denied", "path", r.URL.Path, "role", role)
			http.Error(w, `{"error":"admin access required"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// CORS handles cross-origin requests.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "http://localhost:5173"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "86400")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func GetUser(r *http.Request) *auth.Claims {
	claims, _ := r.Context().Value(UserKey).(*auth.Claims)
	return claims
}

// GetRequestID returns the request ID from context, if set.
func GetRequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}
