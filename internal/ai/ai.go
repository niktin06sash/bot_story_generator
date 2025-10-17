package ai

import (
	"bot_story_generator/internal/config"
	"context"
	"net/http"
	"time"

	"go.uber.org/zap"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type AIClient struct {
	client openai.Client
	logger *zap.Logger
	model  string
}

func NewAIClient(cfg *config.Config, logger *zap.Logger) *AIClient {
	logger.Info("initializing AIClient...")

	httpClient := &http.Client{Timeout: 60 * time.Second}

	apiKey := cfg.AIApiKey
	if apiKey == "" {
		logger.Info("using empty OpenAI API key for local LM Studio")
	}

	model := cfg.AIModel

	// Create OpenAI client
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithHTTPClient(httpClient),
		option.WithMiddleware(func(req *http.Request, next option.MiddlewareNext) (*http.Response, error) {
			req.URL.Scheme = "http" 		 // Use HTTP for local LM Studio
			req.URL.Host = "localhost:1234"  // LM Studio default host and port
			return next(req)
		}),
	)

	logger.Info("aiClient successfully initialized")
	return &AIClient{
		client: client,
		logger: logger,
		model:  model,
	}
}

func (ai *AIClient) GetChatCompletion(message_history string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	params := openai.ChatCompletionNewParams{
		Model: openai.ChatModel(ai.model),
		// Заглушка для примера
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("You are a helpful assistant. Answer on russian concisely."),
			openai.UserMessage(message_history),
		},
		// Messages: historySlice,
		// // Or
		// Messages: []openai.ChatCompletionMessageParamUnion{
		// 	openai.SystemMessage("You are a helpful assistant."), // system message
		// 	openai.UserMessage("Hello, how are you?"),            // user message
		// },
	}

	resp, err := ai.client.Chat.Completions.New(ctx, params)
	if err != nil {
		ai.logger.Error("chat completion request failed", zap.Error(err))
		return "", err
	}
	if resp == nil || len(resp.Choices) == 0 {
		ai.logger.Error("empty response from chat completion")
		return "", nil
	}
	answer := resp.Choices[0].Message.Content
	return answer, nil
}
