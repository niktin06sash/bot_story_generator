package config

import (
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Logger   LoggerConfig
	Telegram TelegramConfig
	AI       AIConfig
	Database DatabaseConfig
	Cache    CacheConfig
	Setting  ServerSetting
}
type LoggerConfig struct {
	LogPaths FilePaths
}
type FilePaths struct {
	Info  string
	Debug string
	Warn  string
	Error string
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
	PathMainGameRules     string
	PathCreateHero        string
}

type Schema struct {
	Name        string
	Description string
}

type DatabaseConfig struct {
	ConnectTimeout time.Duration
	URL            string
}

type ServerSetting struct {
	NumWorkers             int
	TokenDayLimit          int
	PremiumTokenDayLimit   int
	PriceBasicSubscription int
}

func NewConfig() (*Config, error) {
	err := godotenv.Load("cfg.env")
	if err != nil {
		return nil, err
	}
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
	aiPathMainGameRules := os.Getenv("AI_PATH_PROMT_MAIN_GAME_RULES")
	aiPathCreateHero := os.Getenv("AI_PATH_PROMT_CREATE_HERO")
	aiSchemaParamsNameH := os.Getenv("AI_SCHEMAPARAMS_NAME_HEROES")
	aiSchemaParamsDescriptionH := os.Getenv("AI_SCHEMAPARAMS_DESCRIPTION_HEROES")

	aiSchemaParamsNameSeg := os.Getenv("AI_SCHEMAPARAMS_NAME_STORYSEGMENT")
	aiSchemaParamsDescriptionSeg := os.Getenv("AI_SCHEMAPARAMS_DESCRIPTION_STORYSEGMENT")

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
	//че не написал какие значения в cfg.env
	tokenLimit := os.Getenv("TOKEN_DAY_LIMIT")
	numTokenLimit, err := strconv.Atoi(tokenLimit)
	if err != nil {
		return nil, err
	}

	// 10000
	premiumLimit := os.Getenv("PREMIUM_TOKEN_DAY_LIMIT")
	numPremiumLimit, err := strconv.Atoi(premiumLimit)
	if err != nil {
		return nil, err
	}

	priceBasicS := os.Getenv("PRICE_BASIC_SUBSCRIPTION")
	numPriceBasicS, err := strconv.Atoi(priceBasicS)
	if err != nil {
		return nil, err
	}

	cacheurl := os.Getenv("CACHE_URL")

	cachereguser := os.Getenv("CACHE_USER_CREATED_KEY")

	cacheexceededlimit := os.Getenv("CACHE_EXCEEDED_LIMIT_KEY")

	cacheConnectTimeout := os.Getenv("CACHE_CONNECT_TIMEOUT")
	cachecondur, err := time.ParseDuration(cacheConnectTimeout)
	if err != nil {
		return nil, err
	}
	/*cfg.env
	LOGGER_INFO_FILE_PATH = internal/logger/logs/info.log
	LOGGER_WARN_FILE_PATH = internal/logger/logs/warn.log
	LOGGER_ERROR_FILE_PATH = internal/logger/logs/error.log
	LOGGER_FATAL_FILE_PATH = internal/logger/logs/fatal.log
	LOGGER_DEBUG_FILE_PATH = internal/logger/logs/debug.log
	*/
	loggerInfoPath := os.Getenv("LOGGER_INFO_FILE_PATH")
	loggerWarnPath := os.Getenv("LOGGER_WARN_FILE_PATH")
	loggerErrorPath := os.Getenv("LOGGER_ERROR_FILE_PATH")
	loggerDebugPath := os.Getenv("LOGGER_DEBUG_FILE_PATH")

	cfg := &Config{
		Logger: LoggerConfig{
			LogPaths: FilePaths{
				Info:  loggerInfoPath,
				Warn:  loggerWarnPath,
				Error: loggerErrorPath,
				Debug: loggerDebugPath,
			},
		},
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
			SchemaHeroes:          Schema{Name: aiSchemaParamsNameH, Description: aiSchemaParamsDescriptionH},
			SchemaSegments:        Schema{Name: aiSchemaParamsNameSeg, Description: aiSchemaParamsDescriptionSeg},
			PathMainGameRules:     aiPathMainGameRules,
			PathCreateHero:        aiPathCreateHero,
		},
		Database: DatabaseConfig{
			ConnectTimeout: databasecondur,
			URL:            databaseConnectUrl,
		},
		Setting: ServerSetting{
			TokenDayLimit:          numTokenLimit,
			PremiumTokenDayLimit:   numPremiumLimit,
			PriceBasicSubscription: numPriceBasicS,
			NumWorkers:             num,
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
