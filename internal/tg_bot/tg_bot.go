package tgbot

import (
	"bot_story_generator/internal/config"
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/models"
	"bot_story_generator/internal/text_messages"
	"context"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

type Bot struct {
	ctx           context.Context
	cancel        context.CancelFunc
	updatesChan   tgbotapi.UpdatesChannel
	api           *tgbotapi.BotAPI
	logger        *logger.Logger
	router        StoryRouter
	wg            *sync.WaitGroup
	numworkers    int
	payload_msgId map[string]int
	mux           *sync.Mutex
}

type StoryRouter interface {
	AddComand(ctx context.Context, data string, userID int64, msgID int, arguments []models.Argument, trace models.Trace)
	AddPaymentQuery(ctx context.Context, userID int64, payload string, queryId string, amount int, currency string, chargeID string, trace models.Trace)
	GetRouterChans() (chan models.OutboundMessage, chan models.EditMessage, chan models.DeleteMessage, chan models.InvoiceMessage, chan *models.PaymentData)
	CloseInputChans()
}

func NewBot(cfg *config.Config, logger *logger.Logger, router StoryRouter) (*Bot, error) {
	bot, err := tgbotapi.NewBotAPI(cfg.Telegram.BotToken)
	if err != nil {
		return nil, err
	}

	bot.Debug = cfg.Telegram.BotDebug
	if bot.Debug {
		logger.ZapLogger.Debug("Telegram bot debug mode is enabled")
	}

	logger.ZapLogger.Debug("Authorized on account " + bot.Self.UserName)
	u := tgbotapi.NewUpdate(cfg.Telegram.Offset)
	u.Timeout = cfg.Telegram.Timeout

	updates := bot.GetUpdatesChan(u)
	ctx, cancel := context.WithCancel(context.Background())
	return &Bot{
		ctx:           ctx,
		cancel:        cancel,
		api:           bot,
		logger:        logger,
		updatesChan:   updates,
		router:        router,
		wg:            &sync.WaitGroup{},
		numworkers:    cfg.Setting.NumWorkers,
		payload_msgId: make(map[string]int),
		mux:           &sync.Mutex{},
	}, nil
}

func (bot *Bot) StartBot() {
	outbound, edit, delete, invoiceChan, pdchan := bot.router.GetRouterChans()

	//maybe increase the number of worker-bots(field = numworkers)
	bot.wg.Add(6)
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
	go func() {
		defer bot.wg.Done()
		bot.sendInvoiceMessage(invoiceChan)
	}()
	go func() {
		defer bot.wg.Done()
		bot.sendPaymentData(pdchan)
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
			//executionTime у лога пишем только при отправке действия в ТГ-АПИ
			trace := models.NewTrace()
			// Обработка pre-checkout запроса для платежей (Stars/XTR)
			// PreCheckoutQuery обрабатывается в боте, так как это системный запрос Telegram API
			//обратиться за проверкой существования заказа в базу данных
			//асинхронно получить ответ о существовании и подтвердить answerPreCheckoutQuery
			if update.PreCheckoutQuery != nil {
				//можно было бы удалить, но я хз где взять айди invoice
				query := update.PreCheckoutQuery
				id := query.ID
				payload := query.InvoicePayload
				userID := query.From.ID
				amount := query.TotalAmount
				currency := query.Currency
				//1 лог
				bot.logger.ZapLogger.Info("Received PreCheckout query", zap.Any("userID", userID), zap.Any("payload", payload), zap.Any("traceID", trace.ID))
				bot.router.AddPaymentQuery(bot.ctx, userID, payload, id, amount, currency, "", trace)
				continue
			}
			//можно будет раскидать по отдельным каналам precheck и successPayment
			// Обработка успешной оплаты
			// Передаем данные в роутер для сохранения в БД и отправки сообщения
			if update.Message != nil && update.Message.SuccessfulPayment != nil {
				payment := update.Message.SuccessfulPayment
				userID := update.Message.From.ID
				chargeID := payment.TelegramPaymentChargeID
				payload := payment.InvoicePayload
				amount := payment.TotalAmount
				currency := payment.Currency
				//1 лог
				bot.logger.ZapLogger.Info("Received successful payment", zap.Any("userID", userID), zap.Any("payload", payload), zap.Any("traceID", trace.ID))
				bot.router.AddPaymentQuery(bot.ctx, userID, payload, "", amount, currency, chargeID, trace)
				continue
			}

			if update.CallbackQuery != nil {
				data := update.CallbackQuery.Data
				userID := update.CallbackQuery.From.ID
				msgID := update.CallbackQuery.Message.MessageID
				//1 лог
				bot.logger.ZapLogger.Info("Received CallbackQuery", zap.Any("userID", userID), zap.Any("data", data), zap.Any("traceID", trace.ID))
				bot.router.AddComand(bot.ctx, data, userID, msgID, nil, trace)
				bot.api.Request(tgbotapi.NewCallback(update.CallbackQuery.ID, ""))
			} else if update.Message != nil {
				text := update.Message.Text
				msg := update.Message
				userID := update.Message.From.ID
				msgID := msg.MessageID
				if msg.IsCommand() {
					//1 лог
					bot.logger.ZapLogger.Info("Received Command", zap.Any("userID", userID), zap.Any("data", text), zap.Any("traceID", trace.ID))
					command := update.Message.Command()
					//передаем обновление: /changeSetting limit.day.premium=20
					parts := strings.Fields(text)
					if len(parts) > 1 {
						var arguments []models.Argument
						tokens := parts[1:]
						for _, t := range tokens {
							if strings.Contains(t, "=") {
								kv := strings.SplitN(t, "=", 2)
								arguments = append(arguments, models.Argument{NameSetting: kv[0], ValueSetting: kv[1]})
							}
						}
						bot.router.AddComand(bot.ctx, command, userID, msgID, arguments, trace)
					} else {
						bot.router.AddComand(bot.ctx, command, userID, msgID, nil, trace)
					}
				} else {
					//обычные сообщения также игнорируются
					bot.logger.ZapLogger.Info("Received Message", zap.Any("userID", userID), zap.Any("data", text), zap.Any("traceID", trace.ID))
					bot.sendMessage(userID, text_messages.TextUnknownCommand, nil, trace)
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
			trace := editMsg.Trace
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
					"Failed to edit message",
					zap.Error(err),
					zap.Any("userID", editMsg.UserID),
					zap.Any("msgID", editMsg.MsgID),
					zap.Any("traceID", trace.ID),
					zap.Any("executionTime", time.Since(trace.StartTime)),
				)
				continue
			}
			//последний лог
			bot.logger.ZapLogger.Info(
				"Message edited successfully",
				zap.Any("userID", editMsg.UserID),
				zap.Any("msgID", editMsg.MsgID),
				zap.Any("traceID", trace.ID),
				zap.Any("executionTime", time.Since(trace.StartTime)),
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
			trace := deleteMsg.Trace
			del := tgbotapi.NewDeleteMessage(deleteMsg.UserID, deleteMsg.MsgID)
			_, err := bot.api.Request(del)
			if err != nil {
				bot.logger.ZapLogger.Error(
					"Failed to delete message",
					zap.Error(err),
					zap.Any("userID", deleteMsg.UserID),
					zap.Any("msgID", deleteMsg.MsgID),
					zap.Any("traceID", trace.ID),
					zap.Any("executionTime", time.Since(trace.StartTime)),
				)
				continue
			}
			//последний лог
			bot.logger.ZapLogger.Info(
				"Message deleted successfully",
				zap.Any("userID", deleteMsg.UserID),
				zap.Any("msgID", deleteMsg.MsgID),
				zap.Any("traceID", trace.ID),
				zap.Any("executionTime", time.Since(trace.StartTime)),
			)

		}
	}
}
func (bot *Bot) getInvoiceId(payload string, userID int64, queryId string, trace models.Trace) int {
	bot.mux.Lock()
	msgId, ok := bot.payload_msgId[payload]
	if !ok {
		bot.mux.Unlock()
		//если в мапе нет ключа по payload - делаем запрос на ошибку checkoutQuery с единым ответом ошибки
		bot.logger.ZapLogger.Error("Invoice's messageID for payload not found", zap.Any("userID", userID), zap.String("payload", payload), zap.Any("traceID", trace.ID), zap.Any("executionTime", time.Since(trace.StartTime)))
		_, err := bot.api.MakeRequest("answerPreCheckoutQuery", tgbotapi.Params{"pre_checkout_query_id": queryId, "ok": "false", "error_message": text_messages.TextErrorProcessPayment})
		if err != nil {
			bot.logger.ZapLogger.Error("Failed to make request with Invalid payment data", zap.Error(err), zap.Any("userID", userID), zap.Any("payload", payload), zap.Any("traceID", trace.ID), zap.Any("executionTime", time.Since(trace.StartTime)))
		} else {
			bot.logger.ZapLogger.Info("Made false-request to invoice successfully", zap.Any("userID", userID), zap.Any("payload", payload), zap.Any("traceID", trace.ID), zap.Any("executionTime", time.Since(trace.StartTime)))
		}
		return 0
	}
	delete(bot.payload_msgId, payload)
	bot.mux.Unlock()
	return msgId
}
func (bot *Bot) sendPaymentData(ch chan *models.PaymentData) {
	for {
		select {
		case <-bot.ctx.Done():
			return
		case data, ok := <-ch:
			//разделить в будущем сущности preCheck и successPayment
			if !ok {
				return
			}
			trace := data.Trace
			if data.Error != nil && data.ChargeID == "" && data.QueryID != "" {
				msgId := bot.getInvoiceId(data.InvoicePayload, data.UserID, data.QueryID, trace)
				if msgId == 0 {
					continue
				}
				//если в мапе есть ключ по payload - делаем запрос на ошибку checkoutQuery которая произошла в сервисе
				_, err := bot.api.MakeRequest("answerPreCheckoutQuery", tgbotapi.Params{"pre_checkout_query_id": data.QueryID, "ok": "false", "error_message": data.Error.Error()})
				if err != nil {
					bot.logger.ZapLogger.Error("Failed to make request with Invalid payment data", zap.Error(err), zap.Any("userID", data.UserID), zap.Any("payload", data.InvoicePayload), zap.Any("traceID", trace.ID), zap.Any("executionTime", time.Since(trace.StartTime)))
				} else {
					bot.logger.ZapLogger.Info("Made false-request to invoice successfully", zap.Any("userID", data.UserID), zap.Any("payload", data.InvoicePayload), zap.Any("traceID", trace.ID), zap.Any("executionTime", time.Since(trace.StartTime)))
				}
				//удаляем invoice сообщение
				del := tgbotapi.NewDeleteMessage(data.UserID, msgId)
				_, err = bot.api.Request(del)
				if err != nil {
					bot.logger.ZapLogger.Error("Failed to delete invoice message", zap.Error(err), zap.Any("userID", data.UserID), zap.Any("payload", data.InvoicePayload), zap.Any("traceID", trace.ID), zap.Any("executionTime", time.Since(trace.StartTime)))
				} else {
					bot.logger.ZapLogger.Info("Invoice message deleted successfully", zap.Any("userID", data.UserID), zap.Any("payload", data.InvoicePayload), zap.Any("traceID", trace.ID), zap.Any("executionTime", time.Since(trace.StartTime)))
				}
			} else if data.Error == nil && data.ChargeID == "" && data.QueryID != "" {
				msgId := bot.getInvoiceId(data.InvoicePayload, data.UserID, data.QueryID, trace)
				if msgId == 0 {
					continue
				}
				_, err := bot.api.MakeRequest("answerPreCheckoutQuery", tgbotapi.Params{"pre_checkout_query_id": data.QueryID, "ok": "true"})
				if err != nil {
					bot.logger.ZapLogger.Error("Failed to answer pre-checkout query", zap.Error(err), zap.Any("userID", data.UserID), zap.Any("payload", data.InvoicePayload))
				} else {
					bot.logger.ZapLogger.Info("Made true-request to invoice successfully", zap.Any("userID", data.UserID), zap.Any("payload", data.InvoicePayload), zap.Any("traceID", trace.ID), zap.Any("executionTime", time.Since(trace.StartTime)))
				}
				//удаляем invoice сообщение
				del := tgbotapi.NewDeleteMessage(data.UserID, msgId)
				_, err = bot.api.Request(del)
				if err != nil {
					bot.logger.ZapLogger.Error("Failed to delete invoice message", zap.Error(err), zap.Any("userID", data.UserID), zap.Any("payload", data.InvoicePayload))
				} else {
					bot.logger.ZapLogger.Info("Invoice message deleted successfully", zap.Any("userID", data.UserID), zap.Any("payload", data.InvoicePayload), zap.Any("traceID", trace.ID), zap.Any("executionTime", time.Since(trace.StartTime)))
				}
			} else if data.Error != nil && data.ChargeID != "" && data.QueryID == "" {
				bot.sendMessage(data.UserID, data.Error.Error(), nil, trace)
			} else if data.Error == nil && data.ChargeID != "" && data.QueryID == "" {
				bot.sendMessage(data.UserID, text_messages.TextSubscriptionActivated, nil, trace)
			}
		}
	}
}
func (bot *Bot) sendInvoiceMessage(ch chan models.InvoiceMessage) {
	for {
		select {
		case <-bot.ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}

			//bot.sendMessage(msg.Subscription.UserID, text_messages.TextSendInvoiceSubscription, nil)
			bot.sendSubscriptionInvoice(msg.Subscription, msg.Trace)
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
			trace := outMsg.Trace
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
			msg, err := bot.sendMessage(outMsg.UserID, text[0], outMsg.ButtonArgs, trace)
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
				bot.logger.ZapLogger.Warn("Context value for 'delete' is not a string", zap.Any("value", value), zap.Any("userID", outMsg.UserID), zap.Any("traceID", trace.ID), zap.Any("executionTime", time.Since(trace.StartTime)))
				continue
			}
			if isDelete == "1" {
				bot.wg.Add(1)
				go func() {
					defer bot.wg.Done()
					bot.waitingMessageWithAnimation(localctx, msg, outMsg.UserID, text, trace)
				}()
			}
		}
	}
}

