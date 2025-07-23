package repository

import (
	"github.com/google/uuid"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/pkg/repository"
	"go.uber.org/fx"
)

func Module() fx.Option {
	return fx.Module("db",
		fx.Provide(
			CreateDBConnection,
		),
		fx.Provide(
			repository.NewTransactionManager,
		),
		fx.Provide(
			fx.Annotate(
				repository.NewDBWrapper,
				fx.As(
					new(repository.DB),
				)),
		),

		// Custom repositories initialization
		fx.Provide(
			repository.NewGenericRepository[model.User, uuid.UUID],
			NewUserRepository,
		),
		fx.Provide(
			repository.NewGenericRepository[model.Duel, uuid.UUID],
			NewDuelRepository,
		),
		fx.Provide(
			repository.NewGenericRepository[model.Player, uuid.UUID],
			NewPlayerRepository,
		),
		fx.Provide(
			repository.NewGenericRepository[model.Coin, uint64],
			NewCoinRepository,
		),
		fx.Provide(
			NewReferralRepository,
		),
		fx.Provide(
			repository.NewGenericRepository[model.Wallet, uuid.UUID],
			NewWalletRepository,
		),
		fx.Provide(
			repository.NewGenericRepository[model.AdvertiserLink, uuid.UUID],
			NewAdvertiserLinkRepository,
		),
		fx.Provide(
			NewTaskRepository,
		),
		fx.Provide(
			NewFAQRepository,
		),
		fx.Provide(
			NewStatsRepository,
		),
		fx.Provide(
			repository.NewGenericRepository[model.DuelEntity, uint64],
			NewDuelEntityRepository,
		),
		fx.Provide(
			repository.NewGenericRepository[model.ModeratorStats, uuid.UUID],
			NewModeratorStatsRepository,
		),
		fx.Provide(
			repository.NewGenericRepository[model.TransactionType, string],
			NewTransactionRepository,
		),
		fx.Provide(
			repository.NewGenericRepository[model.WalletToken, uuid.UUID],
			NewUserTokenRepository,
		),
		fx.Provide(
			repository.NewGenericRepository[model.Tournament, uuid.UUID],
			NewTournamentRepository,
		),
		fx.Provide(
			NewFileRepository,
		),
	)
}
