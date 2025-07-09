package repository

import (
	"context"
	"database/sql"
	"github.com/uptrace/bun"
)

type TransactionManager struct {
	db     *bun.DB
	txOpts *sql.TxOptions
}

func NewTransactionManager(
	db *bun.DB,
) *TransactionManager {
	opts := &sql.TxOptions{
		Isolation: sql.LevelDefault,
		ReadOnly:  false,
	}

	return &TransactionManager{
		db:     db,
		txOpts: opts,
	}
}

func (m *TransactionManager) WithinTransaction(
	ctx context.Context,
	query func(context.Context, bun.Tx) error,
) (err error) {
	tx, err := m.db.BeginTx(ctx, m.txOpts)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			rbErr := tx.Rollback()
			if rbErr != nil {
				err = rbErr
			}
		}
	}()

	err = query(ctx, tx)
	if err != nil {
		return err
	}

	err = tx.Commit()

	return err
}
