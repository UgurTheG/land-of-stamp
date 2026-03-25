package service

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"land-of-stamp-backend/auth"
	"land-of-stamp-backend/db"
	"land-of-stamp-backend/gen/pb"
	"land-of-stamp-backend/gen/pb/pbconnect"
	"land-of-stamp-backend/interceptor"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// AuthService implements pbconnect.AuthServiceHandler.
type AuthService struct {
	pbconnect.UnimplementedAuthServiceHandler
}

func (s *AuthService) Register(ctx context.Context, req *connect.Request[pb.RegisterRequest]) (*connect.Response[pb.AuthResponse], error) {
	msg := req.Msg
	if len(msg.Username) < 2 {
		return nil, connect.NewError(connect.CodeInvalidArgument, nil)
	}
	if len(msg.Password) < 4 {
		return nil, connect.NewError(connect.CodeInvalidArgument, nil)
	}

	role := msg.Role
	if role != "user" && role != "admin" {
		role = "user"
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(msg.Password), bcrypt.DefaultCost)
	if err != nil {
		slog.ErrorContext(ctx, "register: bcrypt failed", "error", err)
		return nil, connect.NewError(connect.CodeInternal, nil)
	}

	user := db.User{
		UUID:         uuid.New(),
		Username:     msg.Username,
		PasswordHash: string(hash),
		Role:         role,
	}
	if err := db.DB.WithContext(ctx).Create(&user).Error; err != nil {
		slog.InfoContext(ctx, "register: username taken", "username", msg.Username)
		return nil, connect.NewError(connect.CodeAlreadyExists, nil)
	}

	uid := user.UUID.String()
	token, err := auth.GenerateToken(uid, msg.Username, role)
	if err != nil {
		slog.ErrorContext(ctx, "register: token generation failed", "error", err)
		return nil, connect.NewError(connect.CodeInternal, nil)
	}

	slog.InfoContext(ctx, "user registered", "uuid", uid, "username", msg.Username, "role", role)
	resp := connect.NewResponse(&pb.AuthResponse{
		User: &pb.User{Id: uid, Username: msg.Username, Role: role},
	})
	SetTokenCookie(resp.Header(), token)
	return resp, nil
}

func (s *AuthService) Login(ctx context.Context, req *connect.Request[pb.LoginRequest]) (*connect.Response[pb.AuthResponse], error) {
	msg := req.Msg

	var user db.User
	if err := db.DB.WithContext(ctx).Where("username = ?", msg.Username).First(&user).Error; err != nil {
		slog.InfoContext(ctx, "login: user not found", "username", msg.Username)
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(msg.Password)); err != nil {
		slog.InfoContext(ctx, "login: wrong password", "username", msg.Username)
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	uid := user.UUID.String()
	token, err := auth.GenerateToken(uid, user.Username, user.Role)
	if err != nil {
		slog.ErrorContext(ctx, "login: token generation failed", "error", err)
		return nil, connect.NewError(connect.CodeInternal, nil)
	}

	slog.InfoContext(ctx, "user logged in", "uuid", uid, "username", user.Username, "role", user.Role)
	resp := connect.NewResponse(&pb.AuthResponse{User: user.ToProto()})
	SetTokenCookie(resp.Header(), token)
	return resp, nil
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
