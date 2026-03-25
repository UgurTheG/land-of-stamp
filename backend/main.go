package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"land-of-stamp-backend/constants"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"land-of-stamp-backend/auth"
	"land-of-stamp-backend/db"
	"land-of-stamp-backend/docs"
	"land-of-stamp-backend/gen/pb/pbconnect"
	"land-of-stamp-backend/interceptor"
	"land-of-stamp-backend/middleware"
	"land-of-stamp-backend/service"

	"connectrpc.com/connect"
)

func main() {
	ctx := context.Background()

	initLogging()
	initJWT(ctx)

	// ── SQLite database ──
	dbPath := os.Getenv(constants.EnvDBPath)
	if dbPath == "" {
		dbPath = constants.DefaultDBPath
	}
	db.Init(ctx, dbPath)
	defer db.Close(ctx)

	mux := buildMux()
	serveFrontend(ctx, mux)

	port := os.Getenv(constants.EnvPort)
	if port == "" {
		port = constants.DefaultPort
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           middleware.RequestLog(middleware.CORS(mux)),
		ReadHeaderTimeout: 10 * time.Second,
	}
	slog.InfoContext(ctx, "server starting (gRPC + Connect + gRPC-Web)", "port", port, "url", "http://localhost:"+port)
	if err := srv.ListenAndServe(); err != nil {
		slog.ErrorContext(ctx, "server failed", "error", err)
	}
}

func initLogging() {
	var level slog.Level
	switch os.Getenv(constants.EnvLogLevel) {
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
}

func initJWT(ctx context.Context) {
	secret := os.Getenv(constants.EnvJWTSecret)
	if secret == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			slog.ErrorContext(ctx, "failed to generate random JWT secret", "error", err)
			os.Exit(1)
		}
		secret = hex.EncodeToString(b)
		slog.InfoContext(ctx, "generated random JWT secret (set JWT_SECRET env var to persist across restarts)")
	}
	auth.Init(secret)
}

func buildMux() *http.ServeMux {
	mux := http.NewServeMux()

	// Shared ConnectRPC handler options: auth interceptor.
	opts := connect.WithInterceptors(interceptor.NewAuthInterceptor())

	// ── gRPC services (each serves Connect + gRPC + gRPC-Web) ──
	authPath, authHandler := pbconnect.NewAuthServiceHandler(&service.AuthService{}, opts)
	shopPath, shopHandler := pbconnect.NewShopServiceHandler(&service.ShopService{}, opts)
	stampPath, stampHandler := pbconnect.NewStampServiceHandler(&service.StampService{}, opts)
	docsPath, docsHandler := pbconnect.NewDocsServiceHandler(&docs.DocsService{}, opts)

	mux.Handle(authPath, authHandler)
	mux.Handle(shopPath, shopHandler)
	mux.Handle(stampPath, stampHandler)
	mux.Handle(docsPath, docsHandler)

	// ── OAuth login (plain HTTP — providers require GET redirects) ──
	oauthSvc := service.NewOAuthService()
	oauthSvc.Register(mux)

	// ── Test seed endpoint (only when TEST_SEED=true) ──
	service.RegisterTestSeed(mux)

	return mux
}

func serveFrontend(ctx context.Context, mux *http.ServeMux) {
	distDir := os.Getenv(constants.EnvDistDir)
	if distDir == "" {
		distDir = constants.DefaultDistDir
	}
	if _, err := os.Stat(distDir); err == nil {
		slog.InfoContext(ctx, "serving frontend", "dir", distDir)
		fs := http.FileServer(http.Dir(distDir))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			path := filepath.Join(distDir, filepath.Clean(r.URL.Path))
			if _, err := os.Stat(path); err != nil {
				http.ServeFile(w, r, filepath.Join(distDir, "index.html"))
				return
			}
			fs.ServeHTTP(w, r)
		})
	}
}
