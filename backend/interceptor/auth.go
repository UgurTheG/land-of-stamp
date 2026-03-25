// Package interceptor provides ConnectRPC interceptors for authentication and authorization.
package interceptor

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"land-of-stamp-backend/auth"
	"land-of-stamp-backend/gen/pb/pbconnect"

	"connectrpc.com/connect"
)

type ctxKey string

const userKey ctxKey = "user"

// Public procedures that require no authentication.
var publicProcedures = map[string]bool{
	pbconnect.AuthServiceLogoutProcedure:        true,
	pbconnect.ShopServiceListShopsProcedure:     true,
	pbconnect.DocsServiceGetOpenAPISpecProcedure: true,
	pbconnect.DocsServiceGetDocsPageProcedure:    true,
}

// Admin-only procedures that require role "admin".
var adminProcedures = map[string]bool{
	pbconnect.ShopServiceCreateShopProcedure:           true,
	pbconnect.ShopServiceUpdateShopProcedure:           true,
	pbconnect.ShopServiceGetMyShopsProcedure:           true,
	pbconnect.StampServiceGetShopCardsProcedure:        true,
	pbconnect.StampServiceGrantStampProcedure:          true,
	pbconnect.StampServiceUpdateStampCountProcedure:    true,
	pbconnect.StampServiceGetShopCustomersProcedure:    true,
	pbconnect.StampServiceCreateStampTokenProcedure:    true,
	pbconnect.StampServiceGetStampTokenStatusProcedure: true,
}

// NewAuthInterceptor returns a unary interceptor that validates JWT tokens
// from the Authorization header or __token cookie and enforces role-based access.
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
				return nil, connect.NewError(connect.CodeUnauthenticated, nil)
			}

			claims, err := auth.ValidateToken(tokenStr)
			if err != nil {
				slog.WarnContext(ctx, "auth: invalid token", "procedure", procedure, "error", err)
				return nil, connect.NewError(connect.CodeUnauthenticated, nil)
			}

			// Admin-only check.
			if adminProcedures[procedure] && claims.Role != "admin" {
				slog.WarnContext(ctx, "auth: admin access denied", "procedure", procedure, "role", claims.Role)
				return nil, connect.NewError(connect.CodePermissionDenied, nil)
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
	if header := h.Get("Authorization"); strings.HasPrefix(header, "Bearer ") {
		return strings.TrimPrefix(header, "Bearer ")
	}
	// Parse cookies from the Cookie header.
	req := &http.Request{Header: h}
	if c, err := req.Cookie("__token"); err == nil {
		return c.Value
	}
	return ""
}
