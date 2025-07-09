package repository

import (
	"context"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/pkg/repository"
)

type WalletTokenRepository struct {
	DB repository.DB
}

func NewUserTokenRepository(
	db repository.DB,
) *WalletTokenRepository {
	return &WalletTokenRepository{
		DB: db,
	}
}

func (r *WalletTokenRepository) WithTx(tx bun.Tx) *WalletTokenRepository {
	return &WalletTokenRepository{DB: r.DB.WithTx(tx)}
}

func (r *WalletTokenRepository) FindTokens(
	ctx context.Context,
	opts *repository.Options,
	userID uuid.UUID,
) ([]model.WalletToken, error) {
	tokens := make([]model.WalletToken, 0)

	q := r.DB.NewSelect().
		Model(&tokens).
		Where("user_id = ?", userID)
	q = opts.Apply(q)

	if err := q.Scan(ctx); err != nil {
		return nil, err
	}

	return tokens, nil
}

func (r *WalletTokenRepository) Update(
	ctx context.Context,
	userID uuid.UUID,
	token *model.WalletTokenEditReq,
) error {
	_, err := r.DB.NewUpdate().
		Model(token).
		Where("user_id = ?", userID).
		Where("mint = ?", token.Mint).
		OmitZero().
		Exec(ctx)

	return err
}

func (r *WalletTokenRepository) Create(
	ctx context.Context,
	token *model.WalletToken,
) error {
	_, err := r.DB.NewInsert().
		Model(token).
		Exec(ctx)

	return err
}

func (r *WalletTokenRepository) CreateBulk(
	ctx context.Context,
	tokens []model.WalletToken,
) error {
	_, err := r.DB.NewInsert().
		Model(&tokens).
		Exec(ctx)

	return err
}

func (r *WalletTokenRepository) Delete(
	ctx context.Context,
	userID uuid.UUID,
	mint string,
) error {
	_, err := r.DB.NewDelete().
		Model((*model.WalletToken)(nil)).
		Where("user_id = ?", userID).
		Where("mint = ?", mint).
		Exec(ctx)

	return err
}
