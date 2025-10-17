package ai

import (
	"bot_story_generator/internal/logger"
	"context"

	"github.com/openai/openai-go/v3"
	"go.uber.org/zap"
)

type StoryAIImpl struct {
	conn   *AIConnection
	logger *logger.Logger
	model  string
}

func NewStoryAI(conn *AIConnection, model string, logger *logger.Logger) *StoryAIImpl {
	return &StoryAIImpl{
		conn:   conn,
		logger: logger,
		model:  model,
	}
}

func (ah *StoryAIImpl) GetChatCompletion(messageHistory string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ah.conn.timeout)
	defer cancel()

	params := openai.ChatCompletionNewParams{
		Model: openai.ChatModel(ah.model),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("You are a helpful assistant. Answer on Russian concisely."),
			openai.UserMessage(messageHistory),
		},
	}

	resp, err := ah.conn.client.Chat.Completions.New(ctx, params)
	if err != nil {
		ah.logger.ZapLogger.Error("Chat completion request failed", zap.Error(err))
		return "", err
	}
	if resp == nil || len(resp.Choices) == 0 {
		ah.logger.ZapLogger.Error("Empty response from chat completion")
		return "", nil
	}
	answer := resp.Choices[0].Message.Content
	return answer, nil
}
