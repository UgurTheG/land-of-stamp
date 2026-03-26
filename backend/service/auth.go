package service

import (
	"encoding/base64"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"land-of-stamp-backend/auth"

	"land-of-stamp-backend/apperrors"
	"land-of-stamp-backend/constants"
	"land-of-stamp-backend/db"
	"land-of-stamp-backend/gen/pb"
	"land-of-stamp-backend/gen/pb/pbconnect"
	"land-of-stamp-backend/interceptor"

	"connectrpc.com/connect"
	"gorm.io/gorm"
)

const maxAvatarBytes = 2 * 1024 * 1024

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
	user, err := currentUser(ctx)
	if err != nil {
		slog.WarnContext(ctx, "getMe: user lookup failed", "error", err)
		return nil, err
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

// UpdateProfile updates editable user profile fields.
func (s *AuthService) UpdateProfile(ctx context.Context, req *connect.Request[pb.UpdateProfileRequest]) (*connect.Response[pb.User], error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	displayName := strings.TrimSpace(req.Msg.DisplayName)
	if len(displayName) > 64 {
		return nil, apperrors.ErrInvalidArgument
	}
	user.DisplayName = displayName

	if err := db.DB.WithContext(ctx).Save(user).Error; err != nil {
		slog.ErrorContext(ctx, "updateProfile: save failed", "error", err)
		return nil, apperrors.ErrInternal
	}

	return connect.NewResponse(user.ToProto()), nil
}

// UploadProfilePicture stores a profile picture as a data URL.
func (s *AuthService) UploadProfilePicture(ctx context.Context, req *connect.Request[pb.UploadProfilePictureRequest]) (*connect.Response[pb.User], error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	mimeType := strings.TrimSpace(strings.ToLower(req.Msg.MimeType))
	switch mimeType {
	case "image/png", "image/jpeg", "image/webp", "image/gif":
	default:
		return nil, apperrors.ErrInvalidArgument
	}

	data := strings.TrimSpace(req.Msg.DataBase64)
	if data == "" {
		return nil, apperrors.ErrInvalidArgument
	}
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, apperrors.ErrInvalidArgument
	}
	if len(decoded) == 0 || len(decoded) > maxAvatarBytes {
		return nil, apperrors.ErrInvalidArgument
	}

	user.AvatarURL = fmt.Sprintf("data:%s;base64,%s", mimeType, data)
	if err := db.DB.WithContext(ctx).Save(user).Error; err != nil {
		slog.ErrorContext(ctx, "uploadProfilePicture: save failed", "error", err)
		return nil, apperrors.ErrInternal
	}

	return connect.NewResponse(user.ToProto()), nil
}

// DeleteProfilePicture clears the current user's profile picture.
func (s *AuthService) DeleteProfilePicture(ctx context.Context, _ *connect.Request[pb.DeleteProfilePictureRequest]) (*connect.Response[pb.User], error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	user.AvatarURL = ""
	if err := db.DB.WithContext(ctx).Save(user).Error; err != nil {
		return nil, apperrors.ErrInternal
	}
	return connect.NewResponse(user.ToProto()), nil
}

// GetProfileStats returns simple profile metrics for the current user.
func (s *AuthService) GetProfileStats(ctx context.Context, _ *connect.Request[pb.GetProfileStatsRequest]) (*connect.Response[pb.ProfileStatsResponse], error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	uid := user.UUID.String()
	var joinedShops, activeCards, redeemedCards int64
	var totalStamps int64

	if err := db.DB.WithContext(ctx).Model(&db.StampCard{}).Where("user_id = ?", uid).Count(&joinedShops).Error; err != nil {
		return nil, apperrors.ErrInternal
	}
	if err := db.DB.WithContext(ctx).Model(&db.StampCard{}).Where("user_id = ? AND redeemed = false", uid).Count(&activeCards).Error; err != nil {
		return nil, apperrors.ErrInternal
	}
	if err := db.DB.WithContext(ctx).Model(&db.StampCard{}).Where("user_id = ? AND redeemed = true", uid).Count(&redeemedCards).Error; err != nil {
		return nil, apperrors.ErrInternal
	}
	if err := db.DB.WithContext(ctx).Model(&db.StampCard{}).Where("user_id = ?", uid).Select("COALESCE(SUM(stamps), 0)").Scan(&totalStamps).Error; err != nil {
		return nil, apperrors.ErrInternal
	}

	return connect.NewResponse(&pb.ProfileStatsResponse{
		JoinedShops:  int32(joinedShops),
		ActiveCards:  int32(activeCards),
		RedeemedCards: int32(redeemedCards),
		TotalStamps:  int32(totalStamps),
	}), nil
}

// DeleteAccount permanently removes the authenticated user and associated data.
func (s *AuthService) DeleteAccount(ctx context.Context, _ *connect.Request[pb.DeleteAccountRequest]) (*connect.Response[pb.StatusResponse], error) {
	claims := interceptor.GetUser(ctx)
	if claims == nil {
		return nil, apperrors.ErrUnauthenticated
	}
	userID := claims.UserID

	err := db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		hard := tx.Unscoped()

		var shops []db.Shop
		if err := hard.Where("owner_id = ?", userID).Find(&shops).Error; err != nil {
			return err
		}
		for _, shop := range shops {
			shopID := shop.UUID.String()
			var tokens []db.StampToken
			if err := hard.Where("shop_id = ?", shopID).Find(&tokens).Error; err != nil {
				return err
			}
			for _, t := range tokens {
				if err := hard.Where("token_id = ?", t.UUID.String()).Delete(&db.StampTokenClaim{}).Error; err != nil {
					return err
				}
			}
			if err := hard.Where("shop_id = ?", shopID).Delete(&db.StampToken{}).Error; err != nil {
				return err
			}
			if err := hard.Where("shop_id = ?", shopID).Delete(&db.StampCard{}).Error; err != nil {
				return err
			}
			if err := hard.Delete(&shop).Error; err != nil {
				return err
			}
		}

		if err := hard.Where("user_id = ?", userID).Delete(&db.StampCard{}).Error; err != nil {
			return err
		}
		if err := hard.Where("user_id = ?", userID).Delete(&db.StampTokenClaim{}).Error; err != nil {
			return err
		}
		return hard.Where("uuid = ?", userID).Delete(&db.User{}).Error
	})
	if err != nil {
		slog.ErrorContext(ctx, "deleteAccount: transaction failed", "user_id", userID, "error", err)
		return nil, apperrors.ErrInternal
	}

	resp := connect.NewResponse(&pb.StatusResponse{Status: "deleted"})
	ClearTokenCookie(resp.Header())
	return resp, nil
}

func currentUser(ctx context.Context) (*db.User, error) {
	claims := interceptor.GetUser(ctx)
	if claims == nil {
		return nil, apperrors.ErrUnauthenticated
	}

	var user db.User
	if err := db.DB.WithContext(ctx).Where("uuid = ?", claims.UserID).First(&user).Error; err != nil {
		return nil, apperrors.ErrNotFound
	}
	return &user, nil
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
