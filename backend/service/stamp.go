package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"time"

	"land-of-stamp-backend/auth"
	"land-of-stamp-backend/db"
	"land-of-stamp-backend/gen/pb"
	"land-of-stamp-backend/gen/pb/pbconnect"
	"land-of-stamp-backend/interceptor"

	"connectrpc.com/connect"
	"github.com/google/uuid"
)

// StampService implements pbconnect.StampServiceHandler.
type StampService struct {
	pbconnect.UnimplementedStampServiceHandler
}

// ── Cards ──────────────────────────────────────────────

func (s *StampService) GetMyCards(ctx context.Context, _ *connect.Request[pb.GetMyCardsRequest]) (*connect.Response[pb.StampCardList], error) {
	claims := interceptor.GetUser(ctx)
	if claims == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	var cards []db.StampCard
	if err := db.DB.WithContext(ctx).Where("user_id = ?", claims.UserID).Find(&cards).Error; err != nil {
		slog.ErrorContext(ctx, "stamps: queryCards failed", "error", err)
		return nil, connect.NewError(connect.CodeInternal, nil)
	}
	return connect.NewResponse(cardsToProtoList(cards)), nil
}

func (s *StampService) JoinShop(ctx context.Context, req *connect.Request[pb.JoinShopRequest]) (*connect.Response[pb.StampCard], error) {
	claims := interceptor.GetUser(ctx)
	if claims == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}
	shopID := req.Msg.ShopId
	if shopID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, nil)
	}

	// Verify shop exists
	var shop db.Shop
	if err := db.DB.WithContext(ctx).Where("uuid = ?", shopID).First(&shop).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}

	// Check if user already has an active card
	var existing db.StampCard
	if err := db.DB.WithContext(ctx).
		Where("user_id = ? AND shop_id = ? AND redeemed = ?", claims.UserID, shopID, false).
		First(&existing).Error; err == nil {
		return connect.NewResponse(existing.ToProto()), nil
	}

	card := db.StampCard{
		UUID: uuid.New(), UserID: claims.UserID, ShopID: shopID,
		Stamps: 0, Redeemed: false,
	}
	if err := db.DB.WithContext(ctx).Create(&card).Error; err != nil {
		slog.ErrorContext(ctx, "stamps: join shop failed", "user", claims.UserID, "shop", shopID, "error", err)
		return nil, connect.NewError(connect.CodeInternal, nil)
	}

	slog.InfoContext(ctx, "user joined shop", "user", claims.UserID, "shop", shopID)
	return connect.NewResponse(card.ToProto()), nil
}

func (s *StampService) GetShopCards(ctx context.Context, req *connect.Request[pb.GetShopCardsRequest]) (*connect.Response[pb.StampCardList], error) {
	claims := interceptor.GetUser(ctx)
	shopID := req.Msg.ShopId
	if shopID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, nil)
	}
	if err := verifyShopOwner(ctx, shopID, claims); err != nil {
		return nil, err
	}

	var cards []db.StampCard
	if err := db.DB.WithContext(ctx).Where("shop_id = ?", shopID).Find(&cards).Error; err != nil {
		slog.ErrorContext(ctx, "stamps: queryCards failed", "error", err)
		return nil, connect.NewError(connect.CodeInternal, nil)
	}
	return connect.NewResponse(cardsToProtoList(cards)), nil
}

func (s *StampService) GrantStamp(ctx context.Context, req *connect.Request[pb.GrantStampRequest]) (*connect.Response[pb.StampCard], error) {
	claims := interceptor.GetUser(ctx)
	shopID := req.Msg.ShopId
	if shopID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, nil)
	}
	if err := verifyShopOwner(ctx, shopID, claims); err != nil {
		return nil, err
	}

	var shop db.Shop
	if err := db.DB.WithContext(ctx).Where("uuid = ?", shopID).First(&shop).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, nil)
	}

	card, err := getOrCreateCard(ctx, req.Msg.UserId, shopID)
	if err != nil {
		return nil, err
	}

	if card.Stamps < shop.StampsRequired {
		card.Stamps++
		if err := db.DB.WithContext(ctx).Model(&card).Update("stamps", card.Stamps).Error; err != nil {
			slog.ErrorContext(ctx, "stamps: failed to update", "card", card.UUID, "error", err)
			return nil, connect.NewError(connect.CodeInternal, nil)
		}
		slog.InfoContext(ctx, "stamp granted", "card", card.UUID, "user", req.Msg.UserId, "shop", shopID, "stamps", card.Stamps)
	}

	return connect.NewResponse(card.ToProto()), nil
}

