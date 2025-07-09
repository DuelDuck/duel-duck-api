package repository

import (
	"context"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/pkg/repository"
)

type TransactionRepository struct {
	repository.Generic[model.TransactionType, string]
}

func (r *TransactionRepository) WithTx(tx bun.Tx) *TransactionRepository {
	return &TransactionRepository{Generic: r.Generic.WithTx(tx)}
}

func NewTransactionRepository(
	genericRepository repository.Generic[model.TransactionType, string],
) *TransactionRepository {
	return &TransactionRepository{
		Generic: genericRepository,
	}
}

func (r *TransactionRepository) BulkInsert(
	ctx context.Context,
	transactions []model.TransactionType,
) error {
	_, err := r.DB.NewInsert().
		Model(&transactions).
		On("CONFLICT (signature) DO NOTHING").
		Exec(ctx)

	return err
}

func (r *TransactionRepository) Create(ctx context.Context, model *model.TransactionType) error {
	_, err := r.DB.NewInsert().
		Model(model).
		On("CONFLICT (signature) DO NOTHING").
		Exec(ctx)

	return err
}

func (r *TransactionRepository) BulkInsertWithSameTxType(
	ctx context.Context,
	txType uint8,
	signatures []string,
) error {
	// unnest postgresql function used to unpack array
	// in order to escape for loop usage for data preparation

	_, err := r.DB.NewRaw(`
	INSERT INTO transactions (signature, tx_type)
	SELECT unnest(?::varchar(88)[]), ? 
	ON CONFLICT DO NOTHING`,
		pgdialect.Array(signatures), txType).Exec(ctx)

	return err
}

func (r *TransactionRepository) GetTransactionsBySignatures(
	ctx context.Context,
	signatures []string,
) ([]model.TransactionType, error) {
	txTypes := make([]model.TransactionType, 0, len(signatures))

	err := r.DB.NewSelect().
		Model(&txTypes).
		Where("signature = ANY(?)", pgdialect.Array(signatures)).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	return txTypes, nil
}
