package ai

import (
	"bot_story_generator/internal/config"
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/models"
	"net/http"
	"os"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type AIConnection struct {
	client                *openai.Client
	model                 string
	main_game_rules_promt string
	create_hero_promt     string
	heroschema            openai.ResponseFormatJSONSchemaJSONSchemaParam
	segschema             openai.ResponseFormatJSONSchemaJSONSchemaParam
}

func NewAIConnection(cfg *config.Config, logger *logger.Logger, model string) (*AIConnection, error) {
	httpClient := &http.Client{Timeout: cfg.AI.ConnectTimeout}

	apiKey := cfg.AI.ApiKey
	if apiKey == "" {
		logger.ZapLogger.Debug("Using empty OpenAI API key for local LM Studio")
	}

	client := openai.NewClient(
		//перенес таймаут выполнения запроса в базовую настройку клиента ИИ
		option.WithRequestTimeout(cfg.AI.ChatCompletionTimeout),
		option.WithAPIKey(apiKey),
		option.WithHTTPClient(httpClient),
		option.WithMiddleware(func(req *http.Request, next option.MiddlewareNext) (*http.Response, error) {
			// Use OpenRouter endpoint instead of local LM Studio
			req.URL.Scheme = "https"
			req.URL.Host = "openrouter.ai"
			req.URL.Path = "/api/v1/chat/completions"
			return next(req)
		}),
	)
	fileData, err := os.ReadFile(cfg.AI.PathMainGameRules)
	if err != nil {
		return nil, err
	}
	mgpromt := string(fileData)
	fileData, err = os.ReadFile(cfg.AI.PathCreateHero)
	if err != nil {
		return nil, err
	}
	crhero := string(fileData)
	return &AIConnection{
		client:                &client,
		model:                 model,
		main_game_rules_promt: mgpromt,
		create_hero_promt:     crhero,
		heroschema: openai.ResponseFormatJSONSchemaJSONSchemaParam{
			Name:        cfg.AI.SchemaHeroes.Name,
			Description: openai.String(cfg.AI.SchemaHeroes.Description),
			Schema:      models.FantasyCharactersResponseSchema,
			Strict:      openai.Bool(true),
		},
		segschema: openai.ResponseFormatJSONSchemaJSONSchemaParam{
			Name:        cfg.AI.SchemaSegments.Name,
			Description: openai.String(cfg.AI.SchemaSegments.Description),
			Schema:      models.StoryScriptResponseSchema,
			Strict:      openai.Bool(true),
		},
	}, nil
}
