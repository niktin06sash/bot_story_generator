package tgbot

import (
	"bot_story_generator/internal/config"
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/models"
	"context"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

type Bot struct {
	ctx         context.Context
	cancel      context.CancelFunc
	updatesChan tgbotapi.UpdatesChannel
	api         *tgbotapi.BotAPI
	logger      *logger.Logger
	router      StoryRouter
	wg          *sync.WaitGroup
	numworkers  int
}
type StoryRouter interface {
	AddComand(ctx context.Context, data string, chatID int64, userID int64)
	GetOutboundChan() chan models.OutboundMessage
	CloseCommandChan()
}

func NewBot(cfg *config.Config, logger *logger.Logger, router StoryRouter) (*Bot, error) {
	bot, err := tgbotapi.NewBotAPI(cfg.Telegram.BotToken)
	if err != nil {
		logger.ZapLogger.Error("Failed to create Telegram bot", zap.Error(err))
		return nil, err
	}

	bot.Debug = cfg.Telegram.BotDebug
	if bot.Debug {
		logger.ZapLogger.Info("telegram bot debug mode is enabled")
	}

	logger.ZapLogger.Info("Authorized on account " + bot.Self.UserName)
	u := tgbotapi.NewUpdate(cfg.Telegram.Offset)
	u.Timeout = cfg.Telegram.Timeout

	updates := bot.GetUpdatesChan(u)
	ctx, cancel := context.WithCancel(context.Background())
	return &Bot{
		ctx:         ctx,
		cancel:      cancel,
		api:         bot,
		logger:      logger,
		updatesChan: updates,
		router:      router,
		wg:          &sync.WaitGroup{},
		numworkers:  cfg.NumWorkers,
	}, nil
}
func (bot *Bot) StartBot() {
	//maybe increase the number of worker-bots(field = numworkers)
	bot.wg.Add(2)
	go func() {
		defer bot.wg.Done()
		bot.readIncommingMessage()
	}()
	go func() {
		defer bot.wg.Done()
		bot.sendOutboundMessage()
	}()
}
func (bot *Bot) readIncommingMessage() {
	for {
		select {
		case <-bot.ctx.Done():
			return
		case update, ok := <-bot.updatesChan:
			if !ok {
				return
			}
			if update.CallbackQuery != nil {
				data := update.CallbackQuery.Data
				chatID := update.CallbackQuery.Message.Chat.ID
				userID := update.CallbackQuery.From.ID
				bot.logger.ZapLogger.Info("Received update", zap.Any("data", data), zap.Any("userID", userID), zap.Any("chatID", chatID))
				bot.router.AddComand(bot.ctx, data, chatID, userID)
				//после нажатия на кнопку выбора она исчезает с экрана(надо тестить и проверять как это отображается)
				edit := tgbotapi.NewEditMessageReplyMarkup(
					chatID,
					update.CallbackQuery.Message.MessageID,
					tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}},
				)
				bot.api.Request(edit)
				callback := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
				bot.api.Request(callback)
			} else if update.Message != nil {
				chatID := update.Message.Chat.ID
				text := update.Message.Text
				msg := update.Message
				userID := update.Message.From.ID
				if msg.IsCommand() {
					bot.logger.ZapLogger.Info("Received update", zap.Any("data", text), zap.Any("userID", userID), zap.Any("chatID", chatID))
					command := update.Message.Command()
					bot.router.AddComand(bot.ctx, command, chatID, userID)
				}
			}
		}
	}
}

