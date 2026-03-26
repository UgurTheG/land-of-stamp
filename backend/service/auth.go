package service

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"land-of-stamp-backend/auth"

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

// ChooseRole allows a newly registered user to pick their role (user or admin).
// This can only be called once — subsequent calls are rejected.
func (s *AuthService) ChooseRole(ctx context.Context, req *connect.Request[pb.ChooseRoleRequest]) (*connect.Response[pb.User], error) {
	claims := interceptor.GetUser(ctx)
	if claims == nil {
		return nil, apperrors.ErrUnauthenticated
	}

	role := req.Msg.Role
	if role != constants.RoleUser && role != constants.RoleAdmin {
		return nil, apperrors.ErrInvalidArgument
	}

	var user db.User
	if err := db.DB.WithContext(ctx).Where("uuid = ?", claims.UserID).First(&user).Error; err != nil {
		return nil, apperrors.ErrNotFound
	}

	if user.RoleChosen {
		return nil, apperrors.ErrFailedPrecondition
	}

	user.Role = role
	user.RoleChosen = true
	if err := db.DB.WithContext(ctx).Save(&user).Error; err != nil {
		slog.ErrorContext(ctx, "chooseRole: save failed", "error", err)
		return nil, apperrors.ErrInternal
	}

	// Issue a new JWT with the updated role
	jwtToken, err := auth.GenerateToken(user.UUID.String(), user.Username, user.Role)
	if err != nil {
		slog.ErrorContext(ctx, "chooseRole: token generation failed", "error", err)
		return nil, apperrors.ErrInternal
	}

	slog.InfoContext(ctx, "user chose role", "uuid", user.UUID, "role", role)
	resp := connect.NewResponse(user.ToProto())
	SetTokenCookie(resp.Header(), jwtToken)
	return resp, nil
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
