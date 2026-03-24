package db

import (
	"context"
	"database/sql"
	"log/slog"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

func Init(ctx context.Context, path string) {
	var err error
	DB, err = sql.Open("sqlite", path)
	if err != nil {
		slog.ErrorContext(ctx, "failed to open database", "path", path, "error", err)
		panic(err)
	}
	DB.SetMaxOpenConns(1) // SQLite doesn't support concurrent writes
	slog.InfoContext(ctx, "database opened", "path", path)

	migrate(ctx)
}

func migrate(ctx context.Context) {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL CHECK(role IN ('user','admin')),
			shop_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS shops (
			id TEXT PRIMARY KEY,
			name TEXT UNIQUE NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			reward_description TEXT NOT NULL,
			stamps_required INTEGER NOT NULL DEFAULT 8,
			color TEXT NOT NULL DEFAULT '#6366f1',
			owner_id TEXT NOT NULL REFERENCES users(id),
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// For existing databases that already have the shops table without the UNIQUE constraint:
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_shops_name ON shops(name)`,
		`CREATE TABLE IF NOT EXISTS stamp_cards (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id),
			shop_id TEXT NOT NULL REFERENCES shops(id),
			stamps INTEGER NOT NULL DEFAULT 0,
			redeemed BOOLEAN NOT NULL DEFAULT FALSE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, shop_id, redeemed) ON CONFLICT IGNORE
		)`,
		`CREATE TABLE IF NOT EXISTS stamp_tokens (
			id TEXT PRIMARY KEY,
			shop_id TEXT NOT NULL REFERENCES shops(id),
			token TEXT UNIQUE NOT NULL,
			expires_at DATETIME NOT NULL,
			claimed_by TEXT REFERENCES users(id),
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS stamp_token_claims (
			token_id TEXT NOT NULL REFERENCES stamp_tokens(id),
			user_id TEXT NOT NULL REFERENCES users(id),
			claimed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (token_id, user_id)
		)`,
	}
	for i, s := range statements {
		if _, err := DB.ExecContext(ctx, s); err != nil {
			slog.ErrorContext(ctx, "migration failed", "statement", i, "error", err)
			panic(err)
		}
	}
	slog.InfoContext(ctx, "database migrations complete", "tables", 5)
}

func Close(ctx context.Context) {
	if DB != nil {
		DB.Close()
		slog.InfoContext(ctx, "database connection closed")
	}
}

