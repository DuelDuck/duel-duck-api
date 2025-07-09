package telegram

import (
	"context"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gitlab.com/duel-duck/duel-duck-api/config"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/internal/service"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	"go.uber.org/zap"
	"strconv"
	"strings"
	"sync"
)

const (
	GetTournamentOffersCommand = "get_all"
)

type TournamentTGBot struct {
	b                 *tgbotapi.BotAPI
	TournamentService *service.TournamentService
	allowOrigins      map[int64]int64
	mu                sync.RWMutex
}

func NewTournamentTGBot(
	c *config.Config,
	tournamentService *service.TournamentService,
) (*TournamentTGBot, error) {
	bot, err := tgbotapi.NewBotAPI(c.Telegram.TournamentBot.BotTgAPIKey)
	if err != nil {
		return nil, err
	}

	allowTelegramIDs := strings.Split(c.Telegram.TournamentBot.AllowTelegramIDs, ",")

	allowOrigins := make(map[int64]int64, len(allowTelegramIDs))
	for _, id := range allowTelegramIDs {
		parsedID, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return nil, apperrors.Internal("failed to parse telegram id on tournament bot init", err)
		}

		allowOrigins[parsedID] = parsedID
	}

	tournamentBot := &TournamentTGBot{
		b:                 bot,
		TournamentService: tournamentService,
		allowOrigins:      allowOrigins,
	}

	return tournamentBot, nil
}

func (b *TournamentTGBot) start(_ context.Context) error {
	if b == nil {
		return apperrors.Internal("start() called on nil WelcomeTGBot")
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.b.GetUpdatesChan(u)

	go b.WatchUpdates(updates)

	return nil
}

func (b *TournamentTGBot) WatchUpdates(updates tgbotapi.UpdatesChannel) {
	for update := range updates {
		func() {
			defer func() {
				err := recover()
				if err == nil {
					return
				}
			}()

			if update.Message == nil {
				return
			}

			if !b.isAllowedOrigin(update.Message.From.ID) {
				if err := b.DenyPermission(update.Message); err != nil {
					zap.L().Error("failed to send message on permission deny", zap.Error(err))
				}
			}

			if !update.Message.IsCommand() {
				if err := b.UnknownCommand(update.Message); err != nil {
					zap.L().Error("welcome bot: failed to send message", zap.Error(err))
				}
			}

			command := update.Message.Command()

			switch command {
			case StartCommand:
				if err := b.StartCommandHandler(update.Message); err != nil {
					zap.L().Error("tournament bot: handle start failed", zap.Error(err))
				}
				return
			case GetTournamentOffersCommand:
				if err := b.GetAllCommandHandler(update.Message); err != nil {
					zap.L().Error("tournament bot: handle get_all failed", zap.Error(err))
				}
				return
			}
		}()
	}
}

func (b *TournamentTGBot) stop(_ context.Context) error {
	b.b.StopReceivingUpdates()
	return nil
}

func (b *TournamentTGBot) UnknownCommand(message *tgbotapi.Message) error {
	msg := tgbotapi.NewMessage(message.Chat.ID, UnknownCommandText)
	_, err := b.b.Send(msg)

	return err
}

func (b *TournamentTGBot) isAllowedOrigin(userID int64) bool {
	_, ok := b.allowOrigins[userID]
	return ok
}

func (b *TournamentTGBot) DenyPermission(message *tgbotapi.Message) error {
	msg := tgbotapi.NewMessage(message.Chat.ID, "Permission denied, unlucky 😼")
	_, err := b.b.Send(msg)

	return err
}

func (b *TournamentTGBot) StartCommandHandler(message *tgbotapi.Message) error {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("/get_all"),
		),
	)

	messageText := "You were subscribed on tournament offers updates 🥸"
	msg := tgbotapi.NewMessage(message.Chat.ID, messageText)
	msg.ReplyMarkup = keyboard
	_, err := b.b.Send(msg)

	return err
}

func (b *TournamentTGBot) GetAllCommandHandler(message *tgbotapi.Message) error {
	offers, err := b.TournamentService.GetAllOffers(context.Background())
	if err != nil {
		return err
	}

	var buf strings.Builder
	buf.WriteString("All offers from old to new!\n\n")

	for i := range offers {
		buf.WriteString(offers[i].Notification() + "\n")
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, buf.String())
	_, err = b.b.Send(msg)

	return err
}

func (b *TournamentTGBot) Notify(offer *model.TournamentOffer) error {
	for _, chatID := range b.allowOrigins {
		msg := tgbotapi.NewMessage(chatID, "New tournament offer!\n"+offer.Notification())
		_, err := b.b.Send(msg)
		if err != nil {
			zap.L().Error("tournament bot: new offer: failed to notify", zap.Error(err))
		}
	}

	return nil
}
