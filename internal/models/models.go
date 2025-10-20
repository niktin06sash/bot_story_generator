package models

import (
	"github.com/invopop/jsonschema"
)

type Message struct {
	Command   string
	Arguments []string
	ChatID    int64
}

type OutboundMessage struct {
	ChatID int64
	Text   string
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
	Name       string   `json:"name" jsonschema_description:"Имя персонажа, подходящее для фэнтези-сеттинга (короткое/запоминающееся)" jsonschema:"minLength=1,maxLength=60"`
	Race       string   `json:"race" jsonschema_description:"Раса (например: человек, эльф, орк, драконорожденный и т.п.)" jsonschema:"minLength=1,maxLength=40"`
	Class      string   `json:"class" jsonschema_description:"Класс или профессия (маг, воин, охотник, некромант и т. д.)" jsonschema:"minLength=1,maxLength=60"`
	Appearance string   `json:"appearance" jsonschema_description:"Краткое описание внешности (2–3 предложения)" jsonschema:"minLength=20,maxLength=300"`
	Traits     []string `json:"traits" jsonschema_description:"Основные черты характера (2–3 пункта)" jsonschema:"minItems=2,maxItems=3"`
	Feature    string   `json:"feature" jsonschema_description:"Ключевая особенность или сила, делающая персонажа уникальным" jsonschema:"minLength=10,maxLength=200"`
	Biography  string   `json:"biography" jsonschema_description:"Короткий фрагмент биографии (2–5 предложений, атмосферно, без лишней воды)" jsonschema:"minLength=20,maxLength=500"`
	Tone       string   `json:"tone,omitempty" jsonschema:"enum=epic,enum=dark,enum=neutral,enum=tragic,enum=mysterious" jsonschema_description:"Необязательное поле — тон/стиль описания (можно использовать для фильтрации)"`
}

// FantasyCharacters представляет массив персонажей
type FantasyCharacters struct {
	Characters []Hero `json:"characters" jsonschema_description:"Массив фэнтезийных персонажей" jsonschema:"minItems=5,maxItems=5"`
}

// Генерируем JSON схему во время инициализации
var FantasyCharactersResponseSchema = GenerateSchema[FantasyCharacters]()
