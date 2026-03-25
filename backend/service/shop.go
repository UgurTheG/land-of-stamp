package service

import (
	"context"
	"log/slog"
	"strings"

	"land-of-stamp-backend/apperrors"
	"land-of-stamp-backend/constants"
	"land-of-stamp-backend/db"
	"land-of-stamp-backend/gen/pb"
	"land-of-stamp-backend/gen/pb/pbconnect"
	"land-of-stamp-backend/interceptor"

	"connectrpc.com/connect"
	"github.com/google/uuid"
)

// ShopService implements pbconnect.ShopServiceHandler.
type ShopService struct {
	pbconnect.UnimplementedShopServiceHandler
}

// ListShops returns all shops in the system.
func (s *ShopService) ListShops(ctx context.Context, _ *connect.Request[pb.ListShopsRequest]) (*connect.Response[pb.ShopList], error) {
	var shops []db.Shop
	if err := db.DB.WithContext(ctx).Find(&shops).Error; err != nil {
		slog.ErrorContext(ctx, "shops: failed to list", "error", err)
		return nil, apperrors.ErrInternal
	}
	out := make([]*pb.Shop, len(shops))
	for i := range shops {
		out[i] = shops[i].ToProto()
	}
	return connect.NewResponse(&pb.ShopList{Shops: out}), nil
}

// CreateShop creates a new shop owned by the authenticated admin.
func (s *ShopService) CreateShop(ctx context.Context, req *connect.Request[pb.CreateShopRequest]) (*connect.Response[pb.Shop], error) {
	claims := interceptor.GetUser(ctx)
	if claims == nil {
		return nil, apperrors.ErrUnauthenticated
	}
	msg := req.Msg

	if msg.Name == "" || msg.RewardDescription == "" {
		return nil, apperrors.ErrInvalidArgument
	}
	if msg.StampsRequired < constants.MinStampsRequired || msg.StampsRequired > constants.MaxStampsRequired {
		msg.StampsRequired = constants.DefaultStampsRequired
	}
	if msg.Color == "" {
		msg.Color = constants.DefaultShopColor
	}

	shop := db.Shop{
		UUID:              uuid.New(),
		Name:              msg.Name,
		Description:       msg.Description,
		RewardDescription: msg.RewardDescription,
		StampsRequired:    msg.StampsRequired,
		Color:             msg.Color,
		OwnerID:           claims.UserID,
	}
	if err := db.DB.WithContext(ctx).Create(&shop).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") {
			return nil, apperrors.ErrAlreadyExists
		}
		slog.ErrorContext(ctx, "shops: create failed", "error", err)
		return nil, apperrors.ErrInternal
	}

	slog.InfoContext(ctx, "shop created", "uuid", shop.UUID, "name", msg.Name, "owner", claims.UserID)
	return connect.NewResponse(shop.ToProto()), nil
}

// UpdateShop updates an existing shop's details (owner only).
func (s *ShopService) UpdateShop(ctx context.Context, req *connect.Request[pb.UpdateShopRequest]) (*connect.Response[pb.Shop], error) {
	claims := interceptor.GetUser(ctx)
	if claims == nil {
		return nil, apperrors.ErrUnauthenticated
	}
	msg := req.Msg

	if msg.Id == "" {
		return nil, apperrors.ErrInvalidArgument
	}

	var shop db.Shop
	if err := db.DB.WithContext(ctx).Where("uuid = ?", msg.Id).First(&shop).Error; err != nil {
		return nil, apperrors.ErrNotFound
	}
	if shop.OwnerID != claims.UserID {
		return nil, apperrors.ErrPermissionDenied
	}

	result := db.DB.WithContext(ctx).
		Model(&db.Shop{}).
		Where("uuid = ?", msg.Id).
		Updates(map[string]any{
			"name":               msg.Name,
			"description":        msg.Description,
			"reward_description": msg.RewardDescription,
			"stamps_required":    msg.StampsRequired,
			"color":              msg.Color,
		})
	if result.Error != nil {
		if strings.Contains(result.Error.Error(), "UNIQUE") || strings.Contains(result.Error.Error(), "unique") {
			return nil, apperrors.ErrAlreadyExists
		}
		slog.ErrorContext(ctx, "shops: update failed", "uuid", msg.Id, "error", result.Error)
		return nil, apperrors.ErrInternal
	}

	slog.InfoContext(ctx, "shop updated", "uuid", msg.Id, "name", msg.Name)
	return connect.NewResponse(&pb.Shop{
		Id: msg.Id, Name: msg.Name, Description: msg.Description,
		RewardDescription: msg.RewardDescription, StampsRequired: msg.StampsRequired,
		Color: msg.Color, OwnerId: claims.UserID,
	}), nil
}

// GetMyShops returns all shops owned by the authenticated admin.
func (s *ShopService) GetMyShops(ctx context.Context, _ *connect.Request[pb.GetMyShopsRequest]) (*connect.Response[pb.ShopList], error) {
	claims := interceptor.GetUser(ctx)
	if claims == nil {
		return nil, apperrors.ErrUnauthenticated
	}

	var shops []db.Shop
	if err := db.DB.WithContext(ctx).Where("owner_id = ?", claims.UserID).Find(&shops).Error; err != nil {
		slog.ErrorContext(ctx, "shops: fetch my shops failed", "error", err)
		return nil, apperrors.ErrInternal
	}
	out := make([]*pb.Shop, len(shops))
	for i := range shops {
		out[i] = shops[i].ToProto()
	}
	return connect.NewResponse(&pb.ShopList{Shops: out}), nil
}
