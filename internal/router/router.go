package router

import (
	"bot_story_generator/internal/config"
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/models"
	"bot_story_generator/internal/text_messages"
	"context"
	"strings"
	"sync"
	//* Для будуших логов роутера
	// "go.uber.org/zap"
)

type StoryService interface {
	CreateStory(ctx context.Context, userID int64) ([]string, error)
	UserChoice(ctx context.Context, userID int64, data string) ([]string, error)
	CreateUser(ctx context.Context, userID int64) (string, error)
}

type StoryRouterImpl struct {
	ctx           context.Context
	cancel        context.CancelFunc
	service       StoryService
	chan_command  chan models.IncommingMessage
	chan_outbound chan models.OutboundMessage
	logger        *logger.Logger
	userState     map[int64]struct{}
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
		userState:     make(map[int64]struct{}),
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
			if _, ok := r.userState[msg.UserID]; ok {
				r.mux.Unlock()
				continue
			}
			r.userState[msg.UserID] = struct{}{}
			r.mux.Unlock()
			data := msg.Data
			userID := msg.UserID
			if data == "start" {
				resp, _ := r.service.CreateUser(r.ctx, userID)
				r.createOutboundMessage(r.ctx, userID, resp)
				r.cleanUserState(userID)
			} else if data == "newstory" {
				localctx, cancel := context.WithCancel(r.ctx)
				//*можно будет потом добавить еще типы сообщений для обработки
				ctxWithValue := context.WithValue(localctx, "delete", "1")
				r.createOutboundMessage(r.ctx, userID, text_messages.TextStartCreateHero)
				r.createOutboundMessage(ctxWithValue, userID, text_messages.WaitingTextHeroes)
				resp, err := r.service.CreateStory(r.ctx, userID)
				if err != nil {
					cancel()
					r.createOutboundMessage(r.ctx, userID, err.Error())
					r.cleanUserState(userID)
					continue
				}
				cancel()
				for i := range len(resp) {
					r.createOutboundMessage(r.ctx, userID, resp[i])
				}
				r.createOutboundMessage(r.ctx, userID, resp[len(resp)-1], models.NewButtonArg("userChoice_", []string{"1", "2", "3", "4", "5"}))
				r.cleanUserState(userID)

			} else if strings.HasPrefix(data, "userChoice_") {
				localctx, cancel := context.WithCancel(r.ctx)
				//*можно будет потом добавить еще типы сообщений для обработки
				ctxWithValue := context.WithValue(localctx, "delete", "1")
				r.createOutboundMessage(ctxWithValue, userID, text_messages.WaitingTextNarrative)
				arg := strings.TrimPrefix(data, "userChoice_")
				resp, err := r.service.UserChoice(r.ctx, userID, arg)
				if err != nil {
					cancel()
					r.createOutboundMessage(r.ctx, userID, err.Error())
					r.cleanUserState(userID)
					continue
				}
				cancel()
				r.createOutboundMessage(r.ctx, userID, resp[0])
				r.createOutboundMessage(r.ctx, userID, resp[1], models.NewButtonArg("userChoice_", []string{"1", "2", "3", "4", "5"}))
				r.cleanUserState(userID)

			} else if data == "help" {
				text := text_messages.TextHelp()
				r.createOutboundMessage(r.ctx, userID, text)
				r.cleanUserState(userID)
			} else {

			}
		}
	}
}

func (r *StoryRouterImpl) AddComand(ctx context.Context, data string, userID int64) {
	select {
	case <-r.ctx.Done():
		return
	case <-ctx.Done():
		return
	case r.chan_command <- models.NewIncommingMessage(data, userID):
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
