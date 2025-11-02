package config

import (
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Telegram   TelegramConfig
	AI         AIConfig
	Database   DatabaseConfig
	Cache      CacheConfig
	NumWorkers int
	Setting    BotSetting
}
type CacheConfig struct {
	ConnectTimeout   time.Duration
	URL              string
	UserCreatedKey   string
	ExceededLimitKey string
}
type TelegramConfig struct {
	BotToken string
	BotDebug bool
	Offset   int
	Timeout  int
}

type AIConfig struct {
	ConnectTimeout        time.Duration
	ChatCompletionTimeout time.Duration
	ApiKey                string
	Model                 string
	SchemaHeroes          Schema
	SchemaSegments        Schema
}
type Schema struct {
	Name        string
	Description string
	Strict      bool
}
type DatabaseConfig struct {
	ConnectTimeout time.Duration
	URL            string
}

type BotSetting struct {
	TokenDayLimit int
}

func NewConfig() (*Config, error) {
	_ = godotenv.Load("cfg.env")

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		return nil, errors.New("TELEGRAM_BOT_TOKEN environment variable is required")
	}

	botDebug := os.Getenv("TELEGRAM_BOT_DEBUG")
	telegramBotDebug := true
	if botDebug == "False" || botDebug == "false" || botDebug == "0" {
		telegramBotDebug = false
	}
	botOffset := os.Getenv("TELEGRAM_BOT_OFFSET")
	bo, err := strconv.Atoi(botOffset)
	if err != nil {
		return nil, err
	}
	botTimeOut := os.Getenv("TELEGRAM_BOT_TIMEOUT")
	to, err := strconv.Atoi(botTimeOut)
	if err != nil {
		return nil, err
	}
	aIApiKey := os.Getenv("AI_API_KEY")
	aIModel := os.Getenv("AI_MODEL")
	if aIModel == "" {
		return nil, errors.New("AI_MODEL environment variable is required")
	}
	aiConnectTimeout := os.Getenv("AI_CONNECT_TIMEOUT")
	aicondur, err := time.ParseDuration(aiConnectTimeout)
	if err != nil {
		return nil, err
	}
	aiCompletionTimeout := os.Getenv("AI_COMPLETION_TIMEOUT")
	aicomdur, err := time.ParseDuration(aiCompletionTimeout)
	if err != nil {
		return nil, err
	}
	aiSchemaParamsNameH := os.Getenv("AI_SCHEMAPARAMS_NAME_HEROES")
	aiSchemaParamsDescriptionH := os.Getenv("AI_SCHEMAPARAMS_DESCRIPTION_HEROES")
	aiSchemaParamsStrictH := os.Getenv("AI_SCHEMAPARAMS_STRICT_HEROES")
	boolaiSchemaParamsStrictH, err := strconv.ParseBool(aiSchemaParamsStrictH)
	if err != nil {
		return nil, err
	}
	aiSchemaParamsNameSeg := os.Getenv("AI_SCHEMAPARAMS_NAME_STORYSEGMENT")
	aiSchemaParamsDescriptionSeg := os.Getenv("AI_SCHEMAPARAMS_DESCRIPTION_STORYSEGMENT")
	aiSchemaParamsStrictSeg := os.Getenv("AI_SCHEMAPARAMS_STRICT_STORYSEGMENT")
	boolaiSchemaParamsStrictSeg, err := strconv.ParseBool(aiSchemaParamsStrictSeg)
	if err != nil {
		return nil, err
	}
	//удали как перенесешь в cfg.env
	//AI_SCHEMAPARAMS_NAME_HEROES = fantasy_characters
	//AI_SCHEMAPARAMS_DESCRIPTION_HEROES = Массив фэнтезийных персонажей
	//AI_SCHEMAPARAMS_STRICT_HEROES = true
	//AI_SCHEMAPARAMS_NAME_STORYSEGMENT = story_segment
	//AI_SCHEMAPARAMS_DESCRIPTION_STORYSEGMENT = Повествование + варианты ответов
	//AI_SCHEMAPARAMS_STRICT_STORYSEGMENT = true
	databaseConnectTimeout := os.Getenv("DATABASE_CONNECT_TIMEOUT")
	databasecondur, err := time.ParseDuration(databaseConnectTimeout)
	if err != nil {
		return nil, err
	}
	databaseConnectUrl := os.Getenv("DATABASE_CONNECT_URL")
	numworkers := os.Getenv("NUM_WORKERS")
	num, err := strconv.Atoi(numworkers)
	if err != nil {
		return nil, err
	}
	tokenLimit := os.Getenv("TOKEN_DAY_LIMIT")
	numTokenLimit, err := strconv.Atoi(tokenLimit)
	if err != nil {
		return nil, err
	}
	cacheurl := os.Getenv("CACHE_URL")
	//user_created:%d
	cachereguser := os.Getenv("CACHE_USER_CREATED_KEY")
	//limit_exceeded:%d
	cacheexceededlimit := os.Getenv("CACHE_EXCEEDED_LIMIT_KEY")
	//30s
	cacheConnectTimeout := os.Getenv("CACHE_CONNECT_TIMEOUT")
	cachecondur, err := time.ParseDuration(cacheConnectTimeout)
	if err != nil {
		return nil, err
	}
	cfg := &Config{
		Telegram: TelegramConfig{
			BotToken: botToken,
			BotDebug: telegramBotDebug,
			Offset:   bo,
			Timeout:  to,
		},
		AI: AIConfig{
			ApiKey:                aIApiKey,
			Model:                 aIModel,
			ConnectTimeout:        aicondur,
			ChatCompletionTimeout: aicomdur,
			SchemaHeroes:          Schema{Name: aiSchemaParamsNameH, Description: aiSchemaParamsDescriptionH, Strict: boolaiSchemaParamsStrictH},
			SchemaSegments:        Schema{Name: aiSchemaParamsNameSeg, Description: aiSchemaParamsDescriptionSeg, Strict: boolaiSchemaParamsStrictSeg},
		},
		Database: DatabaseConfig{
			ConnectTimeout: databasecondur,
			URL:            databaseConnectUrl,
		},
		NumWorkers: num,
		Setting: BotSetting{
			TokenDayLimit: numTokenLimit,
		},
		Cache: CacheConfig{
			ConnectTimeout:   cachecondur,
			URL:              cacheurl,
			UserCreatedKey:   cachereguser,
			ExceededLimitKey: cacheexceededlimit,
		},
	}

	return cfg, nil
}