func (s *StampService) UpdateStampCount(ctx context.Context, req *connect.Request[pb.UpdateStampCountRequest]) (*connect.Response[pb.StampCard], error) {
	claims := interceptor.GetUser(ctx)
	shopID := req.Msg.ShopId
	if shopID == "" || req.Msg.UserId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, nil)
	}
	if err := verifyShopOwner(ctx, shopID, claims); err != nil {
		return nil, err
	}

	var shop db.Shop
	if err := db.DB.WithContext(ctx).Where("uuid = ?", shopID).First(&shop).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, nil)
	}

	stamps := req.Msg.Stamps
	if stamps < 0 {
		stamps = 0
	}
	if stamps > shop.StampsRequired {
		stamps = shop.StampsRequired
	}

	card, err := getOrCreateCard(ctx, req.Msg.UserId, shopID)
	if err != nil {
		return nil, err
	}

	if err := db.DB.WithContext(ctx).Model(&card).Update("stamps", stamps).Error; err != nil {
		slog.ErrorContext(ctx, "stamps: failed to update card stamps", "card", card.UUID, "error", err)
		return nil, connect.NewError(connect.CodeInternal, nil)
	}
	card.Stamps = stamps

	slog.InfoContext(ctx, "stamp count updated", "card", card.UUID, "user", req.Msg.UserId, "shop", shopID, "stamps", stamps)
	return connect.NewResponse(card.ToProto()), nil
}

func (s *StampService) RedeemCard(ctx context.Context, req *connect.Request[pb.RedeemCardRequest]) (*connect.Response[pb.StatusResponse], error) {
	claims := interceptor.GetUser(ctx)
	if claims == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}
	cardID := req.Msg.CardId
	if cardID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, nil)
	}

	var card db.StampCard
	if err := db.DB.WithContext(ctx).Where("uuid = ? AND redeemed = ?", cardID, false).First(&card).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	if card.UserID != claims.UserID {
		return nil, connect.NewError(connect.CodePermissionDenied, nil)
	}

	var shop db.Shop
	if err := db.DB.WithContext(ctx).Where("uuid = ?", card.ShopID).First(&shop).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, nil)
	}
	if card.Stamps < shop.StampsRequired {
		return nil, connect.NewError(connect.CodeFailedPrecondition, nil)
	}

	if err := db.DB.WithContext(ctx).Model(&card).Update("redeemed", true).Error; err != nil {
		slog.ErrorContext(ctx, "stamps: failed to redeem card", "card", cardID, "error", err)
		return nil, connect.NewError(connect.CodeInternal, nil)
	}

	// Auto-create a fresh card
	freshCard := db.StampCard{UUID: uuid.New(), UserID: claims.UserID, ShopID: card.ShopID}
	db.DB.WithContext(ctx).Create(&freshCard) // ignore unique constraint error

	slog.InfoContext(ctx, "card redeemed", "card", cardID, "user", claims.UserID, "shop", card.ShopID)
	return connect.NewResponse(&pb.StatusResponse{Status: "redeemed"}), nil
}

func (s *StampService) GetShopCustomers(ctx context.Context, req *connect.Request[pb.GetShopCustomersRequest]) (*connect.Response[pb.UserList], error) {
	claims := interceptor.GetUser(ctx)
	shopID := req.Msg.ShopId
	if shopID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, nil)
	}
	if err := verifyShopOwner(ctx, shopID, claims); err != nil {
		return nil, err
	}

	var userUUIDs []string
	if err := db.DB.WithContext(ctx).Model(&db.StampCard{}).Where("shop_id = ?", shopID).Distinct("user_id").Pluck("user_id", &userUUIDs).Error; err != nil {
		slog.ErrorContext(ctx, "stamps: failed to fetch customer IDs", "shop", shopID, "error", err)
		return nil, connect.NewError(connect.CodeInternal, nil)
	}
	if len(userUUIDs) == 0 {
		return connect.NewResponse(&pb.UserList{}), nil
	}

	var users []db.User
	if err := db.DB.WithContext(ctx).Where("uuid IN ? AND role = ?", userUUIDs, "user").Find(&users).Error; err != nil {
		slog.ErrorContext(ctx, "stamps: failed to fetch customers", "shop", shopID, "error", err)
		return nil, connect.NewError(connect.CodeInternal, nil)
	}
	out := make([]*pb.User, len(users))
	for i := range users {
		out[i] = users[i].ToProto()
	}
	return connect.NewResponse(&pb.UserList{Users: out}), nil
}

