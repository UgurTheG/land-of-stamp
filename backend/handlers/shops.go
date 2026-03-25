package handlers

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"strings"

	"land-of-stamp-backend/auth"
	"land-of-stamp-backend/db"
	"land-of-stamp-backend/gen/pb"
	"land-of-stamp-backend/middleware"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

// scanShop scans a shop row into a pb.Shop.
func scanShop(scanner interface{ Scan(dest ...any) error }) (*pb.Shop, error) {
	var s pb.Shop
	err := scanner.Scan(&s.Id, &s.Name, &s.Description, &s.RewardDescription, &s.StampsRequired, &s.Color, &s.OwnerId)
	return &s, err
}

// verifyShopOwner checks that the request has a valid auth claim, a shop ID in
// the path, and that the authenticated user owns the shop.  It writes an HTTP
// error and returns "", nil on failure.
func verifyShopOwner(ctx context.Context, w http.ResponseWriter, r *http.Request) (string, *auth.Claims) {
	claims := middleware.GetUser(r)
	if claims == nil {
		jsonError(ctx, w, "unauthorized", http.StatusUnauthorized)
		return "", nil
	}

	shopID := r.PathValue("id")
	if shopID == "" {
		jsonError(ctx, w, "shop id required", http.StatusBadRequest)
		return "", nil
	}

	var ownerID string
	if err := db.DB.QueryRowContext(ctx, "SELECT owner_id FROM shops WHERE id = ?", shopID).Scan(&ownerID); err != nil {
		slog.WarnContext(ctx, "shops: not found", "shop", shopID, "error", err)
		jsonError(ctx, w, "shop not found", http.StatusNotFound)
		return "", nil
	}
	if ownerID != claims.UserID {
		jsonError(ctx, w, "you don't own this shop", http.StatusForbidden)
		return "", nil
	}
	return shopID, claims
}

// scanShops scans all rows from a shop query into a proto.Message slice.
func scanShops(ctx context.Context, rows *sql.Rows) []proto.Message {
	var shops []proto.Message
	for rows.Next() {
		s, err := scanShop(rows)
		if err != nil {
			slog.WarnContext(ctx, "shops: scan error", "error", err)
			continue
		}
		shops = append(shops, s)
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "shops: rows iteration error", "error", err)
	}
	return shops
}

// ListShops returns all shops.
func ListShops(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := db.DB.QueryContext(ctx, "SELECT id, name, description, reward_description, stamps_required, color, owner_id FROM shops")
	if err != nil {
		slog.ErrorContext(ctx, "shops: failed to list", "error", err)
		jsonError(ctx, w, "failed to fetch shops", http.StatusInternalServerError)
		return
	}
	defer func() { _ = rows.Close() }()
	writeProtoList(ctx, w, scanShops(ctx, rows))
}

// CreateShop creates a new shop owned by the authenticated admin.
func CreateShop(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims := middleware.GetUser(r)
	if claims == nil {
		jsonError(ctx, w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req pb.CreateShopRequest
	if err := readProto(r, &req); err != nil {
		slog.WarnContext(ctx, "shops: invalid request body", "error", err)
		jsonError(ctx, w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.RewardDescription == "" {
		jsonError(ctx, w, "name and reward description are required", http.StatusBadRequest)
		return
	}
	if req.StampsRequired < 2 || req.StampsRequired > 20 {
		req.StampsRequired = 8
	}
	if req.Color == "" {
		req.Color = "#6366f1"
	}

	id := uuid.New().String()
	_, err := db.DB.ExecContext(ctx,
		"INSERT INTO shops (id, name, description, reward_description, stamps_required, color, owner_id) VALUES (?, ?, ?, ?, ?, ?, ?)",
		id, req.Name, req.Description, req.RewardDescription, req.StampsRequired, req.Color, claims.UserID,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") {
			jsonError(ctx, w, "a shop with this name already exists", http.StatusConflict)
			return
		}
		slog.ErrorContext(ctx, "shops: create failed", "error", err)
		jsonError(ctx, w, "failed to create shop", http.StatusInternalServerError)
		return
	}

	slog.InfoContext(ctx, "shop created", "id", id, "name", req.Name, "owner", claims.UserID)
	writeProto(ctx, w, http.StatusCreated, &pb.Shop{
		Id: id, Name: req.Name, Description: req.Description,
		RewardDescription: req.RewardDescription, StampsRequired: req.StampsRequired,
		Color: req.Color, OwnerId: claims.UserID,
	})
}

// UpdateShop updates an existing shop owned by the authenticated admin.
func UpdateShop(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	shopID, claims := verifyShopOwner(ctx, w, r)
	if claims == nil {
		return
	}

	var req pb.CreateShopRequest
	if err := readProto(r, &req); err != nil {
		slog.WarnContext(ctx, "shops: invalid request body for update", "error", err)
		jsonError(ctx, w, "invalid request body", http.StatusBadRequest)
		return
	}

	_, err := db.DB.ExecContext(ctx,
		"UPDATE shops SET name=?, description=?, reward_description=?, stamps_required=?, color=? WHERE id=?",
		req.Name, req.Description, req.RewardDescription, req.StampsRequired, req.Color, shopID,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") {
			jsonError(ctx, w, "a shop with this name already exists", http.StatusConflict)
			return
		}
		slog.ErrorContext(ctx, "shops: update failed", "id", shopID, "error", err)
		jsonError(ctx, w, "failed to update shop", http.StatusInternalServerError)
		return
	}

	slog.InfoContext(ctx, "shop updated", "id", shopID, "name", req.Name)
	writeProto(ctx, w, http.StatusOK, &pb.Shop{
		Id: shopID, Name: req.Name, Description: req.Description,
		RewardDescription: req.RewardDescription, StampsRequired: req.StampsRequired,
		Color: req.Color, OwnerId: claims.UserID,
	})
}

// GetMyShops returns all shops owned by the authenticated admin.
func GetMyShops(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims := middleware.GetUser(r)
	if claims == nil {
		jsonError(ctx, w, "unauthorized", http.StatusUnauthorized)
		return
	}

	rows, err := db.DB.QueryContext(ctx,
		"SELECT id, name, description, reward_description, stamps_required, color, owner_id FROM shops WHERE owner_id = ?",
		claims.UserID,
	)
	if err != nil {
		slog.ErrorContext(ctx, "shops: fetch my shops failed", "error", err)
		jsonError(ctx, w, "failed to fetch shops", http.StatusInternalServerError)
		return
	}
	defer func() { _ = rows.Close() }()
	writeProtoList(ctx, w, scanShops(ctx, rows))
}
