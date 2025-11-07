package router

import (
	"bot_story_generator/internal/config"
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/models"
	"bot_story_generator/internal/text_messages"
	"context"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

type StoryService interface {
	CreateStory(ctx context.Context, userID int64) ([]string, error)
	UserChoice(ctx context.Context, userID int64, arg string) ([]string, error)
	CreateUser(ctx context.Context, userID int64) ([]string, error)
	StopStory(ctx context.Context, userID int64) ([]string, error)
	StopStoryChoice(ctx context.Context, userID int64, arg string) ([]string, error)
	AddSubscription(ctx context.Context, subscription *models.Subscription) error
	GetUserSubscription(ctx context.Context, userID int64) (*models.Subscription, error)
}

type StoryRouterImpl struct {
	ctx           context.Context
	cancel        context.CancelFunc
	service       StoryService
	chan_command  chan models.IncommingMessage
	chan_outbound chan models.OutboundMessage
	chan_edit     chan models.EditMessage
	chan_delete   chan models.DeleteMessage
	chan_bot_cmd  chan models.BotCommand
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
		chan_edit:     make(chan models.EditMessage, 1000),
		chan_delete:   make(chan models.DeleteMessage, 1000),
		chan_bot_cmd:  make(chan models.BotCommand, 1000),
		userState:     make(map[int64]struct{}),
		mux:           &sync.Mutex{},
		logger:        logger,
		wg:            &sync.WaitGroup{},
		numworkers:    cfg.Setting.NumWorkers,
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
			msgID := msg.MsgID
			// args := msg.Arguments

			if data == "start" {
				//2 лог
				r.logger.ZapLogger.Info("Creating user...", zap.Any("userID", userID))
				resp, err := r.service.CreateUser(r.ctx, userID)
				if err != nil {
					r.createOutboundMessage(r.ctx, userID, err.Error())
				} else {
					r.createOutboundMessage(r.ctx, userID, resp[0])
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
					r.createOutboundMessage(r.ctx, userID, resp[0], models.NewButtonArg("stopStoryChoice_", []string{"✅", "❌"}))
				}
				r.cleanUserState(userID)

			} else if strings.HasPrefix(data, "stopStoryChoice_") {
				//2 лог
				r.logger.ZapLogger.Info("User making a stop story choice...", zap.Any("userID", userID))
				arg := strings.TrimPrefix(data, "stopStoryChoice_")
				resp, err := r.service.StopStoryChoice(r.ctx, userID, arg)
				r.createDeleteMessage(userID, msgID)
				if resp == nil && err == nil {
					r.cleanUserState(userID)
					continue
				}
				if err != nil {
					r.createOutboundMessage(r.ctx, userID, err.Error())
					r.cleanUserState(userID)
					continue
				}
				r.createOutboundMessage(r.ctx, userID, resp[0])
				r.cleanUserState(userID)

			} else if data == "canselSubscription" {
				// Эта команда разрешена только для определённых chatID
				allowedChatIDs := map[int64]struct{}{
					1370660713: {}, // заменить на реальные chatID
					0000:       {}, // свой id добавь
				}
				if _, ok := allowedChatIDs[userID]; ok {
					// TODO вынести потом в сервис это
					// * То, что щас, делала ии
					// Получаем подписку пользователя из БД
					subscription, err := r.service.GetUserSubscription(r.ctx, userID)
					if err != nil {
						r.logger.ZapLogger.Error("Failed to get user subscription", zap.Error(err), zap.Any("userID", userID))
						r.createOutboundMessage(r.ctx, userID, "Ошибка при получении данных подписки. Обратитесь в поддержку.")
						r.cleanUserState(userID)
						continue
					}

					if subscription == nil || subscription.ChargeId == "" {
						r.logger.ZapLogger.Warn("No active subscription found for user", zap.Any("userID", userID))
						r.createOutboundMessage(r.ctx, userID, "У вас нет активной подписки для отмены.")
						r.cleanUserState(userID)
						continue
					}

					cmd := models.BotCommand{
						Type:     models.BotCommandCancelSubscription,
						UserID:   userID,
						ChargeID: subscription.ChargeId,
					}
					r.SendBotCommand(cmd)
					r.createOutboundMessage(r.ctx, userID, "Запрос на отмену подписки отправлен.")

					//TODO отменить подписку в бд
				}

				r.cleanUserState(userID)

			} else if data == "successful_payment" {
				// Обработка успешной оплаты подписки
				//2 лог
				r.logger.ZapLogger.Info("Processing successful payment...", zap.Any("userID", userID))

				// Получаем данные платежа из arguments
				paymentData, ok := msg.Arguments.(*models.PaymentData)
				if !ok || paymentData == nil {
					r.logger.ZapLogger.Error("Invalid payment data format", zap.Any("userID", userID))
					r.createOutboundMessage(r.ctx, userID, "Ошибка обработки платежа. Обратитесь в поддержку.")
					r.cleanUserState(userID)
					continue
				}

				if paymentData.ChargeID == "" {
					r.logger.ZapLogger.Error("Missing or invalid charge_id", zap.Any("userID", userID))
					r.createOutboundMessage(r.ctx, userID, "Ошибка обработки платежа. Обратитесь в поддержку.")
					r.cleanUserState(userID)
					continue
				}

				// TODO вынести потом в сервис это
				// * То, что щас, делала ии
				// TODO время на которае дается подписка убрать в из хардкора
				// Сохраняем подписку в БД через сервис
				subscription := models.NewSubscription(paymentData.ChargeID, userID, "basic", time.Now().AddDate(0, 0, 30))
				err := r.service.AddSubscription(r.ctx, subscription)
				if err != nil {
					r.logger.ZapLogger.Error("Failed to add subscription", zap.Error(err), zap.Any("userID", userID))
					r.createOutboundMessage(r.ctx, userID, "Ошибка активации подписки. Обратитесь в поддержку.")
					r.cleanUserState(userID)
					continue
				}

				r.logger.ZapLogger.Info("Payment processed successfully",
					zap.Any("userID", userID),
					zap.String("charge_id", paymentData.ChargeID),
					zap.String("currency", paymentData.Currency),
					zap.Int("total_amount", paymentData.TotalAmount),
				)
				r.createOutboundMessage(r.ctx, userID, "Подписка активирована! Наслаждайтесь неограниченными историями.")
				r.cleanUserState(userID)

			} else if data == "buy_subscription" {
				// * То, что щас, делала ии
				// Обработка команды покупки подписки
				r.logger.ZapLogger.Info("User requested to buy subscription", zap.Any("userID", userID))

				// Здесь отправим команду боту на создание и отправку инвойса пользователю
				cmd := models.BotCommand{
					Type:   models.BotCommandSendSubscriptionInvoice,
					UserID: userID,
					ChatID: userID, // для Telegram ChatID == UserID для личных чатов
				}
				r.SendBotCommand(cmd)

				r.createOutboundMessage(r.ctx, userID, "Счёт на оплату отправлен. Следуйте инструкциям Telegram для завершения покупки подписки.")
				r.cleanUserState(userID)
			} else {
				//2 лог
				r.logger.ZapLogger.Info("User entered an unknown command...", zap.Any("userID", userID))
				r.createOutboundMessage(r.ctx, userID, text_messages.TextUnknownCommand)
				r.cleanUserState(userID)
			}

			// TODO проверить подписку

			// TODO посмотреть все истории

		}
	}
}

