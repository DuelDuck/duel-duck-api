package client

import (
	"gitlab.com/duel-duck/duel-duck-api/internal/client/jupiter"
	"gitlab.com/duel-duck/duel-duck-api/internal/client/solana"
	"gitlab.com/duel-duck/duel-duck-api/internal/client/solscan"
	"go.uber.org/fx"
)

func Module() fx.Option {
	return fx.Module("Clients",
		fx.Provide(
			jupiter.NewClient,
			solscan.NewClient,
			solana.NewClient,
		),
	)
}
