package main

import (
	"bot_story_generator/internal/ai"
	"bot_story_generator/internal/config"
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/tg_bot"
	"fmt"

	"go.uber.org/zap"
)

func main() {
	// Create logger for development
	logger, err := logger.New()
	if err != nil {
		panic("failed to initialize logger " + err.Error())
	}
	logger.Info("logger initialized successfully")
	defer logger.Sync()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load config",
			zap.Error(err),
		)
	}
	logger.Info("configuration loaded successfully")

	// Initialize AI client
	aiClient := ai.NewAIClient(cfg, logger)

	// Initialize and start Telegram bot
	bot, err := tgbot.NewBot(cfg, logger)
	if err != nil {
		logger.Fatal("failed to initialize Telegram bot",
			zap.Error(err),
		)
	}
	go bot.Start()

	// Example usage of AI client
	answer, err := aiClient.GetChatCompletion("Hello, how are you?")
	if err != nil {
		logger.Error("failed to get chat completion from AI", zap.Error(err),)
	}
	fmt.Println("AI answer:", answer)
}