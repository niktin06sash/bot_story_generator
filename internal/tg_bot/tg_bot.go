package tgbot

import (
	"bot_story_generator/internal/config"
	"bot_story_generator/internal/logger"
	"context"

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
}
type StoryRouter interface {
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

	logger.ZapLogger.Info("authorized on account " + bot.Self.UserName)
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
	}, nil
}

func (bot *Bot) Start() {
	for {
		select {
		case <-bot.ctx.Done():
			return
		case update, ok := <-bot.updatesChan:
			if !ok {
				return
			}
			if update.Message == nil || !update.Message.IsCommand() {
				continue // Игнорируем не-команды
			}

			// Создаём ответное сообщение
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
			command := update.Message.Command() // Получаем команду без "/"

			switch command {
			case "start":
				//
			case "help":
				//
			default:
				//
			}

			_, err := bot.api.Send(msg)
			if err != nil {
				bot.logger.ZapLogger.Error(
					"failed to send message",
					zap.Error(err),
					zap.Int64("chat_id", update.Message.Chat.ID),
					zap.String("user", update.Message.From.UserName),
					zap.String("command", command),
					zap.String("message", msg.Text),
				)
			}
		}
	}
}
func (bot *Bot) Stop() {
	bot.cancel()
}
