package main

import (
	"errors"
	"strings"

	"github.com/bogdan/booking-service/migrations"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // драйвер PostgreSQL
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// runMigrations применяет все новые миграции. Если база уже актуальна, ничего не делает.
func runMigrations(databaseURL string) error {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return err
	}
	// Драйвер pgx/v5 ожидает схему pgx5:// вместо postgres://
	dbURL := strings.Replace(databaseURL, "postgres://", "pgx5://", 1)
	dbURL = strings.Replace(dbURL, "postgresql://", "pgx5://", 1)

	m, err := migrate.NewWithSourceInstance("iofs", src, dbURL)
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}
