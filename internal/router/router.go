package router

import (
	"bot_story_generator/internal/config"
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/models"
	"bot_story_generator/internal/text_messages"
	"sync"
	"strconv"
	"context"

	"go.uber.org/zap"
)

type StoryService interface {
	CreateStructuredHeroes(ctx context.Context, chatID int64) (string, error)
}

type StoryRouterImpl struct {
	ctx           context.Context
	cancel        context.CancelFunc
	service       StoryService
	chan_command  chan models.IncommingMessage
	chan_outbound chan models.OutboundMessage
	logger        *logger.Logger
	userState     map[int64]bool
	mux           *sync.Mutex
	wg            *sync.WaitGroup
	numworkers    int
}

func NewRouter(cfg *config.Config, service StoryService, logger *logger.Logger) *StoryRouterImpl {
	context, cancel := context.WithCancel(context.Background())
	routerImpl := &StoryRouterImpl{
		ctx:           context,
		cancel:        cancel,
		service:       service,
		chan_command:  make(chan models.IncommingMessage, 1000),
		chan_outbound: make(chan models.OutboundMessage, 1000),
		userState:     make(map[int64]bool),
		mux:           &sync.Mutex{},
		logger:        logger,
		wg:            &sync.WaitGroup{},
		numworkers:    cfg.NumWorkers,
	}

	return routerImpl
}
func (r *StoryRouterImpl) StartRouter() {
	r.wg.Add(r.numworkers)
	for i := 0; i < r.numworkers; i++ {
		go func() {
			defer r.wg.Done()
			r.routerWorker()
		}()
	}
}
func (r *StoryRouterImpl) routerWorker() {
	for {
		select {
		case <-r.ctx.Done():
			return
		case msg, ok := <-r.chan_command:
			if !ok {
				return
			}
			r.mux.Lock()
			if r.userState[msg.ChatID] {
				r.mux.Unlock()
				continue
			}
			r.userState[msg.ChatID] = true
			r.mux.Unlock()
			switch msg.Command {
			case "start":
				r.createOutboundMessage(msg.ChatID, text_messages.TextGreeting)
			case "newstory":
				//Создаем персонажа
				r.createOutboundMessage(msg.ChatID, text_messages.TextStartCreateHero)
				resp, err := r.service.CreateStructuredHeroes(r.ctx, msg.ChatID)
				if err != nil {
					r.logger.ZapLogger.Error("failed to create structured heroes", zap.Error(err), zap.Int64("chat_id", msg.ChatID))
					r.createOutboundMessage(msg.ChatID, text_messages.TextErrorCreateHero)
					continue
				}
				r.createOutboundMessage(msg.ChatID, resp)

				//TODO начинаем повествование

			case "userChoice":
				choiceStr := msg.Arguments["option"]
				choiceInt, err := strconv.Atoi(choiceStr)
				if err != nil {
					r.logger.ZapLogger.Error("invalid user choice", zap.String("choice", choiceStr), zap.Error(err), zap.Int64("chat_id", msg.ChatID))
					r.createOutboundMessage(msg.ChatID, "Некорректный выбор опции")
					continue
				}
				_ = choiceInt

				//TODO записывем выбор в бд
				
				//TODO генерим ответ ии

				//TODO записываем в бд повестование

				//TODO записываем в бд варианты выборов

				//TODO отправляем сообщение юзеру с вариантами ответа

			case "help":
				text := text_messages.TextHelp()
				r.createOutboundMessage(msg.ChatID, text)
			default:
			}
		}
	}
}
func (r *StoryRouterImpl) createOutboundMessage(chatID int64, text string) {
	select {
	case <-r.ctx.Done():
		return
	case r.chan_outbound <- models.NewOutboundMessage(chatID, text):
		r.mux.Lock()
		delete(r.userState, chatID)
		r.mux.Unlock()
	}
}
func (r *StoryRouterImpl) AddComand(ctx context.Context, command string, arguments map[string]string, chatID int64) {
	select {
	case <-r.ctx.Done():
		return
	case <-ctx.Done():
		return
	case r.chan_command <- models.NewIncommingMessage(command, arguments, chatID):
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
	r.wg.Wait()
	close(r.chan_outbound)
	r.logger.ZapLogger.Info("Router stopped")
}
