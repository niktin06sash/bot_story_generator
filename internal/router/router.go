package router

import (
	"bot_story_generator/internal/config"
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/models"
	"bot_story_generator/internal/text_messages"
	"context"
	"strings"
	"sync"

	"go.uber.org/zap"
)

type StoryService interface {
	CreateStory(ctx context.Context, userID int64) ([]string, error)
	StopStory(ctx context.Context, userID int64) (string, error)
	StopStoryChoice(ctx context.Context, userID int64, arg string) (string, error)

	UserChoice(ctx context.Context, userID int64, arg string) ([]string, error)

	CreateUser(ctx context.Context, userID int64) (string, error)

	ValidatePreCheckout(ctx context.Context, pd *models.PaymentData) error
	BuySubscription(ctx context.Context, userID int64) (*models.Subscription, error)
	CommitSubscription(ctx context.Context, pd *models.PaymentData) error
	GetSubscriptionStatus(ctx context.Context, userID int64) (string, error)

	SetSetting(ctx context.Context, key string, value string, updatedBy int64) (string, error)
	ViewSetting(ctx context.Context) (string, error)
	RebootCacheData(ctx context.Context) error

	AdminCommandHandler(ctx context.Context, command string) (string, error)
}

type StoryRouterImpl struct {
	ctx                    context.Context
	cancel                 context.CancelFunc
	service                StoryService
	chan_command           chan models.IncommingMessage
	chan_outbound_payments chan *models.PaymentData
	chan_outbound          chan models.OutboundMessage
	chan_edit              chan models.EditMessage
	chan_delete            chan models.DeleteMessage
	chan_bot_invoice       chan models.InvoiceMessage
	chan_payments          chan *models.PaymentData
	logger                 *logger.Logger
	userState              map[int64]struct{}
	admins                 map[int64]struct{}
	mux                    *sync.RWMutex
	wg                     *sync.WaitGroup
	numworkers             int
}

