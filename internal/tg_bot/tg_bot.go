package tgbot

import (
	"bot_story_generator/internal/config"
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/models"
	"context"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

type Bot struct {
	ctx                    context.Context
	cancel                 context.CancelFunc
	updatesChan            tgbotapi.UpdatesChannel
	api                    *tgbotapi.BotAPI
	logger                 *logger.Logger
	router                 StoryRouter
	wg                     *sync.WaitGroup
	numworkers             int
	priceBasicSubscription int
}

type StoryRouter interface {
	AddComand(ctx context.Context, data string, userID int64, msgID int)
	GetRouterChans() (chan models.OutboundMessage, chan models.EditMessage, chan models.DeleteMessage)
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
		ctx:                    ctx,
		cancel:                 cancel,
		api:                    bot,
		logger:                 logger,
		updatesChan:            updates,
		router:                 router,
		wg:                     &sync.WaitGroup{},
		numworkers:             cfg.NumWorkers,
		priceBasicSubscription: cfg.Setting.PriceBasicSubscription,
	}, nil
}

func (bot *Bot) StartBot() {
	outbound, edit, delete := bot.router.GetRouterChans()
	//maybe increase the number of worker-bots(field = numworkers)
	bot.wg.Add(4)
	go func() {
		defer bot.wg.Done()
		bot.readIncommingMessage()
	}()
	go func() {
		defer bot.wg.Done()
		bot.sendOutboundMessage(outbound)
	}()
	go func() {
		defer bot.wg.Done()
		bot.sendEditMessage(edit)
	}()
	go func() {
		defer bot.wg.Done()
		bot.sendDeleteMessage(delete)
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

			//! СДЕЛАЛ ИИ
			//TODO проверить и передалать, сдеоаю потом. Пока планировка
			// Обработка pre-checkout запроса для платежей (в т.ч. Stars/XTR)
			if update.PreCheckoutQuery != nil {
				query := update.PreCheckoutQuery
				if query.InvoicePayload != "sub_payload_unique" {
					_, _ = bot.api.MakeRequest("answerPreCheckoutQuery", tgbotapi.Params{
						"pre_checkout_query_id": query.ID,
						"ok":                    "false",
						"error_message":         "Invalid payload",
					})
					continue
				}
				_, err := bot.api.MakeRequest("answerPreCheckoutQuery", tgbotapi.Params{
					"pre_checkout_query_id": query.ID,
					"ok":                    "true",
				})
				if err != nil {
					bot.logger.ZapLogger.Error("failed to answer pre-checkout", zap.Error(err))
				}
				continue
			}
			// Обработка успешной оплаты
			if update.Message != nil && update.Message.SuccessfulPayment != nil {
				payment := update.Message.SuccessfulPayment
				userID := update.Message.From.ID
				chargeID := payment.ProviderPaymentChargeID
				untilDate := time.Now().AddDate(0, 0, 30) // +30 дней
				bot.logger.ZapLogger.Info("subscription activated", zap.Int64("user_id", userID), zap.String("charge_id", payment.ProviderPaymentChargeID))
				confirm := tgbotapi.NewMessage(userID, "Subscription active! Enjoy unlimited stories.")
				if _, err := bot.api.Send(confirm); err != nil {
					bot.logger.ZapLogger.Error("failed to send confirmation", zap.Error(err))
				}
				continue
			}
			//! Конец кода ии

			if update.CallbackQuery != nil {
				data := update.CallbackQuery.Data
				userID := update.CallbackQuery.From.ID
				msgID := update.CallbackQuery.Message.MessageID
				bot.logger.ZapLogger.Info("Received update", zap.Any("data", data), zap.Any("userID", userID))
				bot.router.AddComand(bot.ctx, data, userID, msgID)
				bot.api.Request(tgbotapi.NewCallback(update.CallbackQuery.ID, ""))
			} else if update.Message != nil {
				text := update.Message.Text
				msg := update.Message
				userID := update.Message.From.ID
				msgID := msg.MessageID
				if msg.IsCommand() {
					bot.logger.ZapLogger.Info("Received update", zap.Any("data", text), zap.Any("userID", userID))
					command := update.Message.Command()
					bot.router.AddComand(bot.ctx, command, userID, msgID)
				}
			}
		}
	}
}

