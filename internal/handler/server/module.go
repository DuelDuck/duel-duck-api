package server

import (
	"context"
	"github.com/gofiber/fiber/v3"
	"gitlab.com/duel-duck/duel-duck-api/config"
	"go.uber.org/fx"
	"log"
	"net"
)

func Module() fx.Option {
	return fx.Module("handler",
		fx.Provide(
			NewServer,
		),
		fx.Invoke(
			func(lc fx.Lifecycle, app *fiber.App, c *config.Config) {
				lc.Append(fx.Hook{
					OnStart: func(ctx context.Context) error {
						go func() {
							addr := net.JoinHostPort(c.HTTP.Host, c.HTTP.Port)
							if err := app.Listen(addr); err != nil {
								log.Println("failed to start handler listening", err)
							}
						}()

						return nil
					},
					OnStop: func(ctx context.Context) error {
						return app.ShutdownWithContext(ctx)
					},
				},
				)
			},
		),
	)
}
