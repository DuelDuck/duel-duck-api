package service

import (
	"gitlab.com/duel-duck/duel-duck-api/pkg/mailer"
	"gitlab.com/duel-duck/duel-duck-api/pkg/sigtracker"
	"go.uber.org/fx"
)

func Module() fx.Option {
	return fx.Module("AuthService",
		fx.Provide(
			fx.Annotate(
				mailer.NewMailer,
				fx.As(new(mailer.Mailer)),
			),
		),
		fx.Provide(
			sigtracker.NewTransactionTracker,
		),
		fx.Provide(
			NewAuthService,
			NewUserService,
			NewJWTService,
			NewDuelService,
			NewCoinService,
			NewTaskService,
			NewFAQService,
			NewWalletService,
			NewPriorityTracker,
			NewTournamentService,
			NewFileService,
		),
		fx.Invoke(
			func(lc fx.Lifecycle, faqService *FAQService) {
				lc.Append(fx.Hook{
					OnStop: faqService.stop,
				})
			},
		),
	)
}