func (bot *Bot) sendEditMessage(ch chan models.EditMessage) {
	for {
		select {
		case <-bot.ctx.Done():
			return
		case editMsg, ok := <-ch:
			if !ok {
				return
			}

			var keyboard tgbotapi.InlineKeyboardMarkup
			if len(editMsg.ButtonArgs) > 0 {
				var rows [][]tgbotapi.InlineKeyboardButton
				for _, btn := range editMsg.ButtonArgs {
					for _, arg := range btn.Args {
						button := tgbotapi.NewInlineKeyboardButtonData(arg, btn.ButtonName+arg)
						row := tgbotapi.NewInlineKeyboardRow(button)
						rows = append(rows, row)
					}
				}
				keyboard = tgbotapi.NewInlineKeyboardMarkup(rows...)
			} else {
				keyboard = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}}
			}
			var edit tgbotapi.Chattable
			if editMsg.Text == "" {
				edit = tgbotapi.NewEditMessageReplyMarkup(
					editMsg.UserID,
					editMsg.MsgID,
					keyboard,
				)

			} else {
				edit = tgbotapi.NewEditMessageTextAndMarkup(
					editMsg.UserID,
					editMsg.MsgID,
					editMsg.Text,
					keyboard,
				)
			}
			_, err := bot.api.Request(edit)
			if err != nil {
				bot.logger.ZapLogger.Error(
					"failed to edit message",
					zap.Error(err),
					zap.Int64("user_id", editMsg.UserID),
					zap.Int("msg_id", editMsg.MsgID),
				)
				continue
			}
			bot.logger.ZapLogger.Info(
				"message edited successfully",
				zap.Int64("user_id", editMsg.UserID),
				zap.Int("msg_id", editMsg.MsgID),
			)
		}
	}
}

func (bot *Bot) sendDeleteMessage(ch chan models.DeleteMessage) {
	for {
		select {
		case <-bot.ctx.Done():
			return
		case deleteMsg, ok := <-ch:
			if !ok {
				return
			}
			del := tgbotapi.NewDeleteMessage(deleteMsg.UserID, deleteMsg.MsgID)
			_, err := bot.api.Request(del)
			if err != nil {
				bot.logger.ZapLogger.Error(
					"failed to delete message",
					zap.Error(err),
					zap.Int64("user_id", deleteMsg.UserID),
					zap.Int("message_id", deleteMsg.MsgID),
				)
			} else {
				bot.logger.ZapLogger.Info(
					"message deleted successfully",
					zap.Int64("user_id", deleteMsg.UserID),
					zap.Int("message_id", deleteMsg.MsgID),
				)
			}
		}
	}
}

func (bot *Bot) sendOutboundMessage(ch chan models.OutboundMessage) {
	for {
		select {
		case <-bot.ctx.Done():
			return
		case outMsg, ok := <-ch:
			if !ok {
				return
			}
			var text []string
			if strings.Contains(outMsg.Text, "---") {
				parts := strings.Split(outMsg.Text, "---")
				for i := range parts {
					parts[i] = strings.TrimSpace(parts[i])
				}
				text = parts
			} else {
				text = []string{strings.TrimSpace(outMsg.Text)}
			}
			msg, err := bot.sendMessage(outMsg.UserID, text[0], outMsg.ButtonArgs)
			if err != nil {
				continue
			}
			//*опционально в будущем придумать логику для выбора разных значений контекста
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
					bot.waitingMessageWithAnimation(localctx, msg, outMsg.UserID, text)
				}()
			}
		}
	}
}

func (bot *Bot) sendMessage(userID int64, text string, butarg []models.ButtonArg) (tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(userID, text)
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
			zap.Int64("user_id", userID),
			zap.String("message", msg.Text),
		)
		return sentmsg, err
	}
	bot.logger.ZapLogger.Info(
		"message sent successfully",
		zap.Int64("user_id", userID),
		zap.String("message", msg.Text),
	)

	return sentmsg, nil
}

func (bot *Bot) waitingMessageWithAnimation(ctx context.Context, sentMsg tgbotapi.Message, userID int64, inputText []string) {
	currentIdx := 1
	if len(inputText) == 1 {
		currentIdx = 0
	}
	bot.logger.ZapLogger.Info(
		"Starting waitingMessageWithAnimation",
		zap.Int64("user_id", userID),
		zap.Int("message_id", sentMsg.MessageID),
	)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			bot.logger.ZapLogger.Info(
				"context cancelled, deleting loading message",
				zap.Int64("user_id", userID),
				zap.Int("message_id", sentMsg.MessageID),
			)
			del := tgbotapi.NewDeleteMessage(userID, sentMsg.MessageID)
			_, err := bot.api.Request(del)
			if err != nil {
				bot.logger.ZapLogger.Error(
					"failed to delete loading message",
					zap.Error(err),
					zap.Int64("user_id", userID),
					zap.Int("message_id", sentMsg.MessageID),
				)
			} else {
				bot.logger.ZapLogger.Info(
					"loading message deleted successfully",
					zap.Int64("user_id", userID),
					zap.Int("message_id", sentMsg.MessageID),
				)
			}
			return
		case <-bot.ctx.Done():
			bot.logger.ZapLogger.Info(
				"context cancelled, deleting loading message",
				zap.Int64("user_id", userID),
				zap.Int("message_id", sentMsg.MessageID),
			)
			del := tgbotapi.NewDeleteMessage(userID, sentMsg.MessageID)
			_, err := bot.api.Request(del)
			if err != nil {
				bot.logger.ZapLogger.Error(
					"failed to delete loading message",
					zap.Error(err),
					zap.Int64("user_id", userID),
					zap.Int("message_id", sentMsg.MessageID),
				)
			} else {
				bot.logger.ZapLogger.Info(
					"loading message deleted successfully",
					zap.Int64("user_id", userID),
					zap.Int("message_id", sentMsg.MessageID),
				)
			}
			return
		case <-ticker.C:
			editMsg := tgbotapi.NewEditMessageText(userID, sentMsg.MessageID, inputText[currentIdx])
			_, err := bot.api.Send(editMsg)
			if err != nil {
				bot.logger.ZapLogger.Error(
					"failed to update waiting message",
					zap.Error(err),
					zap.Int64("user_id", userID),
					zap.Int("message_id", sentMsg.MessageID),
				)
			}
			currentIdx = (currentIdx + 1) % len(inputText)
		}
	}
}

