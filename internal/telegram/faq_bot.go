package telegram

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/uptrace/bun"
	"gitlab.com/duel-duck/duel-duck-api/config"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/internal/service"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	"go.uber.org/zap"
)

var faqActionsMap = map[string]bool{
	"approve": true,
	"update":  true,
}

type FAQTGBot struct {
	b            *tgbotapi.BotAPI
	FAQService   *service.FAQService
	FileService  *service.FileService
	allowOrigins map[int64]bool
}

func NewFAQTGBot(
	c *config.Config,
	faqService *service.FAQService,
	fileService *service.FileService,
) (*FAQTGBot, error) {
	bot, err := tgbotapi.NewBotAPI(c.Telegram.FAQBot.BotTgAPIKey)
	if err != nil {
		return nil, err
	}

	telegramIDs, err := faqService.FAQRepository.GetTelegramModeratorIDs(context.Background())
	if err != nil {
		return nil, apperrors.Internal("failed to get telegram moderator ids on FAQ bot init", err)
	}

	allowOrigins := make(map[int64]bool, len(telegramIDs))
	for _, id := range telegramIDs {
		allowOrigins[id] = true
	}

	faqBot := &FAQTGBot{
		b:            bot,
		FAQService:   faqService,
		FileService:  fileService,
		allowOrigins: allowOrigins,
	}

	go faqBot.sendFAQToModerators()

	return faqBot, nil
}

func (b *FAQTGBot) start(_ context.Context) error {
	if b == nil {
		return apperrors.Internal("start() called on nil FAQTGBot")
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.b.GetUpdatesChan(u)

	go b.WatchUpdates(updates)

	return nil
}

func (b *FAQTGBot) stop(_ context.Context) error {
	b.b.StopReceivingUpdates()

	return nil
}

func (b *FAQTGBot) WatchUpdates(updates tgbotapi.UpdatesChannel) {
	for update := range updates {
		go func(update tgbotapi.Update) {
			defer func() {
				err := recover()
				if err == nil {
					return
				}
			}()

			if update.CallbackQuery != nil {
				if !b.isAllowedOrigin(update.CallbackQuery.From.ID) {
					b.DenyPermission(update.CallbackQuery.From.ID)
					return
				}

				if update.CallbackQuery != nil {
					b.handleFAQCallback(update.CallbackQuery)
				}
			}

			if update.Message != nil {
				if !b.isAllowedOrigin(update.Message.From.ID) {
					b.DenyPermission(update.Message.From.ID)
					return
				}

				if update.Message != nil {
					b.handleFAQMessage(update.Message)
				}

				switch update.Message.Command() {
				case StartCommand:
					b.StartCommandHandler(update.Message)
				}
			}
		}(update)
	}
}

func (b *FAQTGBot) sendFAQToModerators() {
	for faq := range b.FAQService.ChanFAQBot {

		var media []interface{}

		if len(faq.Images) > 0 {

			for i, url := range faq.Images {
				photo := tgbotapi.NewInputMediaPhoto(tgbotapi.FileURL(url))
				if i == 0 {
					photo.Caption = fmt.Sprintf("ID: %d", faq.ID)
				}

				media = append(media, photo)
			}
		}

		for id := range b.allowOrigins {
			album := b.NewMediaMessage(media, true)
			album.ChatID = id
			err := b.SendMediaMessage(album)
			if err != nil {
				zap.L().Error("failed to send to the moderators media message", zap.Error(err))
			}

			text := strings.Builder{}

			text.WriteString(fmt.Sprintf(
				"❓ New FAQ request submitted:\n\n%s",
				faq.Question,
			))

			text.WriteString(fmt.Sprintf(
				"\n\nID: %d",
				faq.ID,
			))

			msg := b.NewMessage(text.String(), true, id)

			msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("✅ Approve", fmt.Sprintf("approve_%d", faq.ID)),
					tgbotapi.NewInlineKeyboardButtonData("❌ Reject", fmt.Sprintf("reject_%d", faq.ID)),
				),
			)

			err = b.SendMessage(msg)
			if err != nil {
				zap.L().Error("failed to send to the moderators message", zap.Error(err))
			}

		}

	}
}

func (b *FAQTGBot) NewMediaMessage(media []interface{}, notify bool) *tgbotapi.MediaGroupConfig {
	album := tgbotapi.NewMediaGroup(b.b.Self.ID, media)

	album.DisableNotification = notify

	return &album
}