func NewRouter(cfg *config.Config, service StoryService, logger *logger.Logger) *StoryRouterImpl {
	context, cancel := context.WithCancel(context.Background())
	routerImpl := &StoryRouterImpl{
		ctx:                    context,
		cancel:                 cancel,
		service:                service,
		chan_command:           make(chan models.IncommingMessage, 1000),
		chan_outbound_payments: make(chan *models.PaymentData, 1000),
		chan_payments:          make(chan *models.PaymentData, 1000),
		chan_outbound:          make(chan models.OutboundMessage, 1000),
		chan_edit:              make(chan models.EditMessage, 1000),
		chan_delete:            make(chan models.DeleteMessage, 1000),
		chan_bot_invoice:       make(chan models.InvoiceMessage, 1000),
		userState:              make(map[int64]struct{}),
		admins:                 cfg.Setting.Admins,
		mux:                    &sync.RWMutex{},
		logger:                 logger,
		wg:                     &sync.WaitGroup{},
		numworkers:             cfg.Setting.NumWorkers,
	}

	return routerImpl
}
func (r *StoryRouterImpl) StartRouter() {
	totalWorkers := r.numworkers * 2
	r.wg.Add(totalWorkers)
	for i := 0; i < r.numworkers; i++ {
		go func() {
			defer r.wg.Done()
			r.routerWorker()
		}()
		go func() {
			defer r.wg.Done()
			r.paymentWorker()
		}()
	}
}
func (r *StoryRouterImpl) paymentWorker() {
	for {
		select {
		case <-r.ctx.Done():
			return
		case data, ok := <-r.chan_payments:
			{
				if !ok {
					return
				}
				r.mux.Lock()
				if _, ok := r.userState[data.UserID]; ok {
					r.mux.Unlock()
					continue
				}
				r.userState[data.UserID] = struct{}{}
				r.mux.Unlock()
				if data.ChargeID == "" && data.QueryID != "" {
					r.logger.ZapLogger.Info("Validating PreCheckoutQuery...", zap.Any("userID", data.UserID), zap.Any("payload", data.InvoicePayload))
					err := r.service.ValidatePreCheckout(r.ctx, data)
					if err != nil {
						data.Error = err
						r.createPaymentMessage(data)
						r.cleanUserState(data.UserID)
						continue
					}
					r.createPaymentMessage(data)
					r.cleanUserState(data.UserID)

				} else if data.ChargeID != "" && data.QueryID == "" {
					r.logger.ZapLogger.Info("Commiting Subscription...", zap.Any("userID", data.UserID), zap.Any("payload", data.InvoicePayload))
					err := r.service.CommitSubscription(r.ctx, data)
					if err != nil {
						data.Error = err
						r.createPaymentMessage(data)
						r.cleanUserState(data.UserID)
						continue
					}
					r.createPaymentMessage(data)
					r.cleanUserState(data.UserID)
				}
			}
		}
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
			msgID := msg.MsgID

			if data == "start" {
				//2 лог
				r.logger.ZapLogger.Info("Creating user...", zap.Any("userID", userID))
				resp, err := r.service.CreateUser(r.ctx, userID)
				if err != nil {
					r.createOutboundMessage(r.ctx, userID, err.Error())
				} else {
					r.createOutboundMessage(r.ctx, userID, resp)
				}
				r.cleanUserState(userID)

			} else if data == "newstory" {
				//2 лог
				r.logger.ZapLogger.Info("Creating new story...", zap.Any("userID", userID))
				localctx, cancel := context.WithCancel(r.ctx)
				//*можно будет потом добавить еще типы сообщений для обработки
				ctxWithValue := context.WithValue(localctx, "delete", "1")
				r.createOutboundMessage(ctxWithValue, userID, text_messages.WaitingTextHeroes)
				resp, err := r.service.CreateStory(r.ctx, userID)
				cancel()
				if err != nil {
					r.createOutboundMessage(r.ctx, userID, err.Error())
					r.cleanUserState(userID)
					continue
				}
				r.createOutboundMessage(r.ctx, userID, text_messages.TextStartCreateHero)
				// Выводим всех персонажей (первые len(resp)-1 элементов)
				for i := 0; i < len(resp)-1; i++ {
					r.createOutboundMessage(r.ctx, userID, resp[i])
				}
				// Последний элемент - текст с кнопками выбора
				r.createOutboundMessage(r.ctx, userID, resp[len(resp)-1], models.NewButtonArg("userChoice_", []string{"1", "2", "3", "4", "5"}))
				r.cleanUserState(userID)

			} else if strings.HasPrefix(data, "userChoice_") {
				//2 лог
				r.logger.ZapLogger.Info("User making a choice...", zap.Any("userID", userID))
				localctx, cancel := context.WithCancel(r.ctx)
				//*можно будет потом добавить еще типы сообщений для обработки
				ctxWithValue := context.WithValue(localctx, "delete", "1")
				r.createOutboundMessage(ctxWithValue, userID, text_messages.WaitingTextNarrative)
				arg := strings.TrimPrefix(data, "userChoice_")
				resp, err := r.service.UserChoice(r.ctx, userID, arg)
				cancel()
				if err != nil {
					r.createOutboundMessage(r.ctx, userID, err.Error())
					r.cleanUserState(userID)
					continue
				}
				r.createEditMessage(userID, msgID, "")
				r.createOutboundMessage(r.ctx, userID, resp[0])
				r.createOutboundMessage(r.ctx, userID, resp[1], models.NewButtonArg("userChoice_", []string{"1", "2", "3", "4", "5"}))
				r.cleanUserState(userID)

			} else if data == "help" {
				//2 лог
				r.logger.ZapLogger.Info("User getting help...", zap.Any("userID", userID))
				r.createOutboundMessage(r.ctx, userID, text_messages.TextHelp())
				r.cleanUserState(userID)

			} else if data == "stopstory" {
				//2 лог
				r.logger.ZapLogger.Info("User stopping story...", zap.Any("userID", userID))
				resp, err := r.service.StopStory(r.ctx, userID)
				if err != nil {
					r.createOutboundMessage(r.ctx, userID, err.Error())
				} else {
					r.createOutboundMessage(r.ctx, userID, resp, models.NewButtonArg("stopStoryChoice_", []string{"✅", "❌"}))
				}
				r.cleanUserState(userID)

			} else if strings.HasPrefix(data, "stopStoryChoice_") {
				//2 лог
				r.logger.ZapLogger.Info("User making a stop story choice...", zap.Any("userID", userID))
				arg := strings.TrimPrefix(data, "stopStoryChoice_")
				resp, err := r.service.StopStoryChoice(r.ctx, userID, arg)
				r.createDeleteMessage(userID, msgID)
				if resp == "" && err == nil {
					r.cleanUserState(userID)
					continue
				}
				if err != nil {
					r.createOutboundMessage(r.ctx, userID, err.Error())
					r.cleanUserState(userID)
					continue
				}
				r.createOutboundMessage(r.ctx, userID, resp)
				r.cleanUserState(userID)

			} else if data == "buySubscription" {
				//2 лог
				r.logger.ZapLogger.Info("Processing buying subscription...", zap.Any("userID", userID))
				sub, err := r.service.BuySubscription(r.ctx, userID)
				if err != nil {
					r.createOutboundMessage(r.ctx, userID, err.Error())
					r.cleanUserState(userID)
					continue
				}
				r.createInvoiceMessage(sub)
				r.cleanUserState(userID)

			} else if data == "subscription" {
				//2 лог
				r.logger.ZapLogger.Info("Checking subscription status...", zap.Any("userID", userID))
				resp, err := r.service.GetSubscriptionStatus(r.ctx, userID)
				if err != nil {
					r.createOutboundMessage(r.ctx, userID, err.Error())
					r.cleanUserState(userID)
					continue
				}
				r.createOutboundMessage(r.ctx, userID, resp)
				r.cleanUserState(userID)

			} else if data == "terms" {
				// Обработка запроса на просмотр пользовательского соглашения (terms)
				r.logger.ZapLogger.Info("User requesting terms of service...", zap.Any("userID", userID))
				r.createOutboundMessage(r.ctx, userID, text_messages.TextTermsOfService)
				r.cleanUserState(userID)

			} else if data == "support" {
				// Обработка запроса на поддержку
				r.logger.ZapLogger.Info("User requesting support...", zap.Any("userID", userID))
				r.createOutboundMessage(r.ctx, userID, text_messages.TextSupportInfo)
				r.cleanUserState(userID)

			} else if data == "changeSetting" {
				// Обработка изменения настроек администратором
				if !r.checkAdmin(userID) {
					r.logger.ZapLogger.Warn("Unauthorized setting change attempt", zap.Any("userID", userID))
					r.createOutboundMessage(r.ctx, userID, text_messages.TextUnknownCommand)
					r.cleanUserState(userID)
					continue
				}

				if len(msg.Arguments) == 0 {
					r.logger.ZapLogger.Error("Missing setting arguments", zap.Any("userID", userID))
					r.createOutboundMessage(r.ctx, userID, "Не указаны параметры настройки")
					r.cleanUserState(userID)
					continue
				}
				//в логах лучше не хранить конкретные данные настройки. только если имя настройки
				r.logger.ZapLogger.Info("Admin changing settings...", zap.Any("userID", userID), zap.Any("setting", msg.Arguments[0].NameSetting))

				resp, err := r.service.SetSetting(r.ctx, msg.Arguments[0].NameSetting, msg.Arguments[0].ValueSetting, userID)
				if err != nil {
					r.logger.ZapLogger.Error("Failed to change setting", zap.Error(err), zap.Any("userID", userID), zap.Any("setting", msg.Arguments[0].NameSetting))
					r.createOutboundMessage(r.ctx, userID, err.Error())
				} else {
					r.createOutboundMessage(r.ctx, userID, resp)
				}
				r.cleanUserState(userID)

			} else if data == "viewSetting" {
				// Обработка просмотра настроек администратором
				if !r.checkAdmin(userID) {
					r.logger.ZapLogger.Warn("Unauthorized setting view attempt", zap.Any("userID", userID))
					r.createOutboundMessage(r.ctx, userID, text_messages.TextUnknownCommand)
					r.cleanUserState(userID)
					continue
				}

				r.logger.ZapLogger.Info("Admin viewing settings...", zap.Any("userID", userID))
				formattedMessage, err := r.service.ViewSetting(r.ctx)
				if err != nil {
					r.logger.ZapLogger.Error("Failed to view settings", zap.Error(err), zap.Any("userID", userID))
					r.createOutboundMessage(r.ctx, userID, "⚠️ Ошибка при получении данных: "+err.Error())
					r.cleanUserState(userID)
					continue
				}
				r.createOutboundMessage(r.ctx, userID, formattedMessage)

				r.logger.ZapLogger.Info("Admin viewed settings", zap.Any("userID", userID))
				r.cleanUserState(userID)

			} else if data == "rebootCache" {
				// Обработка просмотра настроек администратором
				if !r.checkAdmin(userID) {
					r.logger.ZapLogger.Warn("Unauthorized setting view attempt", zap.Any("userID", userID))
					r.createOutboundMessage(r.ctx, userID, text_messages.TextUnknownCommand)
					r.cleanUserState(userID)
					continue
				}
				r.logger.ZapLogger.Info("Admin rebooting cache...", zap.Any("userID", userID))
				err := r.service.RebootCacheData(r.ctx)
				if err != nil {
					r.logger.ZapLogger.Error("Failed to reboot cache", zap.Error(err), zap.Any("userID", userID))
					r.createOutboundMessage(r.ctx, userID, "⚠️ Ошибка при перезагрузке кэша: "+err.Error())
					r.cleanUserState(userID)
					continue
				}
				r.createOutboundMessage(r.ctx, userID, "Кэш успешно перезагружен")
				r.cleanUserState(userID)

			} else if data == "admin" {
				// Выводим админские команды
				if !r.checkAdmin(userID) {
					r.logger.ZapLogger.Warn("Unauthorized setting view attempt", zap.Any("userID", userID))
					r.createOutboundMessage(r.ctx, userID, text_messages.TextUnknownCommand)
					r.cleanUserState(userID)
					continue
				}

				resp := text_messages.TextAdmin()
				r.createOutboundMessage(r.ctx, userID, resp)
				r.cleanUserState(userID)
			
			// Admin command handler for "addsub", "getsub", "updatesub"
			} else if data == "addsub" || data == "getsub" || data == "updatesub" {
				if !r.checkAdmin(userID) {
					r.logger.ZapLogger.Warn("Unauthorized admin command attempt", zap.String("command", data), zap.Any("userID", userID))
					r.createOutboundMessage(r.ctx, userID, text_messages.TextUnknownCommand)
					r.cleanUserState(userID)
					continue
				}
				resp, err := r.service.AdminCommandHandler(r.ctx, data)
				if err != nil {
					r.logger.ZapLogger.Error("AdminCommandHandler failed", zap.String("command", data), zap.Error(err), zap.Any("userID", userID))
					r.createOutboundMessage(r.ctx, userID, "⚠️ Ошибка при выполнении команды: "+err.Error())
					r.cleanUserState(userID)
					continue
				}
				r.createOutboundMessage(r.ctx, userID, resp)
				r.cleanUserState(userID)

			} else {
				//2 лог
				r.logger.ZapLogger.Info("User entered an unknown command...", zap.Any("userID", userID))
				r.createOutboundMessage(r.ctx, userID, text_messages.TextUnknownCommand)
				r.cleanUserState(userID)
			}

			// TODO посмотреть все истории

		}
	}
}

