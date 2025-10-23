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
	client  *openai.Client
	timeout time.Duration
	model   string
	promt   string
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
	fileData, err := os.ReadFile("promts/create_hero.txt")
	if err != nil {
		logger.ZapLogger.Error("failed to read promt create_hero.txt", zap.Error(err))
		return nil, err
	}
	promt := string(fileData)

	logger.ZapLogger.Info("AIConnection successfully initialized")
	return &AIConnection{
		client:  &client,
		timeout: cfg.AI.ChatCompletionTimeout,
		model:   model,
		promt:   promt,
	}, nil
}
