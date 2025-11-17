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
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
)

func main() {
	cfg, err := config.NewConfig()
	if err != nil {
		//☠☠☠ если конфига нет - смерть ☠☠☠
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
	txman := repository.NewTxManager(pgx)
	userdb := repository.NewUserDatabase(pgx)
	storydb := repository.NewStoryDatabase(pgx)
	vardb := repository.NewVariantDatabase(pgx)
	dldb := repository.NewDailyLimitDatabase(pgx)
	msgdb := repository.NewMessageDatabase(pgx)
	subdb := repository.NewSubscriptionDatabase(pgx)
	setdb := repository.NewSettingDatabase(pgx)
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
	usercache := repository.NewUserCache(c)
	setcache := repository.NewSettingCache(c)
	dlcache := repository.NewDailyLimitCache(c)
	//бизнес-логика(база данных + ии)
	storyService := service.NewService(cfg, txman, userdb, storydb, vardb, dldb, msgdb, subdb, setdb, aiB, dlcache, setcache, usercache, logger)

	// получение переменных настроек из базы и загрузка в кэш
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	settings, err := setdb.GetAllSettings(ctx)
	if err != nil {
		logger.ZapLogger.Debug("Failed to get settings from Database",
			zap.Error(err),
		)
		return
	}
	logger.ZapLogger.Debug("Successful get settings from Database")
	err = setcache.LoadCacheData(ctx, settings)
	if err != nil {
		logger.ZapLogger.Debug("Failed to load settings into Cache",
			zap.Error(err),
		)
		return
	}
	logger.ZapLogger.Debug("Successful load settings into Cache")
	//роутер
	router := router.NewRouter(cfg, storyService, logger)
	defer router.Stop()
	router.StartRouter()
	logger.ZapLogger.Debug("Successful start Router-Workers")
	//запуск бота
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
