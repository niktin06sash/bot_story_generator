package tgbot

import (
	"bot_story_generator/internal/config"

	"go.uber.org/zap"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	api    *tgbotapi.BotAPI
	logger *zap.Logger
}

func NewBot(cfg *config.Config, logger *zap.Logger) (*Bot, error) {
	bot, err := tgbotapi.NewBotAPI(cfg.TelegramBotToken)
	if err != nil {
		logger.Panic("failed to create Telegram bot", zap.Error(err))
		return nil, err
	}

	bot.Debug = cfg.TelegramBotDebug
	if bot.Debug {
		logger.Info("telegram bot debug mode is enabled")
	}


	logger.Info("authorized on account " + bot.Self.UserName)
	return &Bot{
		api:    bot,
		logger: logger,
	}, nil
}

func (bot *Bot) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.api.GetUpdatesChan(u)

	for update := range updates {
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

		// Отправляем ответ
		_, err := bot.api.Send(msg)
		if err != nil {
			bot.logger.Error(
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