func (r *StoryRouterImpl) AddComand(ctx context.Context, data string, userID int64, msgID int, arguments []models.Argument) {
	select {
	case <-r.ctx.Done():
		return
	case <-ctx.Done():
		return
	case r.chan_command <- models.NewIncommingMessage(data, userID, msgID, arguments):
	}
}
func (r *StoryRouterImpl) AddPaymentQuery(ctx context.Context, userID int64, payload string, queryId string, amount int, currency string, chargeID string) {
	select {
	case <-r.ctx.Done():
		return
	case <-ctx.Done():
		return
	case r.chan_payments <- models.NewPaymentData(queryId, currency, payload, amount, userID, chargeID):
	}
}
func (r *StoryRouterImpl) GetRouterChans() (chan models.OutboundMessage, chan models.EditMessage, chan models.DeleteMessage, chan models.InvoiceMessage, chan *models.PaymentData) {
	return r.chan_outbound, r.chan_edit, r.chan_delete, r.chan_bot_invoice, r.chan_outbound_payments
}

func (r *StoryRouterImpl) CloseInputChans() {
	close(r.chan_payments)
	close(r.chan_command)
}

func (r *StoryRouterImpl) Stop() {
	r.cancel()
	r.wg.Wait()
	close(r.chan_outbound)
	close(r.chan_edit)
	close(r.chan_delete)
	close(r.chan_bot_invoice)
	close(r.chan_outbound_payments)
	r.logger.ZapLogger.Debug("Successful stopped Router-Workers")
}
