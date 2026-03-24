package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"os"

	"land-of-stamp-backend/auth"
	"land-of-stamp-backend/db"
	"land-of-stamp-backend/handlers"
	"land-of-stamp-backend/middleware"
)

func main() {
	ctx := context.Background()

	// ── Configure slog (JSON structured logging) ──
	var level slog.Level
	switch os.Getenv("LOG_LEVEL") {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))

	// ── JWT secret ──
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		// Try to read a previously-generated secret from a local file so that
		// tokens survive backend restarts during development.
		const secretFile = ".jwt-secret"
		if data, err := os.ReadFile(secretFile); err == nil && len(data) > 0 {
			secret = string(data)
			slog.InfoContext(ctx, "loaded JWT secret from file", "file", secretFile)
		} else {
			b := make([]byte, 32)
			if _, err := rand.Read(b); err != nil {
				slog.ErrorContext(ctx, "failed to generate random JWT secret", "error", err)
				os.Exit(1)
			}
			secret = hex.EncodeToString(b)
			if err := os.WriteFile(secretFile, []byte(secret), 0600); err == nil {
				slog.InfoContext(ctx, "generated and saved JWT secret", "file", secretFile)
			} else {
				slog.WarnContext(ctx, "generated random JWT secret (could not persist)", "error", err)
			}
		}
	}
	auth.Init(secret)

	// ── SQLite database ──
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "land-of-stamp.db"
	}
	db.Init(ctx, dbPath)
	defer db.Close(ctx)

	mux := http.NewServeMux()

	// ── Public routes ──
	mux.HandleFunc("POST /api/auth/register", handlers.Register)
	mux.HandleFunc("POST /api/auth/login", handlers.Login)
	mux.HandleFunc("POST /api/auth/logout", handlers.Logout)
	mux.HandleFunc("GET /api/shops", handlers.ListShops)

	// ── Authenticated routes ──
	authed := http.NewServeMux()
	authed.HandleFunc("GET /api/auth/me", handlers.GetMe)
	authed.HandleFunc("GET /api/users/me/cards", handlers.GetMyCards)
	authed.HandleFunc("POST /api/cards/{id}/redeem", handlers.RedeemCard)
	authed.HandleFunc("POST /api/stamps/claim", handlers.ClaimStamp)

	// ── Admin routes ──
	admin := http.NewServeMux()
	admin.HandleFunc("POST /api/shops", handlers.CreateShop)
	admin.HandleFunc("PUT /api/shops/{id}", handlers.UpdateShop)
	admin.HandleFunc("GET /api/shops/mine", handlers.GetMyShops)
	admin.HandleFunc("GET /api/shops/{id}/cards", handlers.GetShopCards)
	admin.HandleFunc("POST /api/shops/{id}/stamps", handlers.GrantStamp)
	admin.HandleFunc("PATCH /api/shops/{id}/stamps", handlers.UpdateStampCount)
	admin.HandleFunc("POST /api/shops/{id}/stamp-token", handlers.CreateStampToken)
	admin.HandleFunc("GET /api/users/customers", handlers.ListCustomers)

	// ── Mount with middleware ──
	mux.Handle("/api/auth/me", middleware.Auth(authed))
	mux.Handle("/api/users/me/", middleware.Auth(authed))
	mux.Handle("/api/cards/", middleware.Auth(authed))
	mux.Handle("/api/stamps/", middleware.Auth(authed))
	mux.Handle("GET /api/shops/mine", middleware.Auth(middleware.AdminOnly(admin)))
	mux.Handle("PUT /api/shops/{id}", middleware.Auth(middleware.AdminOnly(admin)))
	mux.Handle("/api/shops/{id}/", middleware.Auth(middleware.AdminOnly(admin)))
	mux.Handle("/api/users/customers", middleware.Auth(middleware.AdminOnly(admin)))
	mux.Handle("POST /api/shops", middleware.Auth(middleware.AdminOnly(admin)))

	// ── Serve frontend static files in production ──
	distDir := os.Getenv("DIST_DIR")
	if distDir == "" {
		distDir = "../frontend/dist"
	}
	if _, err := os.Stat(distDir); err == nil {
		slog.InfoContext(ctx, "serving frontend", "dir", distDir)
		fs := http.FileServer(http.Dir(distDir))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			path := distDir + "/" + r.URL.Path
			if _, err := os.Stat(path); err != nil {
				http.ServeFile(w, r, distDir+"/index.html")
				return
			}
			fs.ServeHTTP(w, r)
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	handler := middleware.RequestLog(middleware.CORS(mux))
	slog.InfoContext(ctx, "server starting", "port", port, "url", "http://localhost:"+port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		slog.ErrorContext(ctx, "server failed", "error", err)
		os.Exit(1)
	}
}