// ── QR Tokens ──────────────────────────────────────────

func (s *StampService) CreateStampToken(ctx context.Context, req *connect.Request[pb.CreateStampTokenRequest]) (*connect.Response[pb.StampToken], error) {
	claims := interceptor.GetUser(ctx)
	shopID := req.Msg.ShopId
	if shopID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, nil)
	}
	if err := verifyShopOwner(ctx, shopID, claims); err != nil {
		return nil, err
	}

	now := time.Now().UTC()

	// Soft-delete existing tokens + claims for this shop
	var tokenUUIDs []string
	db.DB.WithContext(ctx).Model(&db.StampToken{}).Where("shop_id = ?", shopID).Pluck("uuid", &tokenUUIDs)
	if len(tokenUUIDs) > 0 {
		db.DB.WithContext(ctx).Where("token_id IN ?", tokenUUIDs).Delete(&db.StampTokenClaim{})
	}
	db.DB.WithContext(ctx).Where("shop_id = ?", shopID).Delete(&db.StampToken{})

	// Soft-delete globally expired tokens
	var expiredUUIDs []string
	db.DB.WithContext(ctx).Model(&db.StampToken{}).Where("expires_at < ?", now).Pluck("uuid", &expiredUUIDs)
	if len(expiredUUIDs) > 0 {
		db.DB.WithContext(ctx).Where("token_id IN ?", expiredUUIDs).Delete(&db.StampTokenClaim{})
	}
	db.DB.WithContext(ctx).Where("expires_at < ?", now).Delete(&db.StampToken{})

	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		slog.ErrorContext(ctx, "qr: failed to generate random token", "error", err)
		return nil, connect.NewError(connect.CodeInternal, nil)
	}
	token := hex.EncodeToString(tokenBytes)
	expiresAt := now.Add(60 * time.Second)

	st := db.StampToken{UUID: uuid.New(), ShopID: shopID, Token: token, ExpiresAt: expiresAt}
	if err := db.DB.WithContext(ctx).Create(&st).Error; err != nil {
		slog.ErrorContext(ctx, "qr: failed to create stamp token", "error", err)
		return nil, connect.NewError(connect.CodeInternal, nil)
	}

	slog.InfoContext(ctx, "stamp token created", "shop", shopID, "expires", expiresAt)
	return connect.NewResponse(st.ToProtoToken()), nil
}

func (s *StampService) GetStampTokenStatus(ctx context.Context, req *connect.Request[pb.GetStampTokenStatusRequest]) (*connect.Response[pb.StampTokenStatusResponse], error) {
	claims := interceptor.GetUser(ctx)
	shopID := req.Msg.ShopId
	if shopID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, nil)
	}
	if err := verifyShopOwner(ctx, shopID, claims); err != nil {
		return nil, err
	}

	var st db.StampToken
	if err := db.DB.WithContext(ctx).Where("shop_id = ?", shopID).First(&st).Error; err != nil {
		return connect.NewResponse(&pb.StampTokenStatusResponse{Active: false}), nil
	}

	if time.Now().UTC().After(st.ExpiresAt) {
		// Cleanup expired
		var tokenUUIDs []string
		db.DB.WithContext(ctx).Model(&db.StampToken{}).Where("shop_id = ?", shopID).Pluck("uuid", &tokenUUIDs)
		if len(tokenUUIDs) > 0 {
			db.DB.WithContext(ctx).Where("token_id IN ?", tokenUUIDs).Delete(&db.StampTokenClaim{})
		}
		db.DB.WithContext(ctx).Where("shop_id = ?", shopID).Delete(&db.StampToken{})
		return connect.NewResponse(&pb.StampTokenStatusResponse{Active: false}), nil
	}

	return connect.NewResponse(&pb.StampTokenStatusResponse{
		Active:    true,
		ExpiresAt: st.ExpiresAt.Format(time.RFC3339),
	}), nil
}

