package main

import (
	"bot_story_generator/internal/ai"
	"bot_story_generator/internal/cache"
	"bot_story_generator/internal/config"
	"bot_story_generator/internal/database"
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/repository"
	"bot_story_generator/internal/router"
	"bot_story_generator/internal/service"
	tgbot "bot_story_generator/internal/tg_bot"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
)

func main() {
	cfg, err := config.NewConfig()
	if err != nil {
		//если конфига нет - смерть
		panic(fmt.Sprintf("Failed to load config: %v", err))
	}
	logger, err := logger.NewLogger(cfg)
	if err != nil {
		logger.ZapLogger.Debug("Failed to initialize logger",
			zap.Error(err),
		)
		return
	}
	logger.ZapLogger.Debug("Successful init Logger")
	defer logger.Sync()
	//база данных(подключение + методы репозитория)
	pgx, err := database.NewDBObject(cfg, logger)
	if err != nil {
		logger.ZapLogger.Debug("Failed to connect to Database",
			zap.Error(err),
		)
		return
	}
	logger.ZapLogger.Debug("Successful Database-connect")
	defer pgx.Close()
	storyDatabase := repository.NewStoryDatabase(cfg, pgx)

	//ии(подключение + методы ии)
	aiConn, err := ai.NewAIConnection(cfg, logger, cfg.AI.Model)
	if err != nil {
		logger.ZapLogger.Debug("Failed to connect to AI",
			zap.Error(err),
		)
		return
	}
	logger.ZapLogger.Debug("Successful AI-connect")
	aiB := ai.NewStoryAI(aiConn)

	//кэширование(подключение + методы кэширования)
	c, err := cache.NewCacheConnection(cfg, logger)
	if err != nil {
		logger.ZapLogger.Debug("Failed to connect to Cache",
			zap.Error(err),
		)
		return
	}
	logger.ZapLogger.Debug("Successful Cache-connect")
	defer c.Close()
	storyCache := repository.NewStoryCache(c)
	//бизнес-логика(база данных + ии)
	storyService := service.NewStoryService(storyDatabase, aiB, storyCache, logger)

	//роутер
	router := router.NewRouter(cfg, storyService, logger)
	defer router.Stop()
	router.StartRouter()
	logger.ZapLogger.Debug("Successful start Router-Workers")
	bot, err := tgbot.NewBot(cfg, logger, router)
	if err != nil {
		logger.ZapLogger.Debug("Failed to initialize Telegram bot",
			zap.Error(err),
		)
		return
	}
	logger.ZapLogger.Debug("Successful Telegram Bot-connect")
	defer bot.Stop()
	bot.StartBot()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	sig := <-quit
	logger.ZapLogger.Debug("Server shutting down with signal: %v", zap.Any("signal", sig))
}
