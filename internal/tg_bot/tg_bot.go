package tgbot

import (
	"bot_story_generator/internal/config"
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/models"
	"context"
	"sync"

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
	AddComand(ctx context.Context, data string, chatID int64)
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
				bot.logger.ZapLogger.Info("Received update", zap.Any("data", data))
				bot.router.AddComand(bot.ctx, data, chatID)
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
				chatId := update.Message.Chat.ID
				text := update.Message.Text
				msg := update.Message
				if msg.IsCommand() {
					bot.logger.ZapLogger.Info("Received update", zap.Any("text", text))
					command := update.Message.Command()
					bot.router.AddComand(bot.ctx, command, chatId)
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
			msg, err := bot.sendMessage(outMsg.ChatID, outMsg.Text, outMsg.ButtonArgs)
			if err != nil {
				bot.logger.ZapLogger.Error(
					"failed to send outbound message",
					zap.Error(err),
					zap.Int64("chat_id", outMsg.ChatID),
					zap.String("text", outMsg.Text),
				)
				continue
			}
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
					bot.waitingMessage(localctx, msg, outMsg.ChatID)
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

func (bot *Bot) Stop() {
	bot.cancel()
	bot.wg.Wait()
	bot.router.CloseCommandChan()
	bot.logger.ZapLogger.Info("Bot stopped")
}

func (bot *Bot) waitingMessage(ctx context.Context, sentMsg tgbotapi.Message, chatID int64) {
	select {
	case <-ctx.Done():
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
	case <-bot.ctx.Done():
		return
	}
}
