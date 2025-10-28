package main

import (
	"bot_story_generator/internal/ai"
	"bot_story_generator/internal/config"
	"bot_story_generator/internal/database"
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/repository"
	"bot_story_generator/internal/router"
	"bot_story_generator/internal/service"
	tgbot "bot_story_generator/internal/tg_bot"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
)

func main() {
	logger, err := logger.NewLogger()
	if err != nil {
		logger.ZapLogger.Error("Failed to initialize logger",
			zap.Error(err),
		)
		return
	}
	logger.ZapLogger.Info("Successful init Logger")
	defer logger.Sync()

	cfg, err := config.NewConfig()
	if err != nil {
		logger.ZapLogger.Error("Failed to load config",
			zap.Error(err),
		)
		return
	}
	logger.ZapLogger.Info("Successful load config")

	//база данных(подключение + методы репозитория)
	pgx, err := database.NewDBObject(cfg.Database, logger)
	if err != nil {
		// Добавть обработку ошибки подключения к базе данных
	}
	defer pgx.Close()

	storyDatabase := repository.NewStoryDatabase(cfg, pgx)

	//ии(подключение + методы ии)
	aiConn, err := ai.NewAIConnection(cfg, logger, cfg.AI.Model)
	if err != nil {
		logger.ZapLogger.Error("Failed to connect to AI",
			zap.Error(err),
		)
		return
	}

	aiB := ai.NewStoryAI(aiConn)

	//бизнес-логика(база данных + ии)
	storyService := service.NewStoryService(storyDatabase, aiB, logger)

	//роутер
	router := router.NewRouter(cfg, storyService, logger)
	defer router.Stop()
	router.StartRouter()
	//бот
	bot, err := tgbot.NewBot(cfg, logger, router)
	if err != nil {
		logger.ZapLogger.Error("failed to initialize Telegram bot",
			zap.Error(err),
		)
		return
	}
	defer bot.Stop()
	bot.StartBot()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	sig := <-quit
	logger.ZapLogger.Info("Server shutting down with signal: %v", zap.Any("signal", sig))
}
