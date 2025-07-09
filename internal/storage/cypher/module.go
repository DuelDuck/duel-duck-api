package cypher

import (
	"go.uber.org/fx"
)

func Module() fx.Option {
	return fx.Module("cypher",
		fx.Provide(
			CreateCypherDBClient,
		),
		fx.Provide(
			NewPrivateKeyRepository,
		),
	)
}