func (bot *Bot) sendOutboundMessage() {
	outch := bot.router.GetOutboundChan()
	for {
		select {
		case <-bot.ctx.Done():
			return
		case outMsg, ok := <-outch:
			if !ok {
				return
			}
			var text []string
			if strings.Contains(outMsg.Text, "---") {
				parts := strings.Split(outMsg.Text, "---")
				for i := range parts {
					parts[i] = strings.TrimSpace(parts[i])
				}
				text = parts
			} else {
				text = []string{strings.TrimSpace(outMsg.Text)}
			}
			msg, err := bot.sendMessage(outMsg.ChatID, text[0], outMsg.ButtonArgs)
			if err != nil {
				bot.logger.ZapLogger.Error(
					"failed to send outbound message",
					zap.Error(err),
					zap.Int64("chat_id", outMsg.ChatID),
					zap.String("text", outMsg.Text),
				)
				continue
			}
			//*опционально в будущем придумать логику для выбора разных значений контекста
			localctx := outMsg.Ctx
			value := localctx.Value("delete")
			if value == nil {
				continue
			}
			isDelete, ok := value.(string)
			if !ok {
				bot.logger.ZapLogger.Warn("Context value for 'delete' is not a string", zap.Any("value", value))
				continue
			}
			if isDelete == "1" {
				bot.wg.Add(1)
				go func() {
					defer bot.wg.Done()
					bot.waitingMessageWithAnimation(localctx, msg, outMsg.ChatID, text)
				}()
			}
		}
	}
}

func (bot *Bot) sendMessage(chatID int64, text string, butarg []models.ButtonArg) (tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(chatID, text)
	if len(butarg) > 0 {
		var rows [][]tgbotapi.InlineKeyboardButton
		for _, btn := range butarg {
			for _, arg := range btn.Args {
				button := tgbotapi.NewInlineKeyboardButtonData(arg, btn.ButtonName+arg)
				row := tgbotapi.NewInlineKeyboardRow(button)
				rows = append(rows, row)
			}
		}
		keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
		msg.ReplyMarkup = keyboard
	}
	sentmsg, err := bot.api.Send(msg)
	if err != nil {
		bot.logger.ZapLogger.Error(
			"failed to send message",
			zap.Error(err),
			zap.Int64("chat_id", chatID),
			zap.String("message", msg.Text),
		)
		return sentmsg, err
	}
	bot.logger.ZapLogger.Info(
		"message sent successfully",
		zap.Int64("chat_id", chatID),
		zap.String("message", msg.Text),
	)

	return sentmsg, nil
}

// НОВАЯ ВЕРСИЯ
func (bot *Bot) waitingMessageWithAnimation(ctx context.Context, sentMsg tgbotapi.Message, chatID int64, inputText []string) {
	currentIdx := 1
	if len(inputText) == 1 {
		currentIdx = 0
	}
	bot.logger.ZapLogger.Info(
		"Starting waitingMessageWithAnimation",
		zap.Int64("chat_id", chatID),
		zap.Int("message_id", sentMsg.MessageID),
	)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			bot.logger.ZapLogger.Info(
				"context cancelled, deleting loading message",
				zap.Int64("chat_id", chatID),
				zap.Int("message_id", sentMsg.MessageID),
			)
			del := tgbotapi.NewDeleteMessage(chatID, sentMsg.MessageID)
			_, err := bot.api.Request(del)
			if err != nil {
				bot.logger.ZapLogger.Error(
					"failed to delete loading message",
					zap.Error(err),
					zap.Int64("chat_id", chatID),
					zap.Int("message_id", sentMsg.MessageID),
				)
			} else {
				bot.logger.ZapLogger.Info(
					"loading message deleted successfully",
					zap.Int64("chat_id", chatID),
					zap.Int("message_id", sentMsg.MessageID),
				)
			}
			return
		case <-bot.ctx.Done():
			return
		case <-ticker.C:
			editMsg := tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID, inputText[currentIdx])
			_, err := bot.api.Send(editMsg)
			if err != nil {
				bot.logger.ZapLogger.Error(
					"failed to update waiting message",
					zap.Error(err),
					zap.Int64("chat_id", chatID),
					zap.Int("message_id", sentMsg.MessageID),
				)
			}
			currentIdx = (currentIdx + 1) % len(inputText)
		}
	}
}

func (bot *Bot) Stop() {
	bot.cancel()
	bot.wg.Wait()
	bot.router.CloseCommandChan()
	bot.logger.ZapLogger.Info("Bot stopped")
}