func (bot *Bot) sendMessage(userID int64, text string, butarg []models.ButtonArg, trace models.Trace) (tgbotapi.Message, error) {
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
			"Failed to send message",
			zap.Error(err),
			zap.Any("userID", userID),
			zap.Any("traceID", trace.ID),
			zap.Any("executionTime", time.Since(trace.StartTime)),
		)
		return sentmsg, err
	}
	bot.logger.ZapLogger.Info(
		"Message sent successfully",
		zap.Any("userID", userID),
		zap.Any("msgID", sentmsg.MessageID),
		zap.Any("traceID", trace.ID),
		zap.Any("executionTime", time.Since(trace.StartTime)),
	)

	return sentmsg, nil
}

func (bot *Bot) waitingMessageWithAnimation(ctx context.Context, sentMsg tgbotapi.Message, userID int64, inputText []string, trace models.Trace) {
	currentIdx := 1
	if len(inputText) == 1 {
		currentIdx = 0
	}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			del := tgbotapi.NewDeleteMessage(userID, sentMsg.MessageID)
			_, err := bot.api.Request(del)
			if err != nil {
				bot.logger.ZapLogger.Error(
					"Failed to delete loading message",
					zap.Error(err),
					zap.Any("userID", userID),
					zap.Any("msgID", sentMsg.MessageID),
					zap.Any("traceID", trace.ID),
					zap.Any("executionTime", time.Since(trace.StartTime)),
				)
			} else {
				bot.logger.ZapLogger.Info(
					"Loading message deleted successfully",
					zap.Any("userID", userID),
					zap.Any("msgID", sentMsg.MessageID),
					zap.Any("traceID", trace.ID),
					zap.Any("executionTime", time.Since(trace.StartTime)),
				)
			}
			return
		case <-bot.ctx.Done():
			del := tgbotapi.NewDeleteMessage(userID, sentMsg.MessageID)
			_, err := bot.api.Request(del)
			if err != nil {
				bot.logger.ZapLogger.Error(
					"Failed to delete loading message",
					zap.Error(err),
					zap.Any("userID", userID),
					zap.Any("msgID", sentMsg.MessageID),
					zap.Any("traceID", trace.ID),
					zap.Any("executionTime", time.Since(trace.StartTime)),
				)
			} else {
				bot.logger.ZapLogger.Info(
					"Loading message deleted successfully",
					zap.Any("userID", userID),
					zap.Any("msgID", sentMsg.MessageID),
					zap.Any("traceID", trace.ID),
					zap.Any("executionTime", time.Since(trace.StartTime)),
				)
			}
			return
		case <-ticker.C:
			editMsg := tgbotapi.NewEditMessageText(userID, sentMsg.MessageID, inputText[currentIdx])
			_, err := bot.api.Send(editMsg)
			if err != nil {
				bot.logger.ZapLogger.Error(
					"Failed to edit loading message",
					zap.Error(err),
					zap.Any("userID", userID),
					zap.Any("msgID", sentMsg.MessageID),
					zap.Any("traceID", trace.ID),
					zap.Any("executionTime", time.Since(trace.StartTime)),
				)
			}
			currentIdx = (currentIdx + 1) % len(inputText)
		}
	}
}

