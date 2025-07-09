package v1

import (
	"github.com/gofiber/fiber/v3"
	"go.uber.org/fx"

	"gitlab.com/duel-duck/duel-duck-api/internal/handler/v1/swagger"
)

func Module() fx.Option {
	return fx.Module("v1",
		fx.Provide(
			NewAuthHandler,
			NewDuelHandler,
			NewTaskHandler,
			NewFAQHandler,
			NewStatsHandler,
			NewAdminHandler,
			NewUserHandler,
			NewWalletHandler,
			NewCoinHandler,
			NewTournament,
			swagger.NewSwaggerHandler,
		),
		fx.Invoke(func(app *fiber.App, authHandler *AuthHandler) {
			authHandler.RegisterRoutes(app)
		}),
		fx.Invoke(func(app *fiber.App, authHandler *AuthHandler, duelHandler *DuelHandler) {
			duelHandler.RegisterRoutes(app, authHandler)
		}),
		fx.Invoke(func(app *fiber.App, authHandler *AuthHandler, taskHandler *TaskHandler) {
			taskHandler.RegisterRoutes(app, authHandler)
		}),
		fx.Invoke(func(app *fiber.App, authHandler *AuthHandler, faqHandler *FAQHandler) {
			faqHandler.RegisterRoutes(app, authHandler)
		}),
		fx.Invoke(func(app *fiber.App, authHandler *AuthHandler, userHandler *StatsHandler) {
			userHandler.RegisterRoutes(app, authHandler)
		}),
		fx.Invoke(func(app *fiber.App, authHandler *AuthHandler, adminHandler *AdminHandler) {
			adminHandler.RegisterRoutes(app, authHandler)
		}),
		fx.Invoke(func(app *fiber.App, authHandler *AuthHandler, userHandler *UserHandler) {
			userHandler.RegisterRoutes(app, authHandler)
		}),
		fx.Invoke(func(app *fiber.App, authHandler *AuthHandler, walletHandler *WalletHandler) {
			walletHandler.RegisterRoutes(app, authHandler)
		}),
		fx.Invoke(func(app *fiber.App, auth *AuthHandler, admin *AdminHandler, handler *TournamentHandler) {
			handler.RegisterRoutes(app, auth, admin)
		}),
		fx.Invoke(func(app *fiber.App, coinHandler *CoinHandler) {
			coinHandler.RegisterRoutes(app)
		}),
		fx.Invoke(func(app *fiber.App, swaggerHandler *swagger.Handler) {
			swaggerHandler.RegisterRoutes(app)
		}),
	)
}
