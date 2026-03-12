package database

import (
	"errors"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/kerpe-l/metrics-alerting-service/migrations"
)

// RunMigrations применяет все миграции к базе данных.
// dsn — строка подключения вида "postgres://user:pass@host:5432/db".
func RunMigrations(dsn string) error {
	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return err
	}

	// golang-migrate для драйвера pgx/v5 ожидает схему "pgx5://"
	m, err := migrate.NewWithSourceInstance("iofs", source, withPgx5Scheme(dsn))
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}

	return nil
}

// withPgx5Scheme заменяет схему "postgres://" или "postgresql://" на "pgx5://",
// чтобы golang-migrate использовал драйвер pgx/v5.
func withPgx5Scheme(dsn string) string {
	for _, prefix := range []string{"postgres://", "postgresql://"} {
		if len(dsn) > len(prefix) && dsn[:len(prefix)] == prefix {
			return "pgx5://" + dsn[len(prefix):]
		}
	}
	return dsn
}
