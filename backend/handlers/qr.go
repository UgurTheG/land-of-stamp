package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"land-of-stamp-backend/db"
	pb "land-of-stamp-backend/gen/pb"
	"land-of-stamp-backend/middleware"

	"github.com/google/uuid"
)

// generateToken creates a cryptographically random hex token.
func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// CreateStampToken generates a short-lived QR token for the admin's shop.
// The token is valid for 60 seconds and can be claimed once by any user.
func CreateStampToken(w http.ResponseWriter, r *http.Request) {
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
		slog.WarnContext(ctx, "qr: shop not found", "shop", shopID, "error", err)
		jsonError(w, "shop not found", http.StatusNotFound)
		return
	}
	if ownerID != claims.UserID {
		jsonError(w, "you don't own this shop", http.StatusForbidden)
		return
	}

	// Invalidate any existing active tokens for this shop so only the new one is valid
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := db.DB.ExecContext(ctx,
		"DELETE FROM stamp_token_claims WHERE token_id IN (SELECT id FROM stamp_tokens WHERE shop_id = ?)",
		shopID,
	); err != nil {
		slog.WarnContext(ctx, "qr: failed to cleanup token claims for shop", "shop", shopID, "error", err)
	}
	if _, err := db.DB.ExecContext(ctx, "DELETE FROM stamp_tokens WHERE shop_id = ?", shopID); err != nil {
		slog.WarnContext(ctx, "qr: failed to cleanup tokens for shop", "shop", shopID, "error", err)
	}

	// Clean up globally expired tokens for other shops
	if _, err := db.DB.ExecContext(ctx, "DELETE FROM stamp_token_claims WHERE token_id IN (SELECT id FROM stamp_tokens WHERE expires_at < ?)", now); err != nil {
		slog.WarnContext(ctx, "qr: failed to cleanup expired token claims", "error", err)
	}
	if _, err := db.DB.ExecContext(ctx, "DELETE FROM stamp_tokens WHERE expires_at < ?", now); err != nil {
		slog.WarnContext(ctx, "qr: failed to cleanup expired tokens", "error", err)
	}

	token, err := generateToken()
	if err != nil {
		slog.ErrorContext(ctx, "qr: failed to generate random token", "error", err)
		jsonError(w, "failed to create token", http.StatusInternalServerError)
		return
	}
	expiresAt := time.Now().UTC().Add(60 * time.Second)
	id := uuid.New().String()

	_, err = db.DB.ExecContext(ctx,
		"INSERT INTO stamp_tokens (id, shop_id, token, expires_at) VALUES (?, ?, ?, ?)",
		id, shopID, token, expiresAt.Format(time.RFC3339),
	)
	if err != nil {
		slog.ErrorContext(ctx, "qr: failed to create stamp token", "error", err)
		jsonError(w, "failed to create token", http.StatusInternalServerError)
		return
	}

	slog.InfoContext(ctx, "stamp token created", "shop", shopID, "expires", expiresAt)
	writeProto(w, http.StatusCreated, &pb.StampToken{
		Token:     token,
		ExpiresAt: expiresAt.Format(time.RFC3339),
		ShopId:    shopID,
	})
}

