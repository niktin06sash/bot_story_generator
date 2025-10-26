package ai

import (
	"bot_story_generator/internal/config"
	"bot_story_generator/internal/logger"
	"net/http"
	"os"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"go.uber.org/zap"
)

type AIConnection struct {
	client                *openai.Client
	timeout               time.Duration
	model                 string
	main_game_rules_promt string
	create_hero_promt     string
}

func NewAIConnection(cfg *config.Config, logger *logger.Logger, model string) (*AIConnection, error) {
	logger.ZapLogger.Info("Initializing AIConnection...")

	httpClient := &http.Client{Timeout: cfg.AI.ConnectTimeout}

	apiKey := cfg.AI.ApiKey
	if apiKey == "" {
		logger.ZapLogger.Info("Using empty OpenAI API key for local LM Studio")
	}

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithHTTPClient(httpClient),
		option.WithMiddleware(func(req *http.Request, next option.MiddlewareNext) (*http.Response, error) {
			req.URL.Scheme = "http"
			req.URL.Host = "localhost:1234"
			return next(req)
		}),
	)
	fileData, err := os.ReadFile("promts/main_game_rules.txt")
	if err != nil {
		logger.ZapLogger.Error("failed to read promt main_game_rules.txt", zap.Error(err))
		return nil, err
	}
	mgpromt := string(fileData)
	fileData, err = os.ReadFile("promts/create_hero.txt")
	if err != nil {
		logger.ZapLogger.Error("failed to read promt main_game_rules.txt", zap.Error(err))
		return nil, err
	}
	crhero := string(fileData)
	logger.ZapLogger.Info("AIConnection successfully initialized")
	return &AIConnection{
		client:                &client,
		timeout:               cfg.AI.ChatCompletionTimeout,
		model:                 model,
		main_game_rules_promt: mgpromt,
		create_hero_promt:     crhero,
	}, nil
}
