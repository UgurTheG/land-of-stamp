package service

import (
	"context"
	"log/slog"
	"strings"

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

func (s *ShopService) ListShops(ctx context.Context, _ *connect.Request[pb.ListShopsRequest]) (*connect.Response[pb.ShopList], error) {
	var shops []db.Shop
	if err := db.DB.WithContext(ctx).Find(&shops).Error; err != nil {
		slog.ErrorContext(ctx, "shops: failed to list", "error", err)
		return nil, connect.NewError(connect.CodeInternal, nil)
	}
	out := make([]*pb.Shop, len(shops))
	for i := range shops {
		out[i] = shops[i].ToProto()
	}
	return connect.NewResponse(&pb.ShopList{Shops: out}), nil
}

func (s *ShopService) CreateShop(ctx context.Context, req *connect.Request[pb.CreateShopRequest]) (*connect.Response[pb.Shop], error) {
	claims := interceptor.GetUser(ctx)
	if claims == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}
	msg := req.Msg

	if msg.Name == "" || msg.RewardDescription == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, nil)
	}
	if msg.StampsRequired < 2 || msg.StampsRequired > 20 {
		msg.StampsRequired = 8
	}
	if msg.Color == "" {
		msg.Color = "#6366f1"
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
			return nil, connect.NewError(connect.CodeAlreadyExists, nil)
		}
		slog.ErrorContext(ctx, "shops: create failed", "error", err)
		return nil, connect.NewError(connect.CodeInternal, nil)
	}

	slog.InfoContext(ctx, "shop created", "uuid", shop.UUID, "name", msg.Name, "owner", claims.UserID)
	return connect.NewResponse(shop.ToProto()), nil
}

func (s *ShopService) UpdateShop(ctx context.Context, req *connect.Request[pb.UpdateShopRequest]) (*connect.Response[pb.Shop], error) {
	claims := interceptor.GetUser(ctx)
	if claims == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}
	msg := req.Msg

	if msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, nil)
	}

	var shop db.Shop
	if err := db.DB.WithContext(ctx).Where("uuid = ?", msg.Id).First(&shop).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	if shop.OwnerID != claims.UserID {
		return nil, connect.NewError(connect.CodePermissionDenied, nil)
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
			return nil, connect.NewError(connect.CodeAlreadyExists, nil)
		}
		slog.ErrorContext(ctx, "shops: update failed", "uuid", msg.Id, "error", result.Error)
		return nil, connect.NewError(connect.CodeInternal, nil)
	}

	slog.InfoContext(ctx, "shop updated", "uuid", msg.Id, "name", msg.Name)
	return connect.NewResponse(&pb.Shop{
		Id: msg.Id, Name: msg.Name, Description: msg.Description,
		RewardDescription: msg.RewardDescription, StampsRequired: msg.StampsRequired,
		Color: msg.Color, OwnerId: claims.UserID,
	}), nil
}

func (s *ShopService) GetMyShops(ctx context.Context, _ *connect.Request[pb.GetMyShopsRequest]) (*connect.Response[pb.ShopList], error) {
	claims := interceptor.GetUser(ctx)
	if claims == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	var shops []db.Shop
	if err := db.DB.WithContext(ctx).Where("owner_id = ?", claims.UserID).Find(&shops).Error; err != nil {
		slog.ErrorContext(ctx, "shops: fetch my shops failed", "error", err)
		return nil, connect.NewError(connect.CodeInternal, nil)
	}
	out := make([]*pb.Shop, len(shops))
	for i := range shops {
		out[i] = shops[i].ToProto()
	}
	return connect.NewResponse(&pb.ShopList{Shops: out}), nil
}
