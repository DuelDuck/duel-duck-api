package repository

import (
	"context"
	"database/sql"
	"errors"
	"github.com/uptrace/bun"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/pkg/repository"
)

type CoinRepository struct {
	repository.Generic[model.Coin, uint64]
}

func NewCoinRepository(
	genericRepository repository.Generic[model.Coin, uint64],
) *CoinRepository {
	return &CoinRepository{
		Generic: genericRepository,
	}
}

func (r *CoinRepository) ExistsWhereID(
	ctx context.Context,
	coinID int64,
) (bool, error) {
	coin := new(model.Coin)

	ok, err := r.DB.NewSelect().
		Model(coin).
		Where("id = ?", coinID).
		Exists(ctx)
	if err != nil {
		return false, err
	}

	return ok, nil
}

func (r *CoinRepository) GetAllCoins(
	ctx context.Context,
	options *repository.Options,
) ([]model.Coin, error) {
	coins := make([]model.Coin, 0)

	q := r.DB.NewSelect().
		Model(&coins)
	q = options.Apply(q)

	if err := q.Scan(ctx); err != nil {
		return nil, err
	}

	return coins, nil
}

func (r *CoinRepository) GetCoinsByCMCIDs(
	ctx context.Context,
	cmcIDs []uint64,
) ([]model.Coin, error) {
	coins := make([]model.Coin, 0, len(cmcIDs))

	err := r.DB.NewSelect().
		Model(&coins).
		Where("coins.id in (?)", bun.In(cmcIDs)).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	return coins, nil
}

func (r *CoinRepository) RefreshSolanaTokens(
	ctx context.Context,
	tokens []model.JupTokensResp,
) error {
	_, err := r.DB.NewInsert().
		Model(&tokens).
		On(`CONFLICT (mint) DO UPDATE SET
    		name = EXCLUDED.name,
    		symbol = EXCLUDED.symbol,
    		decimals = EXCLUDED.decimals,
    		image_url = EXCLUDED.image_url,
			daily_volume = EXCLUDED.daily_volume`).
		Exec(ctx)

	return err
}

func (r *CoinRepository) GetSolanaTokenByName(
	ctx context.Context,
	token string,
) ([]model.SolanaToken, error) {
	tokenParam := "%" + token + "%"
	tokenInfo := make([]model.SolanaToken, 0)

	err := r.DB.NewSelect().
		Model(&tokenInfo).
		WhereOr("name ILIKE ?", tokenParam).
		WhereOr("symbol ILIKE ?", tokenParam).
		Order("daily_volume DESC").
		Limit(10).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	return tokenInfo, nil
}

func (r *CoinRepository) FindSolanaTokenByMint(
	ctx context.Context,
	mint string,
) (*model.SolanaToken, error) {
	token := new(model.SolanaToken)

	err := r.DB.NewSelect().
		Model(token).
		WhereOr("mint = ?", mint).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return token, nil
}
