package repository

import (
	"github.com/google/uuid"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/pkg/repository"
)

type WalletRepository struct {
	repository.Generic[model.Wallet, uuid.UUID]
}

func NewWalletRepository(
	genericRepository repository.Generic[model.Wallet, uuid.UUID],
) *WalletRepository {
	return &WalletRepository{
		Generic: genericRepository,
	}
}
