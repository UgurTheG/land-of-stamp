// Package handlers implements the HTTP handler functions for all API endpoints.
package handlers

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"land-of-stamp-backend/auth"
	"land-of-stamp-backend/db"
	"land-of-stamp-backend/gen/pb"
	"land-of-stamp-backend/middleware"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Role constants used across handlers.
const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

// protojson marshaler configured to emit fields even when they have zero values,
// so the JSON output always includes every field the frontend expects.
var pjson = protojson.MarshalOptions{EmitUnpopulated: true}

// protojson unmarshaler that accepts unknown fields gracefully.
var pjsonUnmarshal = protojson.UnmarshalOptions{DiscardUnknown: true}

// parseBasicAuth extracts username and password from the Authorization: Basic header.
func parseBasicAuth(r *http.Request) (username, password string, ok bool) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Basic ") {
		return "", "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(header, "Basic "))
	if err != nil {
		return "", "", false
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// setTokenCookie sets the JWT as an HttpOnly, SameSite=Strict cookie.
func setTokenCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "__token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   3 * 24 * 60 * 60, // 3 days
	})
}

// clearTokenCookie removes the auth cookie.
func clearTokenCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "__token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

// Register reads credentials from Authorization: Basic and role from JSON body.
func Register(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	username, password, ok := parseBasicAuth(r)
	if !ok {
		slog.WarnContext(ctx, "register: missing basic auth header")
		jsonError(ctx, w, "missing Authorization: Basic header", http.StatusBadRequest)
		return
	}
	if len(username) < 2 {
		jsonError(ctx, w, "username must be at least 2 characters", http.StatusBadRequest)
		return
	}
	if len(password) < 4 {
		jsonError(ctx, w, "password must be at least 4 characters", http.StatusBadRequest)
		return
	}

	var req pb.RegisterRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
			slog.WarnContext(ctx, "register: failed to decode request body", "error", err)
			jsonError(ctx, w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
			return
		}
	}
	if req.Role != RoleUser && req.Role != RoleAdmin {
		req.Role = RoleUser
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		slog.ErrorContext(ctx, "register: bcrypt failed", "error", err)
		jsonError(ctx, w, "internal error", http.StatusInternalServerError)
		return
	}

	id := uuid.New().String()
	_, err = db.DB.ExecContext(ctx,
		"INSERT INTO users (id, username, password_hash, role) VALUES (?, ?, ?, ?)",
		id, username, string(hash), req.Role,
	)
	if err != nil {
		slog.InfoContext(ctx, "register: username taken", "username", username)
		jsonError(ctx, w, "username already taken", http.StatusConflict)
		return
	}

	token, err := auth.GenerateToken(id, username, req.Role)
	if err != nil {
		slog.ErrorContext(ctx, "register: token generation failed", "error", err)
		jsonError(ctx, w, "failed to generate token", http.StatusInternalServerError)
		return
	}

	setTokenCookie(w, token)
	slog.InfoContext(ctx, "user registered", "id", id, "username", username, "role", req.Role)
	writeProto(ctx, w, http.StatusCreated, &pb.AuthResponse{
		User: &pb.User{Id: id, Username: username, Role: req.Role},
	})
}

