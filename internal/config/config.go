package config

import (
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Telegram TelegramConfig
	AI       AIConfig
	Database DatabaseConfig
}
type TelegramConfig struct {
	BotToken string
	BotDebug bool
	Offset   int
	Timeout  int
}
type AIConfig struct {
	ConnectTimeout        time.Duration //"30s"
	ChatCompletionTimeout time.Duration
	ApiKey                string
	Model                 string
}
type DatabaseConfig struct {
	ConnectTimeout time.Duration
	URL            string
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
	databaseConnectTimeout := os.Getenv("DATABASE_CONNECT_TIMEOUT")
	databasecondur, err := time.ParseDuration(databaseConnectTimeout)
	if err != nil {
		return nil, err
	}
	databaseConnectUrl := os.Getenv("DATABASE_CONNECT_URL")
	cfg := &Config{
		Telegram: TelegramConfig{
			BotToken: botToken,
			BotDebug: telegramBotDebug,
			Offset:   bo,
			Timeout:  to,
		}, AI: AIConfig{
			ApiKey:                aIApiKey,
			Model:                 aIModel,
			ConnectTimeout:        aicondur,
			ChatCompletionTimeout: aicomdur,
		}, Database: DatabaseConfig{
			ConnectTimeout: databasecondur,
			URL:            databaseConnectUrl,
		},
	}

	return cfg, nil
}
