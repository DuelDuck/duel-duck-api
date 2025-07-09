package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"gitlab.com/duel-duck/duel-duck-api/config"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

const migrationDir = "migrations"

func CreateDBConnection(c *config.Config) (*bun.DB, error) {
	conf, err := pgxpool.ParseConfig(c.PG.GetConnectString())
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), conf)
	if err != nil {
		return nil, err
	}

	sqlDB := stdlib.OpenDBFromPool(pool)
	db := bun.NewDB(sqlDB, pgdialect.New())

	if err = db.Ping(); err != nil {
		return nil, err
	}

	if err = makeMigration(sqlDB, migrationDir, c.PG.Database); err != nil {
		return nil, err
	}

	return db, nil
}

func makeMigration(conn *sql.DB, migrationDir, dbName string) error {
	driver, err := postgres.WithInstance(conn, &postgres.Config{
		DatabaseName: dbName,
	})
	if err != nil {
		return fmt.Errorf("postgres.WithInstance: %s", err.Error())
	}
	mg, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationDir),
		dbName, driver)
	if err != nil {
		return fmt.Errorf("migrate.NewWithDatabaseInstance: %s", err.Error())
	}
	if err = mg.Up(); err != nil {
		if !errors.Is(err, migrate.ErrNoChange) {
			return err
		}
	}
	return nil
}
