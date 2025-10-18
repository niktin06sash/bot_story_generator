package router

import (
	"bot_story_generator/internal/models"
	"bot_story_generator/internal/logger"

	// "go.uber.org/zap"
)

type StoryService interface {
}

type StoryRouterImpl struct {
	service  StoryService
	chan_msg chan models.Message
	chan_outbound chan models.OutboundMessage
	logger      *logger.Logger
}

func NewRouter(service StoryService, logger *logger.Logger) *StoryRouterImpl {
	return &StoryRouterImpl{
		service:  service,
		chan_msg: make(chan models.Message, 1000),
		logger:   logger,
	}
}

func (r *StoryRouterImpl) AddingComand(command string, arguments []string, chatID int64) {
	r.chan_msg <- models.Message{Command: command, Arguments: arguments, ChatID: chatID}
}

func (r *StoryRouterImpl) GetOutboundChan(c chan models.OutboundMessage) {
	r.chan_outbound = c
}

func (r *StoryRouterImpl) Start() {
	for msg := range r.chan_msg {
		// Обработка сообщения
		switch msg.Command {
		case "start":
			r.chan_outbound <- models.OutboundMessage{
				ChatID: msg.ChatID,
				Text:   "Welcome to the bot!",
			}
		default:
			// unknown command
		}
	}
}
