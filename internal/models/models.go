package models

import (
	"context"

	"github.com/invopop/jsonschema"
)

func NewIncommingMessage(data string, userID int64, msgID int, arguments interface{}) IncommingMessage {
	return IncommingMessage{Data: data, UserID: userID, MsgID: msgID, Arguments: arguments}
}

type IncommingMessage struct {
	Data      string
	UserID    int64
	MsgID     int
	Arguments interface{}
}

func NewOutboundMessage(ctx context.Context, userId int64, text string, buttonArgs ...ButtonArg) OutboundMessage {
	return OutboundMessage{Ctx: ctx, UserID: userId, Text: text, ButtonArgs: buttonArgs}
}

type OutboundMessage struct {
	Ctx        context.Context
	UserID     int64
	Text       string
	ButtonArgs []ButtonArg
}

type ButtonArg struct {
	ButtonName string
	Args       []string
}

func NewButtonArg(btn string, args []string) ButtonArg {
	return ButtonArg{ButtonName: btn, Args: args}
}

func NewEditMessage(userID int64, msgID int, text string, buttonArgs ...ButtonArg) EditMessage {
	return EditMessage{UserID: userID, MsgID: msgID, ButtonArgs: buttonArgs, Text: text}
}

type EditMessage struct {
	UserID     int64
	MsgID      int
	Text       string
	ButtonArgs []ButtonArg
}

type DeleteMessage struct {
	UserID int64
	MsgID  int
}

func NewDeleteMessage(userID int64, msgID int) DeleteMessage {
	return DeleteMessage{UserID: userID, MsgID: msgID}
}

// GenerateSchema генерирует JSON схему для типа T
func GenerateSchema[T any]() interface{} {
	// Structured Outputs использует подмножество JSON schema
	// Эти флаги необходимы для соответствия подмножеству
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	var v T
	schema := reflector.Reflect(v)
	return schema
}

// Hero представляет фэнтезийного персонажа
type Hero struct {
	Name       string   `json:"name" jsonschema_description:"Имя персонажа, подходящее для фэнтези-сеттинга (короткое/запоминающееся)" jsonschema:"minLength=1,maxLength=40"`
	Race       string   `json:"race" jsonschema_description:"Раса (например: человек, эльф, орк, драконорожденный и т.п.)" jsonschema:"minLength=1,maxLength=50"`
	Class      string   `json:"class" jsonschema_description:"Класс или профессия (маг, воин, охотник, некромант и т. д.)" jsonschema:"minLength=1,maxLength=50"`
	Appearance string   `json:"appearance" jsonschema_description:"Краткое описание внешности (2–3 предложения)" jsonschema:"minLength=20,maxLength=250"`
	Traits     []string `json:"traits" jsonschema_description:"Основные черты характера (2–3 пункта)" jsonschema:"minItems=2,maxItems=3"`
	Feature    string   `json:"feature" jsonschema_description:"Ключевая особенность или сила, делающая персонажа уникальным" jsonschema:"minLength=15,maxLength=250"`
	Biography  string   `json:"biography" jsonschema_description:"Короткий фрагмент биографии (2–5 предложений, атмосферно, без лишней воды)" jsonschema:"minLength=20,maxLength=500"`
}

// FantasyCharacters представляет массив персонажей
type FantasyCharacters struct {
	Characters []Hero `json:"characters" jsonschema_description:"Массив фэнтезийных персонажей" jsonschema:"minItems=5,maxItems=5"`
}

// Генерируем JSON схему во время инициализации
var FantasyCharactersResponseSchema = GenerateSchema[FantasyCharacters]()

// Для проверки выбора ответа для продолжении истории
var PossibleAnswersToStory = map[string]struct{}{
	"1": {}, "2": {}, "3": {}, "4": {}, "5": {},
}

// StoryNode представляет один элемент повествования с вариантами реакции
type StoryNode struct {
	Narrative string   `json:"narrative" jsonschema_description:"Дальнейшее развитие событий" jsonschema:"minLength=1000,maxLength=2200"`
	Choices   []string `json:"choices" jsonschema_description:"Пять вариантов действия героя, реагирующих на повествование" jsonschema:"minItems=5,maxItems=5"`
}

var StoryScriptResponseSchema = GenerateSchema[StoryNode]()

// Extension представляет продолжение сюжета
type Extension struct {
	Narrative string
}

// TODO мб потом передеать
const (
	BotCommandSendSubscriptionInvoice = "send_subscription_invoice"
)

// BotCommand представляет команду для выполнения ботом
type BotCommand struct {
	Type     string
	UserID   int64
	ChargeID string // Для cancelSubscription
}

// NewBotCommand создает команду с указанным chatID
func NewBotCommand(cmdType string, userID int64, chargeID string) BotCommand {
	return BotCommand{
		Type:     cmdType,
		UserID:   userID,
		ChargeID: chargeID,
	}
}

// PaymentData представляет данные успешного платежа
type PaymentData struct {
	ChargeID       string
	Currency       string
	InvoicePayload string
	TotalAmount    int
}

// NewPaymentData создает новые данные платежа
func NewPaymentData(chargeID, currency, invoicePayload string, totalAmount int) *PaymentData {
	return &PaymentData{
		ChargeID:       chargeID,
		Currency:       currency,
		InvoicePayload: invoicePayload,
		TotalAmount:    totalAmount,
	}
}
