package config

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
)

// Config содержит базовые настройки
type Config struct {
	TelegramBotToken string // Токен бота Telegram
	TelegramBotDebug bool   // Флаг отладки бота Telegram

	AIApiKey string // Ключ API OpenAI
	AIModel  string // Модель ИИ
}

// Load загружает конфигурацию
func Load() (*Config, error) {
	// Загружаем .env файл. Если файл не найден, приложение продолжит с системными переменными.
	_ = godotenv.Load("cfg.env") // ignore error — allow env from system too

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		return nil, errors.New("TELEGRAM_BOT_TOKEN environment variable is required")
	}

	botDebug := os.Getenv("TELEGRAM_BOT_DEBUG")
	telegramBotDebug := true
	if botDebug == "False" || botDebug == "false" || botDebug == "0" {
		telegramBotDebug = false
	}

	aIApiKey := os.Getenv("AI_API_KEY")
	// empty API key is acceptable (for local LM Studio)

	aIModel := os.Getenv("AI_MODEL")
	if aIModel == "" {
		return nil, errors.New("AI_MODEL environment variable is required")
	}

	cfg := &Config{
		TelegramBotToken: botToken,
		TelegramBotDebug: telegramBotDebug,
		AIApiKey:         aIApiKey,
		AIModel:          aIModel,
	}

	return cfg, nil
}
