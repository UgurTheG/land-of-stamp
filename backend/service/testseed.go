// Package service — test-only user seed endpoint.
//
// Enabled only when TEST_SEED=true. Creates a user directly in the DB
// without passwords or OAuth, and returns a JWT cookie + user JSON.
// This replaces the removed Register RPC for e2e test setup.
package service

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"

	"land-of-stamp-backend/auth"
	"land-of-stamp-backend/db"

	"github.com/google/uuid"
)

// RegisterTestSeed mounts POST /test/seed-user if TEST_SEED=true.
func RegisterTestSeed(mux *http.ServeMux) {
	if os.Getenv("TEST_SEED") != "true" {
		return
	}
	slog.Warn("test seed endpoint enabled — do NOT use in production")

	mux.HandleFunc("POST /test/seed-user", handleSeedUser)
}

type seedRequest struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}

type seedResponse struct {
	User seedUserResponse `json:"user"`
}

type seedUserResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

func handleSeedUser(w http.ResponseWriter, r *http.Request) {
	var req seedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}
	if req.Username == "" {
		http.Error(w, `{"error":"username required"}`, http.StatusBadRequest)
		return
	}
	if req.Role != "user" && req.Role != "admin" {
		req.Role = "user"
	}

	user := db.User{
		UUID:     uuid.New(),
		Username: req.Username,
		Role:     req.Role,
	}
	if err := db.DB.Create(&user).Error; err != nil {
		slog.Error("test seed: create user failed", "error", err)
		http.Error(w, `{"error":"user creation failed (duplicate?)"}`, http.StatusConflict)
		return
	}

	uid := user.UUID.String()
	token, err := auth.GenerateToken(uid, user.Username, user.Role)
	if err != nil {
		slog.Error("test seed: token generation failed", "error", err)
		http.Error(w, `{"error":"token generation failed"}`, http.StatusInternalServerError)
		return
	}

	SetTokenCookie(w.Header(), token)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(seedResponse{
		User: seedUserResponse{
			ID:       uid,
			Username: user.Username,
			Role:     user.Role,
		},
	})
}