func (b *FAQTGBot) SendMediaMessage(mediaMsg *tgbotapi.MediaGroupConfig) error {
	_, err := b.b.SendMediaGroup(*mediaMsg)

	return err
}

func (b *FAQTGBot) GetFile(fileID string) ([]byte, string, error) {
	file, err := b.b.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get file from telegram: %v", err)
	}

	fileURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", b.b.Token, file.FilePath)

	resp, err := http.Get(fileURL)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get file from url: %v", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read file: %v", err)
	}

	return data, fileURL, nil
}

func (b *FAQTGBot) SendMessage(msg *tgbotapi.MessageConfig) error {
	_, err := b.b.Send(msg)

	return err
}

func (b *FAQTGBot) NewMessage(msg string, notify bool, id int64) *tgbotapi.MessageConfig {
	m := tgbotapi.NewMessage(id, msg)
	m.DisableWebPagePreview = notify

	return &m
}

func (b *FAQTGBot) DenyPermission(chatID int64) error {
	msg := tgbotapi.NewMessage(chatID, "Oh hell nah. Permission denied 😼")
	_, err := b.b.Send(msg)

	return err
}

func (b *FAQTGBot) AnswerCallback(callbackID, text string) error {
	callback := tgbotapi.NewCallback(callbackID, text)
	_, err := b.b.Request(callback)
	return err
}

func (b *FAQTGBot) StartCommandHandler(message *tgbotapi.Message) error {
	messageText := "You were subscribed on FAQ moderations"
	msg := tgbotapi.NewMessage(message.Chat.ID, messageText)
	_, err := b.b.Send(msg)

	return err
}

func (b *FAQTGBot) handleFAQCallback(cb *tgbotapi.CallbackQuery) {
	data := cb.Data

	switch {
	case strings.HasPrefix(data, "approve_"):
		id := extractFAQIDFromButton(data, "approve_")

		msg := b.NewMessage(fmt.Sprintf("📝 Please submit your answer to the question #faq_approve_%d", id), false, cb.From.ID)
		msg.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true, Selective: true}
		_ = b.SendMessage(msg)

	case strings.HasPrefix(data, "reject_"):
		id := extractFAQIDFromButton(data, "reject_")

		faqQuestion, err := b.FAQService.
			FAQRepository.
			GetFAQQuestionByID(context.Background(), id)
		if err != nil {
			zap.L().Error("failed to get faq question by id", zap.Uint32("faq_id", id), zap.Error(err))
			break
		}

		if faqQuestion.ID == 0 {
			_ = b.SendMessage(b.NewMessage(fmt.Sprintf("‼️ Question #%d already rejected", id), false, cb.From.ID))
			break
		}

		faqAnswer, err := b.FAQService.
			FAQRepository.
			GetFAQAnswerByFAQQuestionID(context.Background(), faqQuestion.ID)
		if err != nil {
			zap.L().Error("failed to get faq answer by question id", zap.Uint32("faq_id", id), zap.Error(err))
			break
		}

		if err := b.FAQService.FAQRepository.DeleteFAQQuestion(context.Background(), id); err != nil {
			zap.L().Error("failed to delete faq question", zap.Uint32("faq_id", id), zap.Error(err))
			break
		} else {
			_ = b.SendMessage(b.NewMessage(fmt.Sprintf("🗑 Question #%d rejected", id), false, cb.From.ID))
		}

		err = b.FileService.RemoveFAQImages(faqQuestion.Images, faqAnswer.Images)
		if err != nil {
			zap.L().Error("failed to remove faq images", zap.Uint32("faq_id", id), zap.Error(err))
		}
	}

	_ = b.AnswerCallback(cb.ID, "")
}

func (b *FAQTGBot) HandleError(err error) {
	if err != nil {
		zap.L().Error(err.Error())
		msg := tgbotapi.NewMessage(b.b.Self.ID, "👮 Something went wrong 👮")
		msg.DisableNotification = true

		_, err = b.b.Send(msg)
		if err != nil {
			zap.L().Error("failed to send error message", zap.Error(err))
		}
	}
}

