// Package db provides GORM-based database initialization, migration, and lifecycle management.
package db

import (
	"context"
	"log/slog"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB is the global GORM database connection.
var DB *gorm.DB

// Init opens the SQLite database at the given path and runs auto-migrations.
func Init(ctx context.Context, path string) {
	var err error
	DB, err = gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Silent),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to open database", "path", path, "error", err)
		panic(err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		slog.ErrorContext(ctx, "failed to get underlying sql.DB", "error", err)
		panic(err)
	}
	sqlDB.SetMaxOpenConns(1) // SQLite doesn't support concurrent writes
	slog.InfoContext(ctx, "database opened", "path", path)

	migrate(ctx)
}

func migrate(ctx context.Context) {
	if err := DB.AutoMigrate(
		&User{},
		&Shop{},
		&StampCard{},
		&StampToken{},
		&StampTokenClaim{},
	); err != nil {
		slog.ErrorContext(ctx, "auto-migration failed", "error", err)
		panic(err)
	}

	// Partial unique index: at most one active card per user per shop.
	// GORM tags cannot express WHERE clauses, so we create it manually.
	if err := DB.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_one_active_card_per_user_shop
		ON stamp_cards (user_id, shop_id)
		WHERE redeemed = false AND deleted_at IS NULL
	`).Error; err != nil {
		slog.ErrorContext(ctx, "failed to create partial unique index", "error", err)
		panic(err)
	}

	slog.InfoContext(ctx, "database migrations complete")
}

// Close gracefully closes the database connection.
func Close(ctx context.Context) {
	if DB != nil {
		sqlDB, err := DB.DB()
		if err != nil {
			slog.ErrorContext(ctx, "failed to get underlying sql.DB for close", "error", err)
			return
		}
		if err := sqlDB.Close(); err != nil {
			slog.ErrorContext(ctx, "failed to close database", "error", err)
		}
		slog.InfoContext(ctx, "database connection closed")
	}
}
