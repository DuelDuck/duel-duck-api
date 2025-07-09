package cron

import (
	rcron "github.com/robfig/cron/v3"
	"go.uber.org/fx"
)

func Module() fx.Option {
	return fx.Module("cron",
		fx.Provide(rcron.New),
		fx.Provide(NewCoinCron),
		fx.Provide(NewDuelCron),
		fx.Provide(NewPriorityTrackerCron),
		fx.Provide(NewTaskCron),
		fx.Invoke(
			func(lc fx.Lifecycle, cron *CoinCron) {
				lc.Append(fx.Hook{
					OnStart: cron.start,
					OnStop:  cron.stop,
				})
			},
			func(lc fx.Lifecycle, cron *DuelCron) {
				lc.Append(fx.Hook{
					OnStart: cron.start,
					OnStop:  cron.stop,
				})
			},
			func(lc fx.Lifecycle, cron *PriorityTrackerCron) {
				lc.Append(fx.Hook{
					OnStart: cron.start,
					OnStop:  cron.stop,
				})
			},
			func(lc fx.Lifecycle, cron *TaskCron) {
				lc.Append(fx.Hook{
					OnStart: cron.start,
					OnStop:  cron.stop,
				})
			},
		),
	)
}
