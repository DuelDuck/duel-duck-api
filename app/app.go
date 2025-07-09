package app

import (
	"gitlab.com/duel-duck/duel-duck-api/config"
	"gitlab.com/duel-duck/duel-duck-api/internal/client"
	"gitlab.com/duel-duck/duel-duck-api/internal/cron"
	"gitlab.com/duel-duck/duel-duck-api/internal/handler/server"
	v1 "gitlab.com/duel-duck/duel-duck-api/internal/handler/v1"
	"gitlab.com/duel-duck/duel-duck-api/internal/service"
	"gitlab.com/duel-duck/duel-duck-api/internal/storage/cache"
	"gitlab.com/duel-duck/duel-duck-api/internal/storage/cypher"
	"gitlab.com/duel-duck/duel-duck-api/internal/storage/repository"
	"gitlab.com/duel-duck/duel-duck-api/internal/telegram"
	auth "gitlab.com/duel-duck/duel-duck-api/pkg/jwt"
	"gitlab.com/duel-duck/duel-duck-api/pkg/logger"
	"go.uber.org/fx"
)

func Build() *fx.App {
	return fx.New(
		fx.Options(
			config.Module,
			logger.Module,
		),
		auth.Module(),

		repository.Module(),
		cache.Module(),
		cypher.Module(),

		client.Module(),

		service.Module(),
		server.Module(),

		telegram.Module(),
		v1.Module(),
		cron.Module(),
	)
}