func (s *StampService) ClaimStamp(ctx context.Context, req *connect.Request[pb.ClaimStampRequest]) (*connect.Response[pb.ClaimStampResponse], error) {
	claims := interceptor.GetUser(ctx)
	if claims == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}
	if claims.Role != "user" {
		return nil, connect.NewError(connect.CodePermissionDenied, nil)
	}
	if req.Msg.Token == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, nil)
	}

	var st db.StampToken
	if err := db.DB.WithContext(ctx).Where("token = ?", req.Msg.Token).First(&st).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	if time.Now().UTC().After(st.ExpiresAt) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, nil)
	}

	shopUUID := st.ShopID
	tokenUUID := st.UUID.String()

	// Check if already claimed
	var existingClaim db.StampTokenClaim
	if err := db.DB.WithContext(ctx).Where("token_id = ? AND user_id = ?", tokenUUID, claims.UserID).First(&existingClaim).Error; err == nil {
		var shop db.Shop
		db.DB.WithContext(ctx).Where("uuid = ?", shopUUID).First(&shop)
		var card db.StampCard
		db.DB.WithContext(ctx).Where("user_id = ? AND shop_id = ? AND redeemed = ?", claims.UserID, shopUUID, false).First(&card)
		return connect.NewResponse(&pb.ClaimStampResponse{
			ShopName: shop.Name, Stamps: card.Stamps, StampsRequired: shop.StampsRequired,
			Message: "You already scanned this QR code! ✅",
		}), nil
	}

	// Record the claim
	claim := db.StampTokenClaim{TokenID: tokenUUID, UserID: claims.UserID}
	if err := db.DB.WithContext(ctx).Create(&claim).Error; err != nil {
		slog.ErrorContext(ctx, "qr: failed to record claim", "error", err)
		return nil, connect.NewError(connect.CodeInternal, nil)
	}

	var shop db.Shop
	if err := db.DB.WithContext(ctx).Where("uuid = ?", shopUUID).First(&shop).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, nil)
	}

	card, cerr := getOrCreateCard(ctx, claims.UserID, shopUUID)
	if cerr != nil {
		return nil, cerr
	}

	if card.Stamps >= shop.StampsRequired {
		return connect.NewResponse(&pb.ClaimStampResponse{
			ShopName: shop.Name, Stamps: card.Stamps, StampsRequired: shop.StampsRequired,
			Message: "Your card is already full! Redeem your reward first.",
		}), nil
	}

	card.Stamps++
	if err := db.DB.WithContext(ctx).Model(&card).Update("stamps", card.Stamps).Error; err != nil {
		slog.ErrorContext(ctx, "qr: failed to update stamp count", "card", card.UUID, "error", err)
		return nil, connect.NewError(connect.CodeInternal, nil)
	}

	// Soft-delete the token + claims after successful claim
	db.DB.WithContext(ctx).Where("token_id = ?", tokenUUID).Delete(&db.StampTokenClaim{})
	db.DB.WithContext(ctx).Delete(&st)

	slog.InfoContext(ctx, "stamp claimed via QR", "card", card.UUID, "user", claims.UserID, "shop", shopUUID, "stamps", card.Stamps)

	msg := "Stamp collected! 🎉"
	if card.Stamps >= shop.StampsRequired {
		msg = "Card complete! 🏆 You can now redeem your reward!"
	}
	return connect.NewResponse(&pb.ClaimStampResponse{
		ShopName: shop.Name, Stamps: card.Stamps, StampsRequired: shop.StampsRequired, Message: msg,
	}), nil
}

// ── Shared helpers ─────────────────────────────────────

func verifyShopOwner(ctx context.Context, shopID string, claims *auth.Claims) *connect.Error {
	if claims == nil {
		return connect.NewError(connect.CodeUnauthenticated, nil)
	}
	var shop db.Shop
	if err := db.DB.WithContext(ctx).Where("uuid = ?", shopID).First(&shop).Error; err != nil {
		return connect.NewError(connect.CodeNotFound, nil)
	}
	if shop.OwnerID != claims.UserID {
		return connect.NewError(connect.CodePermissionDenied, nil)
	}
	return nil
}

func getOrCreateCard(ctx context.Context, userID, shopID string) (*db.StampCard, *connect.Error) {
	var card db.StampCard
	err := db.DB.WithContext(ctx).
		Where("user_id = ? AND shop_id = ? AND redeemed = ?", userID, shopID, false).
		First(&card).Error
	if err != nil {
		card = db.StampCard{UUID: uuid.New(), UserID: userID, ShopID: shopID}
		if err := db.DB.WithContext(ctx).Create(&card).Error; err != nil {
			slog.ErrorContext(ctx, "stamps: failed to create card", "user", userID, "shop", shopID, "error", err)
			return nil, connect.NewError(connect.CodeInternal, nil)
		}
	}
	return &card, nil
}

func cardsToProtoList(cards []db.StampCard) *pb.StampCardList {
	out := make([]*pb.StampCard, len(cards))
	for i := range cards {
		out[i] = cards[i].ToProto()
	}
	return &pb.StampCardList{Cards: out}
}