// Login reads credentials from Authorization: Basic, validates via bcrypt, sets cookie.
func Login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	username, password, ok := parseBasicAuth(r)
	if !ok {
		slog.WarnContext(ctx, "login: missing basic auth header")
		jsonError(ctx, w, "missing Authorization: Basic header", http.StatusBadRequest)
		return
	}

	var id, passwordHash, role string
	var shopID sql.NullString
	err := db.DB.QueryRowContext(ctx,
		"SELECT id, password_hash, role, shop_id FROM users WHERE username = ?",
		username,
	).Scan(&id, &passwordHash, &role, &shopID)
	if err != nil {
		slog.InfoContext(ctx, "login: user not found", "username", username)
		jsonError(ctx, w, "invalid username or password", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		slog.InfoContext(ctx, "login: wrong password", "username", username)
		jsonError(ctx, w, "invalid username or password", http.StatusUnauthorized)
		return
	}

	token, err := auth.GenerateToken(id, username, role)
	if err != nil {
		slog.ErrorContext(ctx, "login: token generation failed", "error", err)
		jsonError(ctx, w, "failed to generate token", http.StatusInternalServerError)
		return
	}

	user := &pb.User{Id: id, Username: username, Role: role}
	if shopID.Valid {
		user.ShopId = &shopID.String
	}

	setTokenCookie(w, token)
	slog.InfoContext(ctx, "user logged in", "id", id, "username", username, "role", role)
	writeProto(ctx, w, http.StatusOK, &pb.AuthResponse{User: user})
}

// Logout clears the auth cookie.
func Logout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	clearTokenCookie(w)
	slog.InfoContext(ctx, "user logged out")
	writeJSON(ctx, w, http.StatusOK, map[string]string{"status": "logged out"})
}

// GetMe returns the currently authenticated user's profile.
func GetMe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims := middleware.GetUser(r)
	if claims == nil {
		jsonError(ctx, w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var shopID sql.NullString
	if err := db.DB.QueryRowContext(ctx, "SELECT shop_id FROM users WHERE id = ?", claims.UserID).Scan(&shopID); err != nil {
		slog.WarnContext(ctx, "getMe: failed to fetch user shop_id", "user_id", claims.UserID, "error", err)
	}

	user := &pb.User{Id: claims.UserID, Username: claims.Username, Role: claims.Role}
	if shopID.Valid {
		user.ShopId = &shopID.String
	}
	writeProto(ctx, w, http.StatusOK, user)
}

func jsonError(ctx context.Context, w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": msg}); err != nil {
		slog.ErrorContext(ctx, "jsonError: encode failed", "error", err)
	}
}

func writeJSON(ctx context.Context, w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.ErrorContext(ctx, "writeJSON: encode failed", "error", err)
	}
}

// writeProto marshals a proto message to JSON using protojson (camelCase keys).
func writeProto(ctx context.Context, w http.ResponseWriter, code int, msg proto.Message) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	b, err := pjson.Marshal(msg)
	if err != nil {
		slog.ErrorContext(ctx, "writeProto: marshal failed", "error", err)
		return
	}
	if _, err := w.Write(b); err != nil {
		slog.ErrorContext(ctx, "writeProto: write failed", "error", err)
	}
}

// writeProtoList marshals a slice of proto messages as a JSON array with camelCase keys.
func writeProtoList(ctx context.Context, w http.ResponseWriter, msgs []proto.Message) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("[")); err != nil {
		slog.ErrorContext(ctx, "writeProtoList: write failed", "error", err)
		return
	}
	for i, msg := range msgs {
		if i > 0 {
			if _, err := w.Write([]byte(",")); err != nil {
				slog.ErrorContext(ctx, "writeProtoList: write separator failed", "error", err)
				return
			}
		}
		b, err := pjson.Marshal(msg)
		if err != nil {
			slog.ErrorContext(ctx, "writeProtoList: marshal failed", "index", i, "error", err)
			if _, err := w.Write([]byte("null")); err != nil {
				slog.ErrorContext(ctx, "writeProtoList: write null failed", "error", err)
			}
			continue
		}
		if _, err := w.Write(b); err != nil {
			slog.ErrorContext(ctx, "writeProtoList: write failed", "error", err)
			return
		}
	}
	if _, err := w.Write([]byte("]")); err != nil {
		slog.ErrorContext(ctx, "writeProtoList: write closing bracket failed", "error", err)
	}
}

// readProto reads the request body and unmarshals it into a proto message using protojson.
// protojson accepts both camelCase and snake_case field names.
func readProto(r *http.Request, msg proto.Message) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return pjsonUnmarshal.Unmarshal(body, msg)
}
