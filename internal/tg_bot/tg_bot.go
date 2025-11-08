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
	AddComand(ctx context.Context, data string, userID int64, msgID int, arguments interface{})
	GetRouterChans() (chan models.OutboundMessage, chan models.EditMessage, chan models.DeleteMessage, chan models.BotCommand)
	CloseCommandChan()
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
		ctx:                    ctx,
		cancel:                 cancel,
		api:                    bot,
		logger:                 logger,
		updatesChan:            updates,
		router:                 router,
		wg:                     &sync.WaitGroup{},
		numworkers:             cfg.Setting.NumWorkers,
		priceBasicSubscription: cfg.Setting.PriceBasicSubscription,
	}, nil
}

func (bot *Bot) StartBot() {
	outbound, edit, delete, cmdChan := bot.router.GetRouterChans()

	//maybe increase the number of worker-bots(field = numworkers)
	bot.wg.Add(5)
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
		bot.sendBotCommand(cmdChan)
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
			// Обработка pre-checkout запроса для платежей (Stars/XTR)
			// PreCheckoutQuery обрабатывается в боте, так как это системный запрос Telegram API
			if update.PreCheckoutQuery != nil {
				query := update.PreCheckoutQuery
				bot.logger.ZapLogger.Info("Received pre-checkout query",
					zap.String("query_id", query.ID),
					zap.String("payload", query.InvoicePayload),
					zap.Int64("user_id", query.From.ID),
				)

				// Проверяем payload подписки
				if query.InvoicePayload != "sub_payload_unique" {
					bot.logger.ZapLogger.Warn("Invalid invoice payload",
						zap.String("payload", query.InvoicePayload),
						zap.String("query_id", query.ID),
					)
					_, _ = bot.api.MakeRequest("answerPreCheckoutQuery", tgbotapi.Params{
						"pre_checkout_query_id": query.ID,
						"ok":                    "false",
						"error_message":         "Invalid payload",
					})
					continue
				}

				// Подтверждаем pre-checkout
				_, err := bot.api.MakeRequest("answerPreCheckoutQuery", tgbotapi.Params{
					"pre_checkout_query_id": query.ID,
					"ok":                    "true",
				})
				if err != nil {
					bot.logger.ZapLogger.Error("Failed to answer pre-checkout query",
						zap.Error(err),
						zap.String("query_id", query.ID),
					)
				} else {
					bot.logger.ZapLogger.Info("Pre-checkout query answered successfully",
						zap.String("query_id", query.ID),
					)
				}
				continue
			}

			// Обработка успешной оплаты
			// Передаем данные в роутер для сохранения в БД и отправки сообщения
			if update.Message != nil && update.Message.SuccessfulPayment != nil {
				payment := update.Message.SuccessfulPayment // объект успешного платежа из сообщения
				userID := update.Message.From.ID            // ID пользователя, совершившего платеж
				msgID := update.Message.MessageID           // ID Telegram-сообщения, связанного с платежом
				chargeID := payment.ProviderPaymentChargeID // Уникальный идентификатор чека (charge_id) от платежного провайдера

				// tgChargeID := payment.TelegramPaymentChargeID

				bot.logger.ZapLogger.Info("Received successful payment",
					zap.Int64("user_id", userID),
					zap.String("charge_id", chargeID),
					zap.String("currency", payment.Currency),
					zap.Int("total_amount", payment.TotalAmount),
				)

				// Передаем данные платежа в роутер через arguments
				paymentData := models.NewPaymentData(
					chargeID,               // уникальный идентификатор транзакции (charge_id)
					payment.Currency,       // валюта платежа, например "XTR"
					payment.InvoicePayload, // payload инвойса (строка, используемая для проверки типа покупки)
					payment.TotalAmount,    // общая сумма платежа в минимальных единицах валюты
				)

				// Отправляем команду в роутер для обработки платежа
				bot.router.AddComand(bot.ctx, "successful_payment", userID, msgID, paymentData)
				continue
			}

			if update.CallbackQuery != nil {
				data := update.CallbackQuery.Data
				userID := update.CallbackQuery.From.ID
				msgID := update.CallbackQuery.Message.MessageID
				//1 лог
				bot.logger.ZapLogger.Info("Received update", zap.Any("data", data), zap.Any("userID", userID))
				bot.router.AddComand(bot.ctx, data, userID, msgID, nil)
				bot.api.Request(tgbotapi.NewCallback(update.CallbackQuery.ID, ""))
			} else if update.Message != nil {
				text := update.Message.Text
				msg := update.Message
				userID := update.Message.From.ID
				msgID := msg.MessageID
				if msg.IsCommand() {
					//1 лог
					bot.logger.ZapLogger.Info("Received update", zap.Any("data", text), zap.Any("userID", userID))
					command := update.Message.Command()
					if command == "successful_payment" {
						//костыль что извне нельзя вызывать successful_payment как команду
						bot.sendMessage(userID, text_messages.TextUnknownCommand, nil)
						continue
					}
					bot.router.AddComand(bot.ctx, command, userID, msgID, nil)
				} else {
					//обычные сообщения также игнорируются
					bot.sendMessage(userID, text_messages.TextUnknownCommand, nil)
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
					"Failed to edit message",
					zap.Error(err),
					zap.Any("userID", editMsg.UserID),
					zap.Any("msgID", editMsg.MsgID),
				)
				continue
			}
			//последний лог
			bot.logger.ZapLogger.Info(
				"Message edited successfully",
				zap.Any("userID", editMsg.UserID),
				zap.Any("msgID", editMsg.MsgID),
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
					"Failed to delete message",
					zap.Error(err),
					zap.Any("userID", deleteMsg.UserID),
					zap.Any("msgID", deleteMsg.MsgID),
				)
				continue
			}
			//последний лог
			bot.logger.ZapLogger.Info(
				"Message deleted successfully",
				zap.Any("userID", deleteMsg.UserID),
				zap.Any("msgID", deleteMsg.MsgID),
			)

		}
	}
}

