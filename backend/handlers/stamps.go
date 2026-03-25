package handlers

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"

	"land-of-stamp-backend/db"
	"land-of-stamp-backend/gen/pb"
	"land-of-stamp-backend/middleware"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

// GetMyCards returns all stamp cards for the authenticated user.
// Only returns cards for shops the user has explicitly joined.
func GetMyCards(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims := middleware.GetUser(r)
	if claims == nil {
		jsonError(ctx, w, "unauthorized", http.StatusUnauthorized)
		return
	}

	cards := queryCardsByUser(ctx, claims.UserID)
	writeProtoList(ctx, w, cards)
}

// JoinShop lets a user create a stamp card for a shop (opt-in).
func JoinShop(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims := middleware.GetUser(r)
	if claims == nil {
		jsonError(ctx, w, "unauthorized", http.StatusUnauthorized)
		return
	}

	shopID := r.PathValue("id")
	if shopID == "" {
		jsonError(ctx, w, "shop id required", http.StatusBadRequest)
		return
	}

	// Verify shop exists
	var exists int
	if err := db.DB.QueryRowContext(ctx, "SELECT 1 FROM shops WHERE id = ?", shopID).Scan(&exists); err != nil {
		jsonError(ctx, w, "shop not found", http.StatusNotFound)
		return
	}

	// Check if user already has an active card for this shop
	var existingID string
	err := db.DB.QueryRowContext(ctx,
		"SELECT id FROM stamp_cards WHERE user_id = ? AND shop_id = ? AND redeemed = FALSE",
		claims.UserID, shopID,
	).Scan(&existingID)
	if err == nil {
		// Already joined
		writeProto(ctx, w, http.StatusOK, &pb.StampCard{
			Id: existingID, UserId: claims.UserID, ShopId: shopID,
			Stamps: 0, Redeemed: false,
		})
		return
	}

	cardID := uuid.New().String()
	if _, err := db.DB.ExecContext(ctx,
		"INSERT OR IGNORE INTO stamp_cards (id, user_id, shop_id, stamps, redeemed) VALUES (?, ?, ?, 0, FALSE)",
		cardID, claims.UserID, shopID,
	); err != nil {
		slog.ErrorContext(ctx, "stamps: failed to create card on join", "user", claims.UserID, "shop", shopID, "error", err)
		jsonError(ctx, w, "failed to join shop", http.StatusInternalServerError)
		return
	}

	slog.InfoContext(ctx, "user joined shop", "user", claims.UserID, "shop", shopID)
	writeProto(ctx, w, http.StatusCreated, &pb.StampCard{
		Id: cardID, UserId: claims.UserID, ShopId: shopID,
		Stamps: 0, Redeemed: false,
	})
}

// GetShopCards returns all stamp cards for a given shop.
func GetShopCards(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	shopID := r.PathValue("id")
	if shopID == "" {
		jsonError(ctx, w, "shop id required", http.StatusBadRequest)
		return
	}
	cards := queryCardsByShop(ctx, shopID)
	writeProtoList(ctx, w, cards)
}

