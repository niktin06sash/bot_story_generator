package ai

import (
	"context"

	"github.com/openai/openai-go/v3"
)

type StoryAIImpl struct {
	conn *AIConnection
}

func NewStoryAI(conn *AIConnection) *StoryAIImpl {
	return &StoryAIImpl{
		conn: conn,
	}
}

// remove logging from methods and move them to the service layer
func (ah *StoryAIImpl) GetChatCompletion(messageHistory string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ah.conn.timeout)
	defer cancel()

	params := openai.ChatCompletionNewParams{
		Model: openai.ChatModel(ah.conn.model),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("You are a helpful assistant. Answer on Russian concisely."),
			openai.UserMessage(messageHistory),
		},
	}

	resp, err := ah.conn.client.Chat.Completions.New(ctx, params)
	if err != nil {
		//ah.conn.logger.ZapLogger.Error("Chat completion request failed", zap.Error(err))
		return "", err
	}
	if resp == nil || len(resp.Choices) == 0 {
		//ah.conn.logger.ZapLogger.Error("Empty response from chat completion")
		return "", nil
	}
	answer := resp.Choices[0].Message.Content
	return answer, nil
}
