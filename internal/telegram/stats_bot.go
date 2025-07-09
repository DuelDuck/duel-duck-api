package telegram

import (
	"context"
	"gitlab.com/duel-duck/duel-duck-api/config"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/internal/storage/repository"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
	"time"
)

type StatsTgBot struct {
	statsBot        *tele.Bot
	statsRepository *repository.StatsRepository
	userRepository  *repository.UserRepository
	log             *zap.Logger
	environment     string
}

func NewTgStatsBot(
	c *config.Config,
	log *zap.Logger,
	statsRepository *repository.StatsRepository,
	userRepository *repository.UserRepository,
) (*StatsTgBot, error) {
	poller := &tele.LongPoller{
		Timeout: time.Second * 1,
	}

	statsBot, err := tele.NewBot(tele.Settings{
		Token:  c.Telegram.StatsBotTgAPIKey,
		Poller: poller,
	})

	if err != nil {
		return nil, err
	}

	return &StatsTgBot{
		statsBot:        statsBot,
		statsRepository: statsRepository,
		userRepository:  userRepository,
		environment:     c.App.Environment,
		log:             log,
	}, nil
}

func (b *StatsTgBot) SendDailyStats(c tele.Context) error {
	usersToday, err := b.statsRepository.CountTodayNewUsers(context.Background())
	if err != nil {
		return err
	}

	users, err := b.userRepository.CountUsers(context.Background())
	if err != nil {
		return err
	}

	duelWinners, err := b.statsRepository.CountDuelWinners(context.Background())
	if err != nil {
		return err
	}

	duelWinnersToday, err := b.statsRepository.CountDuelWinnersToday(context.Background())
	if err != nil {
		return err
	}

	completedDuels, err := b.statsRepository.CountCompletedDuels(context.Background())
	if err != nil {
		return err
	}

	completedDuelsToday, err := b.statsRepository.CountCompletedDuelsToday(context.Background())
	if err != nil {
		return err
	}

	duels, err := b.statsRepository.CountDuels(context.Background())
	if err != nil {
		return err
	}

	duelsToday, err := b.statsRepository.CountDuelsToday(context.Background())
	if err != nil {
		return err
	}

	dailyStats := &model.DailyStats{
		TotalUsers:    users,
		TodayNewUsers: usersToday,

		TotalUserWin: duelWinners,
		TodayUserWin: duelWinnersToday,

		TotalCreatedDuels: duels,
		TodayCreatedDuels: duelsToday,

		TotalCompleteDuels: completedDuels,
		TodayCompleteDuels: completedDuelsToday,
	}

	return c.Send(dailyStats.TgMessageFmt(), tele.ModeMarkdownV2)
}

func (b *StatsTgBot) SendWelcomeMessage(c tele.Context) error {
	//welcomeText := "*Welcome to Duel Duck\\!* 🦆\n\nAt first, it might look like another clicker game, but it’s not\\! 😂\n\nWe’re building a huge WEB3 prediction platform\\. Here’s our plan\\:\n\n🐤 We’ve released the MVP of Duel Duck\\.\n🐥 We’ve started a clicker game to onboard you to our platform\\.\n🐣 In September, we’ll add new game features\\.\n🐣 In December, we’ll have a real token airdrop and get ready for it\\.\n\nThe Ducks will lead you into the magical world of WEB3 and Duck Points\\.\n*Join our community\\: https://t\\.me/duelduck*"

	mUP := &tele.ReplyMarkup{
		InlineKeyboard: [][]tele.InlineButton{
			{
				tele.InlineButton{
					Unique: "launch",
					Text:   "Let's Go Play! 🚀",
					URL:    "https://t.me/DuelDuck_bot/quack",
				},
			},
		},
	}

	photo := &tele.Photo{
		File: tele.FromURL("https://imgur.com/UFbJ8S8"),
		//Caption: welcomeText,
	}

	if err := c.Send(photo, mUP, tele.ModeMarkdownV2); err != nil {
		return err
	}

	return nil
}

func (b *StatsTgBot) start(_ context.Context) error {
	b.RegisterRoutes()

	go func() {
		b.statsBot.Start()
	}()

	return nil
}

func (b *StatsTgBot) stop(_ context.Context) error {
	b.statsBot.Stop()
	return nil
}

func (b *StatsTgBot) RegisterRoutes() {
	b.statsBot.Handle("/start",
		func(c tele.Context) error {
			if err := b.SendWelcomeMessage(c); err != nil {
				b.log.Error("failed to send welcome message", zap.Error(err))
				return err
			}

			return nil
		})

	b.statsBot.Handle("/stats",
		func(c tele.Context) error {
			if err := b.SendDailyStats(c); err != nil {
				b.log.Error("failed to send daily stats", zap.Error(err))
				return err
			}

			return nil
		})
}
