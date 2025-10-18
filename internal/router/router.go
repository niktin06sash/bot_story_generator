package router

import (
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/models"
	"context"
	// "go.uber.org/zap"
)

type StoryService interface {
}

type StoryRouterImpl struct {
	ctx           context.Context
	cancel        context.CancelFunc
	service       StoryService
	chan_command  chan models.Message
	chan_outbound chan models.OutboundMessage
	logger        *logger.Logger
}

func NewRouter(service StoryService, logger *logger.Logger) *StoryRouterImpl {
	context, cancel := context.WithCancel(context.Background())
	return &StoryRouterImpl{
		ctx:           context,
		cancel:        cancel,
		service:       service,
		chan_command:  make(chan models.Message, 1000),
		chan_outbound: make(chan models.OutboundMessage, 1000),
		logger:        logger,
	}
}

func (r *StoryRouterImpl) Start() {
	for {
		select {
		case <-r.ctx.Done():
			return
		case msg, ok := <-r.chan_command:
			if !ok {
				return
			}
			switch msg.Command {
			case "start":
				select {
				case <-r.ctx.Done():
					return
				case r.chan_outbound <- models.OutboundMessage{
					ChatID: msg.ChatID,
					Text:   "Welcome to the bot!",
				}:
				}
			default:
			}
		}
	}
}
func (r *StoryRouterImpl) AddComand(ctx context.Context, command string, arguments []string, chatID int64) {
	select {
	case <-r.ctx.Done():
		return
	case <-ctx.Done():
		return
	case r.chan_command <- models.Message{Command: command, Arguments: arguments, ChatID: chatID}:
	}
}

func (r *StoryRouterImpl) GetOutboundChan() chan models.OutboundMessage {
	return r.chan_outbound
}
func (r *StoryRouterImpl) CloseCommandChan() {
	close(r.chan_command)
}
func (r *StoryRouterImpl) Stop() {
	r.cancel()
	close(r.chan_outbound)
}
