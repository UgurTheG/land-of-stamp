package service

import (
	"context"
	"log/slog"
	"net/http"
	"os"

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


func (s *AuthService) Logout(ctx context.Context, _ *connect.Request[pb.LogoutRequest]) (*connect.Response[pb.StatusResponse], error) {
	slog.InfoContext(ctx, "user logged out")
	resp := connect.NewResponse(&pb.StatusResponse{Status: "logged out"})
	ClearTokenCookie(resp.Header())
	return resp, nil
}

func (s *AuthService) GetMe(ctx context.Context, _ *connect.Request[pb.GetMeRequest]) (*connect.Response[pb.User], error) {
	claims := interceptor.GetUser(ctx)
	if claims == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	var user db.User
	if err := db.DB.WithContext(ctx).Where("uuid = ?", claims.UserID).First(&user).Error; err != nil {
		slog.WarnContext(ctx, "getMe: user not found", "user_id", claims.UserID, "error", err)
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	return connect.NewResponse(user.ToProto()), nil
}

// ── Cookie helpers (exported for use by OAuth handlers) ──

func cookieSecure() bool {
	return os.Getenv("COOKIE_SECURE") == "true"
}

func SetTokenCookie(h http.Header, token string) {
	secure := cookieSecure()
	sameSite := http.SameSiteStrictMode
	if secure {
		sameSite = http.SameSiteNoneMode
	}
	cookie := &http.Cookie{
		Name:     "__token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
		MaxAge:   3 * 24 * 60 * 60, // 3 days
	}
	h.Add("Set-Cookie", cookie.String())
}

func ClearTokenCookie(h http.Header) {
	secure := cookieSecure()
	sameSite := http.SameSiteStrictMode
	if secure {
		sameSite = http.SameSiteNoneMode
	}
	cookie := &http.Cookie{
		Name:     "__token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
		MaxAge:   -1,
	}
	h.Add("Set-Cookie", cookie.String())
}
