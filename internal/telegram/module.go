package telegram

import (
	"gitlab.com/duel-duck/duel-duck-api/config"
	"go.uber.org/fx"
)

func Module() fx.Option {
	return fx.Module("tg_bot",
		fx.Provide(
			NewTgStatsBot,
		),
		fx.Provide(
			NewWelcomeTGBot,
		),
		fx.Provide(
			NewTournamentTGBot,
		),
		fx.Provide(
			NewFAQTGBot,
		),
		fx.Invoke(
			func(lc fx.Lifecycle, c *config.Config, bot *StatsTgBot) {
				if c.App.Environment == config.EnvironmentProduction {
					lc.Append(fx.Hook{
						OnStart: bot.start,
						OnStop:  bot.stop,
					})
				}
			},
		),
		fx.Invoke(
			func(lc fx.Lifecycle, c *config.Config, bot *WelcomeTGBot) {
				//if c.App.Environment == config.EnvironmentProduction {
				lc.Append(fx.Hook{
					OnStart: bot.start,
					OnStop:  bot.stop,
				})
				//}
			},
		),
		fx.Invoke(
			func(lc fx.Lifecycle, c *config.Config, bot *TournamentTGBot) {
				//if c.App.Environment == config.EnvironmentProduction {
				lc.Append(fx.Hook{
					OnStart: bot.start,
					OnStop:  bot.stop,
				})
				//}
			},
		),
		fx.Invoke(
			func(lc fx.Lifecycle, c *config.Config, bot *FAQTGBot) {
				lc.Append(fx.Hook{
					OnStart: bot.start,
					OnStop:  bot.stop,
				})
			},
		),
	)
}
