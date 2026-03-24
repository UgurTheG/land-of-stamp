package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"land-of-stamp-backend/db"
	pb "land-of-stamp-backend/gen/pb"
	"land-of-stamp-backend/middleware"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

func GetMyCards(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims := middleware.GetUser(r)
	if claims == nil {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Ensure cards exist for every shop — collect IDs first to avoid
	// holding the query connection while executing inserts (deadlock
	// with MaxOpenConns=1).
	rows, err := db.DB.QueryContext(ctx, "SELECT id FROM shops")
	if err != nil {
		slog.ErrorContext(ctx, "stamps: failed to query shops", "error", err)
		jsonError(w, "failed to load cards", http.StatusInternalServerError)
		return
	}
	var shopIDs []string
	for rows.Next() {
		var sid string
		if err := rows.Scan(&sid); err != nil {
			slog.WarnContext(ctx, "stamps: failed to scan shop id", "error", err)
			continue
		}
		shopIDs = append(shopIDs, sid)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "stamps: rows iteration error for shops", "error", err)
	}

	for _, sid := range shopIDs {
		if _, err := db.DB.ExecContext(ctx,
			"INSERT OR IGNORE INTO stamp_cards (id, user_id, shop_id, stamps, redeemed) VALUES (?, ?, ?, 0, FALSE)",
			uuid.New().String(), claims.UserID, sid,
		); err != nil {
			slog.WarnContext(ctx, "stamps: failed to ensure card exists", "shop", sid, "user", claims.UserID, "error", err)
		}
	}

	cards := queryCards(ctx, "user_id = ?", claims.UserID)
	writeProtoList(w, http.StatusOK, cards)
}

func GetShopCards(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	shopID := r.PathValue("id")
	if shopID == "" {
		jsonError(w, "shop id required", http.StatusBadRequest)
		return
	}
	cards := queryCards(ctx, "shop_id = ?", shopID)
	writeProtoList(w, http.StatusOK, cards)
}