// * Функция отправки инвойса с подпиской
func (bot *Bot) sendSubscriptionInvoice(sub *models.Subscription, trace models.Trace) {
	name := text_messages.NameBasicSubscription
	description := text_messages.DescriptionBasicSubscription
	var provideToken string
	var startParameter string
	if sub.Currency == "XTR" {
		provideToken = ""
		startParameter = ""
	} else {
		provideToken = ""
		startParameter = ""
	}
	// Диагностика: логируем сумму и userID - диагноз: у вас аутизм)
	bot.logger.ZapLogger.Info("Subscription invoice prepare", zap.Any("amount", sub.Price), zap.Any("userID", sub.UserID), zap.Any("payload", sub.Payload), zap.Any("traceID", trace.ID), zap.Any("executionTime", time.Since(trace.StartTime)))

	if sub.Price <= 0 {
		bot.logger.ZapLogger.Warn("Subscription price is zero or negative, aborting invoice send", zap.Any("amount", sub.Price), zap.Any("userID", sub.UserID), zap.Any("payload", sub.Payload), zap.Any("traceID", trace.ID), zap.Any("executionTime", time.Since(trace.StartTime)))
		bot.sendMessage(sub.UserID, text_messages.TextSubPriceError, nil, trace)
		return
	}

	prices := []tgbotapi.LabeledPrice{
		{Label: description, Amount: sub.Price},
	}

	invoice := tgbotapi.NewInvoice(
		sub.UserID,
		name,
		description,
		sub.Payload,
		provideToken,
		startParameter,
		sub.Currency,
		prices,
	)

	// Предлагаем чаевые при оплате подписки, но для XTR их нет, но строку надо оставить, иначе ошибка
	invoice.SuggestedTipAmounts = []int{}
	msg, err := bot.api.Send(invoice)
	if err != nil {
		bot.logger.ZapLogger.Error("Error sending invoice", zap.Error(err), zap.Any("userID", sub.UserID), zap.Any("payload", sub.Payload), zap.Any("traceID", trace.ID), zap.Any("executionTime", time.Since(trace.StartTime)))
	} else {
		bot.logger.ZapLogger.Info("Invoice sent", zap.Any("userID", sub.UserID), zap.Any("msgID", msg.MessageID), zap.Any("payload", sub.Payload), zap.Any("traceID", trace.ID), zap.Any("executionTime", time.Since(trace.StartTime)))
		bot.mux.Lock()
		bot.payload_msgId[sub.Payload] = msg.MessageID
		bot.mux.Unlock()
	}

}

func (bot *Bot) Stop() {
	bot.cancel()
	bot.wg.Wait()
	bot.router.CloseInputChans()
	bot.logger.ZapLogger.Debug("Successful stop Telegram Bot")
}