func (b *FAQTGBot) handleFAQMessage(msg *tgbotapi.Message) {
	if msg.ReplyToMessage == nil {
		return
	}

	id, action := extractFAQIDFromMessage(msg.ReplyToMessage.Text)
	if !faqActionsMap[action] {
		_ = b.SendMessage(b.NewMessage("🛑 You don't have a permission", false, msg.From.ID))
		return
	}

	moderatorID := uint64(msg.From.ID)
	m, err := b.FAQService.
		FAQRepository.
		GetModeratorByTelegramID(context.Background(), moderatorID)
	if err != nil || m.ID == 0 {
		_ = b.SendMessage(b.NewMessage("🛑 You don't have a permission", false, msg.From.ID))
		return
	}

	faq, err := b.FAQService.
		FAQRepository.
		GetFAQQuestionByID(context.Background(), id)
	if err != nil || faq.ID == 0 {
		_ = b.SendMessage(b.NewMessage(fmt.Sprintf("‼️ Question #%d not found", id), false, msg.From.ID))
		return
	}

	var image string
	if msg.Photo != nil {
		for i := len(msg.Photo) - 1; i >= 0; i-- {
			if (msg.Photo)[i].FileSize < 1_000_000 {
				fileID := (msg.Photo)[i].FileID
				file, fileURL, err := b.GetFile(fileID)
				if err != nil {
					b.HandleError(err)
					return
				}

				imageURL, err := b.FileService.
					SaveFAQAnswersImages(b.FAQService.MediaUrl, fileURL, file)
				if err != nil {
					b.HandleError(err)
					return
				}

				if imageURL != "" {
					image = imageURL
				}

			}
		}
	} else if msg.Document != nil && msg.Document.FileID != "" {
		file, fileURL, err := b.GetFile(msg.Document.FileID)
		if err != nil {
			b.HandleError(err)
			return
		}

		imageURL, err := b.FileService.
			SaveFAQAnswersImages(b.FAQService.MediaUrl, fileURL, file)
		if err != nil {
			b.HandleError(err)
			return
		}

		if imageURL != "" {
			image = imageURL
		}
	}

	text := msg.Text
	if text == "" {
		text = msg.Caption
	}

	err = b.createOrUpdateFAQAnswer(id, m, text, image)
	if err != nil {
		b.HandleError(err)
		return
	}

	if action == "approve" && !faq.Answered {
		_ = b.SendMessage(b.NewMessage(fmt.Sprintf("✅ Saved answer for question #%d", id), false, msg.From.ID))
	} else if action == "approve" && faq.Answered {
		_ = b.SendMessage(b.NewMessage(fmt.Sprintf("✅ Updated answer for question #%d", id), false, msg.From.ID))
	}
}

func (b *FAQTGBot) createOrUpdateFAQAnswer(faqID uint32, m *model.FAQModerator, answerText string, image string) error {
	ctx := context.Background()

	err := b.FAQService.TransactionManager.WithinTransaction(ctx, func(ctx context.Context, tx bun.Tx) error {

		answer, err := b.FAQService.FAQRepository.GetFAQAnswerByFAQQuestionID(context.Background(), faqID)
		if err != nil {
			return fmt.Errorf("failed to get faq answer by question id: %v", err)
		}

		if answer.ID == 0 {
			answer.QuestionID = faqID
			answer.ModeratorID = &m.ID
			answer.Answer = answerText

			if image != "" {
				answer.Images = append(answer.Images, image)
			}

			err = b.FAQService.FAQRepository.WithTx(tx).AddFAQAnswer(ctx, answer)
			if err != nil {
				return fmt.Errorf("failed to create faq answer: %v", err)
			}

		} else {
			answer.ModeratorID = &m.ID

			if answerText != "" {
				answer.Answer = answerText
			}

			answer.Images = nil

			if image != "" {
				answer.Images = append(make([]string, 0), image)
			}

			err = b.FAQService.FAQRepository.WithTx(tx).UpdateFAQAnswer(ctx, answer)
			if err != nil {
				return fmt.Errorf("failed to update faq answer: %v", err)
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func extractFAQIDFromButton(data string, prefix string) uint32 {
	idStr := strings.TrimPrefix(data, prefix)

	id, _ := strconv.ParseUint(idStr, 10, 32)

	return uint32(id)
}

func extractFAQIDFromMessage(msg string) (uint32, string) {
	parts := strings.Split(msg, "#faq_")

	if len(parts) > 1 {
		msgParts := strings.Split(parts[len(parts)-1], "_")

		action := msgParts[0]

		id, _ := strconv.ParseUint(msgParts[1], 10, 64)

		return uint32(id), action
	}

	return 0, ""

}

func (b *FAQTGBot) isAllowedOrigin(userID int64) bool {
	return b.allowOrigins[userID]
}
