package service

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"land-of-stamp-backend/apperrors"
	"land-of-stamp-backend/constants"
	"land-of-stamp-backend/db"
	"land-of-stamp-backend/gen/pb"
	"land-of-stamp-backend/gen/pb/pbconnect"
	"land-of-stamp-backend/interceptor"

	"connectrpc.com/connect"
)

// AuthService implements pbconnect.AuthServiceHandler.
type AuthService struct {
	pbconnect.UnimplementedAuthServiceHandler
}

// Logout clears the authentication cookie and logs the user out.
func (s *AuthService) Logout(ctx context.Context, _ *connect.Request[pb.LogoutRequest]) (*connect.Response[pb.StatusResponse], error) {
	slog.InfoContext(ctx, "user logged out")
	resp := connect.NewResponse(&pb.StatusResponse{Status: constants.StatusLoggedOut})
	ClearTokenCookie(resp.Header())
	return resp, nil
}

// GetMe returns the currently authenticated user's profile.
func (s *AuthService) GetMe(ctx context.Context, _ *connect.Request[pb.GetMeRequest]) (*connect.Response[pb.User], error) {
	claims := interceptor.GetUser(ctx)
	if claims == nil {
		return nil, apperrors.ErrUnauthenticated
	}

	var user db.User
	if err := db.DB.WithContext(ctx).Where("uuid = ?", claims.UserID).First(&user).Error; err != nil {
		slog.WarnContext(ctx, "getMe: user not found", "user_id", claims.UserID, "error", err)
		return nil, apperrors.ErrNotFound
	}
	return connect.NewResponse(user.ToProto()), nil
}


// ── Cookie helpers (exported for use by OAuth handlers) ──

func cookieSecure() bool {
	return os.Getenv(constants.EnvCookieSecure) == "true"
}

// SetTokenCookie sets the JWT authentication cookie on the response.
func SetTokenCookie(h http.Header, token string) {
	secure := cookieSecure()
	sameSite := http.SameSiteStrictMode
	if secure {
		sameSite = http.SameSiteNoneMode
	}
	cookie := &http.Cookie{
		Name:     constants.CookieToken,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
		MaxAge:   constants.TokenCookieMaxAge,
	}
	h.Add("Set-Cookie", cookie.String())
}

// ClearTokenCookie removes the JWT authentication cookie from the response.
func ClearTokenCookie(h http.Header) {
	secure := cookieSecure()
	sameSite := http.SameSiteStrictMode
	if secure {
		sameSite = http.SameSiteNoneMode
	}
	cookie := &http.Cookie{
		Name:     constants.CookieToken,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
		MaxAge:   -1,
	}
	h.Add("Set-Cookie", cookie.String())
}
