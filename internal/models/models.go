package models

import (
	"bot_story_generator/internal/tracing"
	"context"

	"github.com/invopop/jsonschema"
)

func NewIncommingMessage(data string, userID int64, msgID int, agrs []Argument, trace tracing.Trace) IncommingMessage {
	return IncommingMessage{Data: data, UserID: userID, MsgID: msgID, Arguments: agrs, Trace: trace}
}

type Argument struct {
	NameSetting  string
	ValueSetting string
}

type IncommingMessage struct {
	Data      string
	UserID    int64
	MsgID     int
	Arguments []Argument
	Trace     tracing.Trace
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

func NewEditMessage(ctx context.Context, userID int64, msgID int, text string, buttonArgs ...ButtonArg) EditMessage {
	return EditMessage{UserID: userID, MsgID: msgID, ButtonArgs: buttonArgs, Text: text, Ctx: ctx}
}

type EditMessage struct {
	Ctx        context.Context
	UserID     int64
	MsgID      int
	Text       string
	ButtonArgs []ButtonArg
}

type DeleteMessage struct {
	Ctx    context.Context
	UserID int64
	MsgID  int
}

func NewDeleteMessage(ctx context.Context, userID int64, msgID int) DeleteMessage {
	return DeleteMessage{UserID: userID, MsgID: msgID, Ctx: ctx}
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
	Name       string   `json:"name" jsonschema_description:"Имя персонажа, подходящее для фэнтези-сеттинга (короткое/запоминающееся)" jsonschema:"minLength=2,maxLength=40"`
	Race       string   `json:"race" jsonschema_description:"Раса (например: человек, эльф, орк, драконорожденный и т.п.)" jsonschema:"minLength=1,maxLength=40"`
	Class      string   `json:"class" jsonschema_description:"Класс или профессия (маг, воин, охотник, некромант и т. д.)" jsonschema:"minLength=1,maxLength=50"`
	Appearance string   `json:"appearance" jsonschema_description:"Краткое описание внешности (2–3 предложения)" jsonschema:"minLength=30,maxLength=350"`
	Traits     []string `json:"traits" jsonschema_description:"Основные черты характера (2–3 пункта)" jsonschema:"minItems=2,maxItems=3"`
	Feature    string   `json:"feature" jsonschema_description:"Ключевая особенность или сила, делающая персонажа уникальным" jsonschema:"minLength=25,maxLength=250"`
	Biography  string   `json:"biography" jsonschema_description:"Короткий фрагмент биографии (2–5 предложений, атмосферно, без лишней воды)" jsonschema:"minLength=50,maxLength=500"`
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
	Narrative      string   `json:"narrative" jsonschema_description:"Дальнейшее развитие событий" jsonschema:"minLength=800,maxLength=1200"`
	ShortNarrative string   `json:"short_narrative" jsonschema_description:"Краткое содержание narrative: перечисли только ключевые события, мотивации и последствия. Ноль украшений, ноль связок, телеграфный формат. Максимум информации в минимуме слов." jsonschema:"minLength=15,maxLength=200"`
	Choices        []string `json:"choices" jsonschema_description:"Пять вариантов действия героя, реагирующих на повествование" jsonschema:"minItems=5,maxItems=5"`
	IsEnding       bool     `json:"is_ending" jsonschema_description:"Является ли данный узел концом истории (true — да, false — нет)"`
}

var StoryScriptResponseSchema = GenerateSchema[StoryNode]()

// Extension представляет продолжение сюжета
type Extension struct {
	Narrative string
}

// InvoiceMessage
type InvoiceMessage struct {
	Ctx          context.Context
	Subscription *Subscription
}

// NewInvoiceMessage создает invoice с указанным chatID
func NewInvoiceMessage(ctx context.Context, sub *Subscription) InvoiceMessage {
	return InvoiceMessage{
		Subscription: sub,
		Ctx:          ctx,
	}
}

// PaymentData представляет данные успешного платежа
type PaymentData struct {
	QueryID        string
	UserID         int64
	Currency       string
	InvoicePayload string
	TotalAmount    int
	ChargeID       string
	Error          error
	Trace          tracing.Trace
}

// NewPaymentData создает новые данные платежа
func NewPaymentData(queryID string, currency, invoicePayload string, totalAmount int, userid int64, chargeId string, trace tracing.Trace) *PaymentData {
	return &PaymentData{
		QueryID:        queryID,
		Currency:       currency,
		InvoicePayload: invoicePayload,
		TotalAmount:    totalAmount,
		UserID:         userid,
		ChargeID:       chargeId,
		Trace:          trace,
	}
}