func (bot *Bot) Stop() {
	bot.cancel()
	bot.wg.Wait()
	bot.router.CloseCommandChan()
	bot.logger.ZapLogger.Info("Bot stopped")
}

//! НИЖЕ ПОКА НИЧЕГО НЕ ТРОГАТЬ - потом разнесу по файлам

// ! ПРИМЕР КОДА ИЗ ИИ
// TODO потом нормально настрою, щас делаю планировку
func (bot *Bot) sendSubscriptionInvoice(chatID int64) {
	prices := []tgbotapi.LabeledPrice{
		{Label: "Monthly Unlimited Stories", Amount: bot.priceBasicSubscription}, // Сумма в Stars
	}

	// Формируем invoice для оплаты подписки через Stars/XTR Telegram
	invoice := tgbotapi.NewInvoice(
		chatID,                              // chatID пользователя, которому отправляем инвойс
		"Premium Subscription",              // Название инвойса (видно пользователю)
		"Unlock unlimited story generation", // Описание подписки (видно пользователю)
		"sub_payload_unique",                // Payload, который вернётся боту после оплаты — можно использовать для идентификации типа покупки
		"",                                  // providerToken — токен платежного провайдера. Для Stars (XTR) оставляем пустым
		"XTR",                               // название валюты. Для Telegram Stars нужно использовать "XTR"
		"",                                  // startParameter — строка для deep-link, обычно пустая если не требуется стартовая ссылка
		prices,                              // массив цен (LabeledPrice), здесь одна строка с суммой подписки
	)
	// Примечание: Если используется сторонний провайдер (providerToken), его указываем вместо "", если только Stars — оставлять пустым (поддерживается с tgbotapi v5.13+)
	// Рекуррентные параметры на уровне InvoiceConfig не поддерживаются. Повторные списания на стороне Stars/Telegram.
	msg, err := bot.api.Send(invoice)
	if err != nil {
		log.Println("Error sending invoice:", err)
	} else {
		log.Println("Invoice sent:", msg)
	}
}

// ! ПРИМЕР КОДА ИЗ ИИ
// TODO потом нормально настрою, щас делаю планировку
func (bot *Bot) handleUpdate(update tgbotapi.Update) {
	if update.PreCheckoutQuery != nil {
		query := update.PreCheckoutQuery
		// Проверьте payload и т.д.
		if query.InvoicePayload != "sub_payload_unique" {
			bot.api.MakeRequest("answerPreCheckoutQuery", tgbotapi.Params{
				"pre_checkout_query_id": query.ID,
				"ok":                    "false",
				"error_message":         "Invalid payload",
			})
			return
		}
		_, err := bot.api.MakeRequest("answerPreCheckoutQuery", tgbotapi.Params{
			"pre_checkout_query_id": query.ID,
			"ok":                    "true",
		})
		if err != nil {
			log.Println("Error answering pre-checkout:", err)
		}
	} else if update.Message != nil && update.Message.SuccessfulPayment != nil {
		payment := update.Message.SuccessfulPayment
		userID := update.Message.From.ID
		// Сохраните в БД: userID, subscription_id (из payload или query), until_date = current + period
		// Предоставьте неограниченный доступ, например, обновите статус пользователя
		log.Println("Subscription activated for user:", userID, "Charge ID:", payment.ProviderPaymentChargeID)
		// Отправьте подтверждение
		msg := tgbotapi.NewMessage(userID, "Subscription active! Enjoy unlimited stories.")
		bot.api.Send(msg)
	}
}

// ! ПРИМЕР КОДА ИЗ ИИ
// TODO потом нормально настрою, щас делаю планировку
func (bot *Bot) cancelSubscription(userID int64, chargeID string) {
	params := tgbotapi.Params{
		"user_id":   strconv.FormatInt(userID, 10),
		"charge_id": chargeID,
		"restore":   "false", // Или true для разрешения повторной активации
	}
	_, err := bot.api.MakeRequest("botCancelStarsSubscription", params)
	if err != nil {
		log.Println("Cancel error:", err)
	}
}