func GrantStamp(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims := middleware.GetUser(r)
	if claims == nil {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	shopID := r.PathValue("id")
	if shopID == "" {
		jsonError(w, "shop id required", http.StatusBadRequest)
		return
	}

	// Verify admin owns this shop
	var ownerID string
	err := db.DB.QueryRowContext(ctx, "SELECT owner_id FROM shops WHERE id = ?", shopID).Scan(&ownerID)
	if err != nil {
		slog.WarnContext(ctx, "stamps: shop not found for grant", "shop", shopID, "error", err)
		jsonError(w, "shop not found", http.StatusNotFound)
		return
	}
	if ownerID != claims.UserID {
		jsonError(w, "you don't own this shop", http.StatusForbidden)
		return
	}

	var req pb.GrantStampRequest
	if err := readProto(r, &req); err != nil {
		slog.WarnContext(ctx, "stamps: invalid request body", "error", err)
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Get stamps_required for this shop
	var stampsRequired int32
	if err := db.DB.QueryRowContext(ctx, "SELECT stamps_required FROM shops WHERE id = ?", shopID).Scan(&stampsRequired); err != nil {
		slog.ErrorContext(ctx, "stamps: failed to get stamps_required", "shop", shopID, "error", err)
		jsonError(w, "failed to get shop configuration", http.StatusInternalServerError)
		return
	}

	// Get or create card
	var cardID string
	var stamps int32
	err = db.DB.QueryRowContext(ctx,
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
			jsonError(w, "failed to create stamp card", http.StatusInternalServerError)
			return
		}
	}

	if stamps < stampsRequired {
		stamps++
		if _, err := db.DB.ExecContext(ctx, "UPDATE stamp_cards SET stamps = ? WHERE id = ?", stamps, cardID); err != nil {
			slog.ErrorContext(ctx, "stamps: failed to update stamp count", "card", cardID, "error", err)
			jsonError(w, "failed to grant stamp", http.StatusInternalServerError)
			return
		}
		slog.InfoContext(ctx, "stamp granted", "card", cardID, "user", req.UserId, "shop", shopID, "stamps", stamps)
	}

	writeProto(w, http.StatusOK, &pb.StampCard{
		Id: cardID, UserId: req.UserId, ShopId: shopID,
		Stamps: stamps, Redeemed: false,
	})
}

// UpdateStampCount allows an admin to set the stamp count on a user's card (including reducing it).
func UpdateStampCount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims := middleware.GetUser(r)
	if claims == nil {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	shopID := r.PathValue("id")
	if shopID == "" {
		jsonError(w, "shop id required", http.StatusBadRequest)
		return
	}

	// Verify admin owns this shop
	var ownerID string
	err := db.DB.QueryRowContext(ctx, "SELECT owner_id FROM shops WHERE id = ?", shopID).Scan(&ownerID)
	if err != nil {
		slog.WarnContext(ctx, "stamps: shop not found for update", "shop", shopID, "error", err)
		jsonError(w, "shop not found", http.StatusNotFound)
		return
	}
	if ownerID != claims.UserID {
		jsonError(w, "you don't own this shop", http.StatusForbidden)
		return
	}

	var req pb.UpdateStampCountRequest
	if err := readProto(r, &req); err != nil {
		slog.WarnContext(ctx, "stamps: invalid request body", "error", err)
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.UserId == "" {
		jsonError(w, "userId is required", http.StatusBadRequest)
		return
	}

	// Clamp stamps to valid range
	var stampsRequired int32
	if err := db.DB.QueryRowContext(ctx, "SELECT stamps_required FROM shops WHERE id = ?", shopID).Scan(&stampsRequired); err != nil {
		slog.ErrorContext(ctx, "stamps: failed to get stamps_required", "shop", shopID, "error", err)
		jsonError(w, "failed to get shop configuration", http.StatusInternalServerError)
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
	err = db.DB.QueryRowContext(ctx,
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
			jsonError(w, "failed to update stamp count", http.StatusInternalServerError)
			return
		}
	} else {
		if _, err := db.DB.ExecContext(ctx, "UPDATE stamp_cards SET stamps = ? WHERE id = ?", req.Stamps, cardID); err != nil {
			slog.ErrorContext(ctx, "stamps: failed to update card stamps", "card", cardID, "error", err)
			jsonError(w, "failed to update stamp count", http.StatusInternalServerError)
			return
		}
	}

	slog.InfoContext(ctx, "stamp count updated", "card", cardID, "user", req.UserId, "shop", shopID, "stamps", req.Stamps)
	writeProto(w, http.StatusOK, &pb.StampCard{
		Id: cardID, UserId: req.UserId, ShopId: shopID,
		Stamps: req.Stamps, Redeemed: false,
	})
}

func RedeemCard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims := middleware.GetUser(r)
	if claims == nil {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	cardID := r.PathValue("id")
	if cardID == "" {
		jsonError(w, "card id required", http.StatusBadRequest)
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
		jsonError(w, "card not found", http.StatusNotFound)
		return
	}
	if userID != claims.UserID {
		jsonError(w, "this card doesn't belong to you", http.StatusForbidden)
		return
	}

	// Check stamps are complete
	var stampsRequired int32
	if err := db.DB.QueryRowContext(ctx, "SELECT stamps_required FROM shops WHERE id = ?", shopID).Scan(&stampsRequired); err != nil {
		slog.ErrorContext(ctx, "stamps: failed to get stamps_required for redeem", "shop", shopID, "error", err)
		jsonError(w, "failed to verify shop requirements", http.StatusInternalServerError)
		return
	}
	if stamps < stampsRequired {
		jsonError(w, "not enough stamps to redeem", http.StatusBadRequest)
		return
	}

	if _, err := db.DB.ExecContext(ctx, "UPDATE stamp_cards SET redeemed = TRUE WHERE id = ?", cardID); err != nil {
		slog.ErrorContext(ctx, "stamps: failed to redeem card", "card", cardID, "error", err)
		jsonError(w, "failed to redeem card", http.StatusInternalServerError)
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
	writeProto(w, http.StatusOK, &pb.StatusResponse{Status: "redeemed"})
}

// ListCustomers returns all users with role 'user' (for admin stamp granting).
func ListCustomers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := db.DB.QueryContext(ctx, "SELECT id, username, role FROM users WHERE role = 'user'")
	if err != nil {
		slog.ErrorContext(ctx, "stamps: failed to fetch customers", "error", err)
		jsonError(w, "failed to fetch users", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

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
		slog.ErrorContext(ctx, "stamps: rows iteration error for customers", "error", err)
	}
	writeProtoList(w, http.StatusOK, users)
}

func queryCards(ctx context.Context, where string, args ...any) []proto.Message {
	rows, err := db.DB.QueryContext(ctx,
		"SELECT id, user_id, shop_id, stamps, redeemed, created_at FROM stamp_cards WHERE "+where,
		args...,
	)
	if err != nil {
		slog.ErrorContext(ctx, "stamps: queryCards failed", "where", where, "error", err)
		return []proto.Message{}
	}
	defer rows.Close()

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
