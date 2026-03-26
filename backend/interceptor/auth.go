// Package interceptor provides ConnectRPC interceptors for authentication and authorization.
package interceptor

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"land-of-stamp-backend/apperrors"
	"land-of-stamp-backend/auth"
	"land-of-stamp-backend/constants"
	"land-of-stamp-backend/gen/pb/pbconnect"

	"connectrpc.com/connect"
)

type ctxKey string

const userKey ctxKey = "user"

// Public procedures that require no authentication.
var publicProcedures = map[string]bool{
	pbconnect.AuthServiceLogoutProcedure:         true,
	pbconnect.ShopServiceListShopsProcedure:      true,
	pbconnect.DocsServiceGetOpenAPISpecProcedure: true,
	pbconnect.DocsServiceGetDocsPageProcedure:    true,
}

// NewAuthInterceptor returns a unary interceptor that validates JWT tokens
// from the Authorization header or __token cookie and enforces authentication.
// Per-resource authorization (e.g. shop ownership) is handled in the service layer.
func NewAuthInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			procedure := req.Spec().Procedure

			// Public endpoints need no auth.
			if publicProcedures[procedure] {
				return next(ctx, req)
			}

			// Extract token from Authorization header or cookie.
			tokenStr := extractToken(req.Header())
			if tokenStr == "" {
				slog.WarnContext(ctx, "auth: no token provided", "procedure", procedure)
				return nil, apperrors.ErrUnauthenticated
			}

			claims, err := auth.ValidateToken(tokenStr)
			if err != nil {
				slog.WarnContext(ctx, "auth: invalid token", "procedure", procedure, "error", err)
				return nil, apperrors.ErrUnauthenticated
			}


			ctx = context.WithValue(ctx, userKey, claims)
			return next(ctx, req)
		})
	}
}

// GetUser extracts the authenticated user claims from the context.
func GetUser(ctx context.Context) *auth.Claims {
	claims, _ := ctx.Value(userKey).(*auth.Claims)
	return claims
}

// extractToken reads a JWT from "Authorization: Bearer <token>" or the "__token" cookie.
func extractToken(h http.Header) string {
	if header := h.Get("Authorization"); strings.HasPrefix(header, constants.BearerPrefix) {
		return strings.TrimPrefix(header, constants.BearerPrefix)
	}
	// Parse cookies from the Cookie header.
	req := &http.Request{Header: h}
	if c, err := req.Cookie(constants.CookieToken); err == nil {
		return c.Value
	}
	return ""
}
