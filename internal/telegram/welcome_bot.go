package telegram

import (
	"context"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	"gitlab.com/duel-duck/duel-duck-api/config"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/internal/service"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	"go.uber.org/zap"
	"strconv"
	"time"
)

type WelcomeTGBot struct {
	WelcomeBot       *tgbotapi.BotAPI
	UserService      *service.UserService
	AuthService      *service.AuthService
	log              *zap.Logger
	environment      string
	loginURLTemplate string
}

const (
	WelcomeStartText     = "Logging into your Duel Duck account has never been easier.\n\nUse this bot for quick and secure access.\nPress the \"Generate Link\" button to log in via Telegram."
	UnknownCommandText   = "Oops! Quack 404!\nThe command you tried is invalid h🦆"
	LoginButtonCoverText = "Quack, Quack! ✨ New Login Alert: Use link below to log in 🚀"

	LoginText    = "Generate new login link 🔮"
	StartCommand = "start"
	LoginCommand = "login"
)

func NewWelcomeTGBot(
	c *config.Config,
	log *zap.Logger,
	userService *service.UserService,
	authService *service.AuthService,
) (*WelcomeTGBot, error) {
	bot, err := tgbotapi.NewBotAPI(c.Telegram.WelcomeBotTgAPIKey)
	if err != nil {
		return nil, err
	}

	welcomeBot := &WelcomeTGBot{
		WelcomeBot:       bot,
		log:              log,
		environment:      c.App.Environment,
		loginURLTemplate: c.HTTP.PublicDomain + "?telegram_id=%s&code=%s&username=%s",
		UserService:      userService,
		AuthService:      authService,
	}

	return welcomeBot, nil
}

func (b *WelcomeTGBot) start(_ context.Context) error {
	if b == nil {
		return apperrors.Internal("start() called on nil WelcomeTGBot")
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.WelcomeBot.GetUpdatesChan(u)

	go b.WatchUpdates(updates)

	return nil
}

func (b *WelcomeTGBot) WatchUpdates(updates tgbotapi.UpdatesChannel) {
	for update := range updates {
		func() {
			defer func() {
				err := recover()
				if err == nil {
					return
				}

				b.log.Error("welcome bot: recovered from panic",
					zap.Any("err", err),
					zap.Stack("stack"))
			}()

			if update.Message == nil {
				return
			}

			if update.Message.Text == LoginText {
				if err := b.LoginCommandHandler(update.Message); err != nil {
					b.log.Error("welcome bot: handle login failed", zap.Error(err))
				}
				return
			}

			if !update.Message.IsCommand() {
				if err := b.UnknownCommand(update.Message); err != nil {
					b.log.Error("welcome bot: failed to send message", zap.Error(err))
				}
			}

			command := update.Message.Command()

			switch command {
			case StartCommand, LoginCommand:
				if err := b.StartCommandHandler(update.Message); err != nil {
					b.log.Error("welcome bot: handle start failed", zap.Error(err))
				}
				return
			}
		}()
	}
}

func (b *WelcomeTGBot) stop(_ context.Context) error {
	b.WelcomeBot.StopReceivingUpdates()
	return nil
}

func (b *WelcomeTGBot) UnknownCommand(message *tgbotapi.Message) error {
	msg := tgbotapi.NewMessage(message.Chat.ID, UnknownCommandText)
	_, err := b.WelcomeBot.Send(msg)

	return err
}

func (b *WelcomeTGBot) LoginCommandHandler(message *tgbotapi.Message) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	telegramID := strconv.FormatInt(message.From.ID, 10)
	authTG := model.SignInWithTelegramMiniApp{
		TelegramID: telegramID,
		FirstName:  message.From.FirstName,
		LastName:   message.From.LastName,
		Username:   message.From.UserName,
	}
	user, err := b.UserService.SignInWithTelegram(ctx, authTG)
	if err != nil {
		return err
	}

	if user == nil {
		return apperrors.Internal("user is nil")
	}

	code, err := uuid.NewUUID()
	if err != nil {
		return err
	}

	if err = b.AuthService.SaveTelegramCode(ctx, telegramID, code); err != nil {
		return err
	}

	loginButton := tgbotapi.NewInlineKeyboardButtonURL(
		"Click here to login",
		fmt.Sprintf(b.loginURLTemplate, user.TelegramID, code.String(), user.Username))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(loginButton),
	)

	msg := tgbotapi.NewMessage(message.Chat.ID, LoginButtonCoverText)
	msg.ReplyMarkup = keyboard
	_, err = b.WelcomeBot.Send(msg)

	return err
}

func (b *WelcomeTGBot) StartCommandHandler(message *tgbotapi.Message) error {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(LoginText),
		),
	)

	msg := tgbotapi.NewMessage(message.Chat.ID, WelcomeStartText)
	msg.ReplyMarkup = keyboard

	_, err := b.WelcomeBot.Send(msg)
	return err
}
