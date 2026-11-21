package migration

import (
	"fmt"
	"log/slog"

	"gorm.io/gorm"
	"pulseframe-review/internal"
)

func Migrate(db *gorm.DB) error {
	models := internal.Models()
	slog.Info("starting database migration", "models", len(models))

	if err := db.AutoMigrate(models...); err != nil {
		return fmt.Errorf("auto migrate failed: %w", err)
	}

	slog.Info("database migration completed")
	return nil
}