func (r *StoryRouterImpl) AddComand(ctx context.Context, data string, userID int64, msgID int, arguments interface{}) {
	select {
	case <-r.ctx.Done():
		return
	case <-ctx.Done():
		return
	case r.chan_command <- models.NewIncommingMessage(data, userID, msgID, arguments):
	}
}

func (r *StoryRouterImpl) GetRouterChans() (chan models.OutboundMessage, chan models.EditMessage, chan models.DeleteMessage, chan models.BotCommand) {
	return r.chan_outbound, r.chan_edit, r.chan_delete, r.chan_bot_cmd
}

func (r *StoryRouterImpl) GetBotCommandChan() chan models.BotCommand {
	return r.chan_bot_cmd
}

// SendBotCommand отправляет команду боту для выполнения
func (r *StoryRouterImpl) SendBotCommand(cmd models.BotCommand) {
	select {
	case <-r.ctx.Done():
		return
	case r.chan_bot_cmd <- cmd:
	}
}

func (r *StoryRouterImpl) CloseCommandChan() {
	close(r.chan_command)
}

func (r *StoryRouterImpl) Stop() {
	r.cancel()
	r.wg.Wait()
	close(r.chan_outbound)
	close(r.chan_edit)
	close(r.chan_delete)
	close(r.chan_bot_cmd)
	r.logger.ZapLogger.Debug("Successful stopped Router-Workers")
}