// ClaimStamp allows an authenticated user to claim a stamp using a QR token.
// Tokens can be used by multiple users, but each user can only claim once per token.
func ClaimStamp(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims := middleware.GetUser(r)
	if claims == nil {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if claims.Role != "user" {
		jsonError(w, "only customers can claim stamps", http.StatusForbidden)
		return
	}

	var req pb.ClaimStampRequest
	if err := readProto(r, &req); err != nil || req.Token == "" {
		slog.WarnContext(ctx, "qr: invalid claim request", "error", err)
		jsonError(w, "token is required", http.StatusBadRequest)
		return
	}

	// Find the token
	var tokenID, shopID, expiresAtStr string
	err := db.DB.QueryRowContext(ctx,
		"SELECT id, shop_id, expires_at FROM stamp_tokens WHERE token = ?",
		req.Token,
	).Scan(&tokenID, &shopID, &expiresAtStr)
	if err != nil {
		slog.InfoContext(ctx, "qr: invalid or expired token", "token_prefix", req.Token[:min(8, len(req.Token))], "error", err)
		jsonError(w, "invalid or expired QR code", http.StatusNotFound)
		return
	}

	// Check expiration
	expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
	if err != nil || time.Now().UTC().After(expiresAt) {
		slog.InfoContext(ctx, "qr: token expired", "token_id", tokenID, "expires_at", expiresAtStr)
		jsonError(w, "this QR code has expired — ask for a new one", http.StatusGone)
		return
	}

	// Check if THIS user already claimed THIS token (prevent double-scan)
	var existingClaim string
	err = db.DB.QueryRowContext(ctx,
		"SELECT token_id FROM stamp_token_claims WHERE token_id = ? AND user_id = ?",
		tokenID, claims.UserID,
	).Scan(&existingClaim)
	if err == nil {
		// Already claimed — return a friendly success-like response (not an error)
		var shopName string
		var stampsRequired int32
		if err := db.DB.QueryRowContext(ctx, "SELECT name, stamps_required FROM shops WHERE id = ?", shopID).Scan(&shopName, &stampsRequired); err != nil {
			slog.ErrorContext(ctx, "qr: failed to get shop info for already-claimed response", "shop", shopID, "error", err)
			jsonError(w, "failed to get shop information", http.StatusInternalServerError)
			return
		}
		var stamps int32
		if err := db.DB.QueryRowContext(ctx,
			"SELECT stamps FROM stamp_cards WHERE user_id = ? AND shop_id = ? AND redeemed = FALSE",
			claims.UserID, shopID,
		).Scan(&stamps); err != nil {
			slog.WarnContext(ctx, "qr: failed to get stamp count for already-claimed response", "user", claims.UserID, "shop", shopID, "error", err)
		}

		writeProto(w, http.StatusOK, &pb.ClaimStampResponse{
			ShopName:       shopName,
			Stamps:         stamps,
			StampsRequired: stampsRequired,
			Message:        "You already scanned this QR code! ✅",
		})
		return
	}

	// Record the claim
	_, err = db.DB.ExecContext(ctx,
		"INSERT INTO stamp_token_claims (token_id, user_id) VALUES (?, ?)",
		tokenID, claims.UserID,
	)
	if err != nil {
		slog.ErrorContext(ctx, "qr: failed to record claim", "error", err)
		jsonError(w, "failed to claim stamp", http.StatusInternalServerError)
		return
	}

	// Get shop info
	var shopName string
	var stampsRequired int32
	if err := db.DB.QueryRowContext(ctx, "SELECT name, stamps_required FROM shops WHERE id = ?", shopID).Scan(&shopName, &stampsRequired); err != nil {
		slog.ErrorContext(ctx, "qr: failed to get shop info", "shop", shopID, "error", err)
		jsonError(w, "failed to get shop information", http.StatusInternalServerError)
		return
	}

	// Get or create stamp card
	var cardID string
	var stamps int32
	err = db.DB.QueryRowContext(ctx,
		"SELECT id, stamps FROM stamp_cards WHERE user_id = ? AND shop_id = ? AND redeemed = FALSE",
		claims.UserID, shopID,
	).Scan(&cardID, &stamps)
	if err != nil {
		cardID = uuid.New().String()
		stamps = 0
		if _, err := db.DB.ExecContext(ctx,
			"INSERT INTO stamp_cards (id, user_id, shop_id, stamps, redeemed) VALUES (?, ?, ?, 0, FALSE)",
			cardID, claims.UserID, shopID,
		); err != nil {
			slog.ErrorContext(ctx, "qr: failed to create card for claim", "user", claims.UserID, "shop", shopID, "error", err)
			jsonError(w, "failed to create stamp card", http.StatusInternalServerError)
			return
		}
	}

	if stamps >= stampsRequired {
		writeProto(w, http.StatusOK, &pb.ClaimStampResponse{
			ShopName:       shopName,
			Stamps:         stamps,
			StampsRequired: stampsRequired,
			Message:        "Your card is already full! Redeem your reward first.",
		})
		return
	}

	stamps++
	if _, err := db.DB.ExecContext(ctx, "UPDATE stamp_cards SET stamps = ? WHERE id = ?", stamps, cardID); err != nil {
		slog.ErrorContext(ctx, "qr: failed to update stamp count", "card", cardID, "error", err)
		jsonError(w, "failed to update stamp count", http.StatusInternalServerError)
		return
	}
	slog.InfoContext(ctx, "stamp claimed via QR", "card", cardID, "user", claims.UserID, "shop", shopID, "stamps", stamps)

	msg := "Stamp collected! 🎉"
	if stamps >= stampsRequired {
		msg = "Card complete! 🏆 You can now redeem your reward!"
	}

	writeProto(w, http.StatusOK, &pb.ClaimStampResponse{
		ShopName:       shopName,
		Stamps:         stamps,
		StampsRequired: stampsRequired,
		Message:        msg,
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
