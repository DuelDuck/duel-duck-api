package repository

import (
	"database/sql"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	ErrDuplicateKeyViolation = "23505"
)

func DuplicateKeyViolation(err error) bool {
	if err == nil {
		return false
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == ErrDuplicateKeyViolation {
		return true
	}

	return false
}

func IsErrNoRows(err error) bool {
	if err == nil {
		return false
	}

	return errors.Is(err, sql.ErrNoRows)
}