// GrantStamp adds a stamp to a user's card for the given shop.
func GrantStamp(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	shopID, _ := verifyShopOwner(ctx, w, r)
	if shopID == "" {
		return
	}

	var req pb.GrantStampRequest
	if err := readProto(r, &req); err != nil {
		slog.WarnContext(ctx, "stamps: invalid request body", "error", err)
		jsonError(ctx, w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Get stamps_required for this shop
	var stampsRequired int32
	if err := db.DB.QueryRowContext(ctx, "SELECT stamps_required FROM shops WHERE id = ?", shopID).Scan(&stampsRequired); err != nil {
		slog.ErrorContext(ctx, "stamps: failed to get stamps_required", "shop", shopID, "error", err)
		jsonError(ctx, w, "failed to get shop configuration", http.StatusInternalServerError)
		return
	}

	// Get or create card
	var cardID string
	var stamps int32
	err := db.DB.QueryRowContext(ctx,
		"SELECT id, stamps FROM stamp_cards WHERE user_id = ? AND shop_id = ? AND redeemed = FALSE",
		req.UserId, shopID,
	).Scan(&cardID, &stamps)
	if err != nil {
		cardID = uuid.New().String()
		stamps = 0
		if _, err := db.DB.ExecContext(ctx,
			"INSERT INTO stamp_cards (id, user_id, shop_id, stamps, redeemed) VALUES (?, ?, ?, 0, FALSE)",
			cardID, req.UserId, shopID,
		); err != nil {
			slog.ErrorContext(ctx, "stamps: failed to create card", "user", req.UserId, "shop", shopID, "error", err)
			jsonError(ctx, w, "failed to create stamp card", http.StatusInternalServerError)
			return
		}
	}

	if stamps < stampsRequired {
		stamps++
		if _, err := db.DB.ExecContext(ctx, "UPDATE stamp_cards SET stamps = ? WHERE id = ?", stamps, cardID); err != nil {
			slog.ErrorContext(ctx, "stamps: failed to update stamp count", "card", cardID, "error", err)
			jsonError(ctx, w, "failed to grant stamp", http.StatusInternalServerError)
			return
		}
		slog.InfoContext(ctx, "stamp granted", "card", cardID, "user", req.UserId, "shop", shopID, "stamps", stamps)
	}

	writeProto(ctx, w, http.StatusOK, &pb.StampCard{
		Id: cardID, UserId: req.UserId, ShopId: shopID,
		Stamps: stamps, Redeemed: false,
	})
}

// UpdateStampCount allows an admin to set the stamp count on a user's card (including reducing it).
func UpdateStampCount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	shopID, _ := verifyShopOwner(ctx, w, r)
	if shopID == "" {
		return
	}

	var req pb.UpdateStampCountRequest
	if err := readProto(r, &req); err != nil {
		slog.WarnContext(ctx, "stamps: invalid request body", "error", err)
		jsonError(ctx, w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.UserId == "" {
		jsonError(ctx, w, "userId is required", http.StatusBadRequest)
		return
	}

	// Clamp stamps to valid range
	var stampsRequired int32
	if err := db.DB.QueryRowContext(ctx, "SELECT stamps_required FROM shops WHERE id = ?", shopID).Scan(&stampsRequired); err != nil {
		slog.ErrorContext(ctx, "stamps: failed to get stamps_required", "shop", shopID, "error", err)
		jsonError(ctx, w, "failed to get shop configuration", http.StatusInternalServerError)
		return
	}
	if req.Stamps < 0 {
		req.Stamps = 0
	}
	if req.Stamps > stampsRequired {
		req.Stamps = stampsRequired
	}

	// Get or create card
	var cardID string
	err := db.DB.QueryRowContext(ctx,
		"SELECT id FROM stamp_cards WHERE user_id = ? AND shop_id = ? AND redeemed = FALSE",
		req.UserId, shopID,
	).Scan(&cardID)
	if err != nil {
		cardID = uuid.New().String()
		if _, err := db.DB.ExecContext(ctx,
			"INSERT INTO stamp_cards (id, user_id, shop_id, stamps, redeemed) VALUES (?, ?, ?, ?, FALSE)",
			cardID, req.UserId, shopID, req.Stamps,
		); err != nil {
			slog.ErrorContext(ctx, "stamps: failed to create card", "user", req.UserId, "shop", shopID, "error", err)
			jsonError(ctx, w, "failed to update stamp count", http.StatusInternalServerError)
			return
		}
	} else {
		if _, err := db.DB.ExecContext(ctx, "UPDATE stamp_cards SET stamps = ? WHERE id = ?", req.Stamps, cardID); err != nil {
			slog.ErrorContext(ctx, "stamps: failed to update card stamps", "card", cardID, "error", err)
			jsonError(ctx, w, "failed to update stamp count", http.StatusInternalServerError)
			return
		}
	}

	slog.InfoContext(ctx, "stamp count updated", "card", cardID, "user", req.UserId, "shop", shopID, "stamps", req.Stamps)
	writeProto(ctx, w, http.StatusOK, &pb.StampCard{
		Id: cardID, UserId: req.UserId, ShopId: shopID,
		Stamps: req.Stamps, Redeemed: false,
	})
}

// RedeemCard marks a completed stamp card as redeemed and creates a fresh card.
func RedeemCard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims := middleware.GetUser(r)
	if claims == nil {
		jsonError(ctx, w, "unauthorized", http.StatusUnauthorized)
		return
	}

	cardID := r.PathValue("id")
	if cardID == "" {
		jsonError(ctx, w, "card id required", http.StatusBadRequest)
		return
	}

	// Verify the card belongs to this user
	var userID, shopID string
	var stamps int32
	err := db.DB.QueryRowContext(ctx,
		"SELECT user_id, shop_id, stamps FROM stamp_cards WHERE id = ? AND redeemed = FALSE",
		cardID,
	).Scan(&userID, &shopID, &stamps)
	if err != nil {
		slog.WarnContext(ctx, "stamps: card not found for redeem", "card", cardID, "error", err)
		jsonError(ctx, w, "card not found", http.StatusNotFound)
		return
	}
	if userID != claims.UserID {
		jsonError(ctx, w, "this card doesn't belong to you", http.StatusForbidden)
		return
	}

	// Check stamps are complete
	var stampsRequired int32
	if err := db.DB.QueryRowContext(ctx, "SELECT stamps_required FROM shops WHERE id = ?", shopID).Scan(&stampsRequired); err != nil {
		slog.ErrorContext(ctx, "stamps: failed to get stamps_required for redeem", "shop", shopID, "error", err)
		jsonError(ctx, w, "failed to verify shop requirements", http.StatusInternalServerError)
		return
	}
	if stamps < stampsRequired {
		jsonError(ctx, w, "not enough stamps to redeem", http.StatusBadRequest)
		return
	}

	if _, err := db.DB.ExecContext(ctx, "UPDATE stamp_cards SET redeemed = TRUE WHERE id = ?", cardID); err != nil {
		slog.ErrorContext(ctx, "stamps: failed to redeem card", "card", cardID, "error", err)
		jsonError(ctx, w, "failed to redeem card", http.StatusInternalServerError)
		return
	}

	// Auto-create a fresh card so the user can keep collecting stamps at this shop.
	newCardID := uuid.New().String()
	if _, err := db.DB.ExecContext(ctx,
		"INSERT OR IGNORE INTO stamp_cards (id, user_id, shop_id, stamps, redeemed) VALUES (?, ?, ?, 0, FALSE)",
		newCardID, claims.UserID, shopID,
	); err != nil {
		slog.WarnContext(ctx, "stamps: failed to create fresh card after redeem", "user", claims.UserID, "shop", shopID, "error", err)
	}

	slog.InfoContext(ctx, "card redeemed", "card", cardID, "user", claims.UserID, "shop", shopID)
	writeProto(ctx, w, http.StatusOK, &pb.StatusResponse{Status: "redeemed"})
}

// GetShopCustomers returns users who have joined the given shop (have a stamp card for it).
func GetShopCustomers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	shopID, _ := verifyShopOwner(ctx, w, r)
	if shopID == "" {
		return
	}

	rows, err := db.DB.QueryContext(ctx,
		`SELECT DISTINCT u.id, u.username, u.role
		 FROM users u
		 INNER JOIN stamp_cards sc ON sc.user_id = u.id
		 WHERE sc.shop_id = ? AND u.role = 'user'`,
		shopID,
	)
	if err != nil {
		slog.ErrorContext(ctx, "stamps: failed to fetch shop customers", "shop", shopID, "error", err)
		jsonError(ctx, w, "failed to fetch customers", http.StatusInternalServerError)
		return
	}
	defer func() { _ = rows.Close() }()

	var users []proto.Message
	for rows.Next() {
		var u pb.User
		if err := rows.Scan(&u.Id, &u.Username, &u.Role); err != nil {
			slog.WarnContext(ctx, "stamps: failed to scan user row", "error", err)
			continue
		}
		users = append(users, &u)
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "stamps: rows iteration error for shop customers", "error", err)
	}
	writeProtoList(ctx, w, users)
}

