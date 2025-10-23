package models

import (
	"context"

	"github.com/invopop/jsonschema"
)

func NewIncommingMessage(data string, chatId int64, userID int64) IncommingMessage {
	return IncommingMessage{Data: data, ChatID: chatId, UserID: userID}
}

type IncommingMessage struct {
	Data   string
	ChatID int64
	UserID int64
}

func NewOutboundMessage(ctx context.Context, chatId int64, text string, buttonArgs ...ButtonArg) OutboundMessage {
	return OutboundMessage{Ctx: ctx, ChatID: chatId, Text: text, ButtonArgs: buttonArgs}
}

type OutboundMessage struct {
	Ctx        context.Context
	ChatID     int64
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
	Feature    string   `json:"feature" jsonschema_description:"Ключевая особенность или сила, делающая персонажа уникальным" jsonschema:"minLength=10,maxLength=200"`
	Biography  string   `json:"biography" jsonschema_description:"Короткий фрагмент биографии (2–5 предложений, атмосферно, без лишней воды)" jsonschema:"minLength=20,maxLength=450"`
	Tone       string   `json:"tone,omitempty" jsonschema:"enum=epic,enum=dark,enum=neutral,enum=tragic,enum=mysterious" jsonschema_description:"Необязательное поле — тон/стиль описания (можно использовать для фильтрации)"`
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
	Narrative string   `json:"narrative" jsonschema_description:"Дальнейшее развитие событий" jsonschema:"minLength=500,maxLength=2200"`
	Choices   []string `json:"choices" jsonschema_description:"Пять вариантов действия героя, реагирующих на повествование" jsonschema:"minItems=5,maxItems=5"`
}

// Генерируем JSON схему во время инициализации
var StoryScriptResponseSchema = GenerateSchema[StoryNode]()

// Story хранит все сегменты истории для текущей игровой сессии
type AllStorySegments struct {
	StorySegments []string
}