func (bot *Bot) sendBotCommand(ch chan models.BotCommand) {
	for {
		select {
		case <-bot.ctx.Done():
			return
		case cmd, ok := <-ch:
			if !ok {
				return
			}
			switch cmd.Type {
			case models.BotCommandSendSubscriptionInvoice:
				bot.logger.ZapLogger.Info(
					"Executing send subscription invoice command",
					zap.Any("userID", cmd.UserID),
				)
				bot.sendSubscriptionInvoice(cmd.UserID)
			default:
				bot.logger.ZapLogger.Warn(
					"Unknown bot command type",
					zap.String("type", string(cmd.Type)),
					zap.Any("userID", cmd.UserID),
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
			"Failed to send message",
			zap.Error(err),
			zap.Any("userID", userID),
		)
		return sentmsg, err
	}
	bot.logger.ZapLogger.Info(
		"Message sent successfully",
		zap.Any("userID", userID),
		zap.Any("msgID", sentmsg.MessageID),
	)

	return sentmsg, nil
}

func (bot *Bot) waitingMessageWithAnimation(ctx context.Context, sentMsg tgbotapi.Message, userID int64, inputText []string) {
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
				)
			} else {
				bot.logger.ZapLogger.Info(
					"Loading message deleted successfully",
					zap.Any("userID", userID),
					zap.Any("msgID", sentMsg.MessageID),
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
				)
			} else {
				bot.logger.ZapLogger.Info(
					"Loading message deleted successfully",
					zap.Any("userID", userID),
					zap.Any("msgID", sentMsg.MessageID),
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
				)
			}
			currentIdx = (currentIdx + 1) % len(inputText)
		}
	}
}

// где здесь время на сколько дней покупается? - оно в инвойсе не указывается, мы сами его задаем при добавлении в бд
// * Функция отправки инвойса с подпиской
func (bot *Bot) sendSubscriptionInvoice(userID int64) {
	// Формируем данные для инвойса

	// Название подписки (видно пользователю)
	name := text_messages.NameBasicSubscription
	// Описание подписки (видно пользователю)
	description := text_messages.DescriptionBasicSubsription
	// Payload, который вернётся боту после оплаты — можно использовать для идентификации типа покупки
	// обязательно надо добавить payload для подписки из уникального ключа, который будем знать только мы
	// TODO сделать отдельный слой с генерацией payload подписки
	payload := "sub_payload_unique"
	// providerToken — токен платежного провайдера. Для Stars (XTR) оставляем пустым
	provideToken := ""
	// startParameter — строка для deep-link, обычно пустая если не требуется стартовая ссылка
	startParameter := ""
	// название валюты. Для Telegram Stars нужно использовать "XTR"
	currency := "XTR"
	// массив цен (LabeledPrice), здесь одна строка с суммой подписки
	prices := []tgbotapi.LabeledPrice{
		{Label: description, Amount: bot.priceBasicSubscription}, // Сумма в Stars
	}

	// Формируем invoice для оплаты подписки через Stars/XTR Telegram
	invoice := tgbotapi.NewInvoice(
		userID,
		name,
		description,
		payload,
		provideToken,
		startParameter,
		currency,
		prices,
	)

	// Предлагаем чаевые при оплате подписки, но для XTR их нет, но строку надо оставить, иначе ошибка
	invoice.SuggestedTipAmounts = []int{}

	// Примечание: Если используется сторонний провайдер (providerToken), его указываем вместо "", если только Stars — оставлять пустым (поддерживается с tgbotapi v5.13+)
	// Рекуррентные параметры на уровне InvoiceConfig не поддерживаются. Повторные списания на стороне Stars/Telegram.
	msg, err := bot.api.Send(invoice)
	if err != nil {
		bot.logger.ZapLogger.Error("Error sending invoice", zap.Error(err), zap.Any("userID", userID))
	} else {
		bot.logger.ZapLogger.Info("Invoice sent", zap.Any("message", msg), zap.Any("userID", userID))
	}
}

func (bot *Bot) Stop() {
	bot.cancel()
	bot.wg.Wait()
	bot.router.CloseCommandChan()
	bot.logger.ZapLogger.Debug("Successful stop Telegram Bot")
}
