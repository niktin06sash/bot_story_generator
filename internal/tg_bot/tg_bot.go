package tgbot

import (
	"bot_story_generator/internal/config"
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/models"
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

	OutboundChan chan models.OutboundMessage // буферизированный канал для исходящих сообщений
}
type StoryRouter interface {
	AddingComand(command string, arguments []string, chatID int64)
	GetOutboundChan(chan models.OutboundMessage)
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

		OutboundChan: make(chan models.OutboundMessage, 100),
	}, nil
}

func (bot *Bot) Start_Read_Messange() {
	for {
		select {
		case <-bot.ctx.Done():
			return
		case update, ok := <-bot.updatesChan:
			if !ok {
				return
			}
			bot.logger.ZapLogger.Info("Received update", zap.Any("text", update.Message.Text))
			if update.Message == nil || !update.Message.IsCommand() {
				continue // Игнорируем не-команды
			}

			command := update.Message.Command() // Получаем команду без "/"
			bot.router.AddingComand(command, nil ,update.Message.Chat.ID)
		}
	}
}

func (bot *Bot) Start_Send_Messange_For_Chan() {
	bot.OutboundChan = make(chan models.OutboundMessage, 100)
	bot.router.GetOutboundChan(bot.OutboundChan)
	for {
		select {
		case <-bot.ctx.Done():
			return
		case outMsg, ok := <-bot.OutboundChan:
			if !ok {
				return
			}
			bot.SendMessage(outMsg.ChatID, outMsg.Text)
		}
	}
}

func (bot *Bot) SendMessage(chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	_, err := bot.api.Send(msg)
	if err != nil {
		bot.logger.ZapLogger.Error(
			"failed to send message",
			zap.Error(err),
			zap.Int64("chat_id", chatID),
			zap.String("message", msg.Text),
		)
		return err
	}
	bot.logger.ZapLogger.Info(
		"message sent successfully",
		zap.Int64("chat_id", chatID),
		zap.String("message", msg.Text),
	)
	return nil
}

func (bot *Bot) Stop() {
	bot.cancel()
}