func queryCardsByUser(ctx context.Context, userID string) []proto.Message {
	rows, err := db.DB.QueryContext(ctx,
		"SELECT id, user_id, shop_id, stamps, redeemed, created_at FROM stamp_cards WHERE user_id = ?", userID)
	if err != nil {
		slog.ErrorContext(ctx, "stamps: queryCards failed", "filter", "user_id", "error", err)
		return []proto.Message{}
	}
	defer func() { _ = rows.Close() }()
	return scanCards(ctx, rows)
}

func queryCardsByShop(ctx context.Context, shopID string) []proto.Message {
	rows, err := db.DB.QueryContext(ctx,
		"SELECT id, user_id, shop_id, stamps, redeemed, created_at FROM stamp_cards WHERE shop_id = ?", shopID)
	if err != nil {
		slog.ErrorContext(ctx, "stamps: queryCards failed", "filter", "shop_id", "error", err)
		return []proto.Message{}
	}
	defer func() { _ = rows.Close() }()
	return scanCards(ctx, rows)
}

func scanCards(ctx context.Context, rows *sql.Rows) []proto.Message {
	var cards []proto.Message
	for rows.Next() {
		var c pb.StampCard
		if err := rows.Scan(&c.Id, &c.UserId, &c.ShopId, &c.Stamps, &c.Redeemed, &c.CreatedAt); err != nil {
			slog.WarnContext(ctx, "stamps: failed to scan card row", "error", err)
			continue
		}
		cards = append(cards, &c)
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "stamps: queryCards rows iteration error", "error", err)
	}
	return cards
}
