package ai

import (
	"bot_story_generator/internal/models"

	"context"
	"encoding/json"

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

func (ah *StoryAIImpl) GetChatCompletion(parctx context.Context, messageHistory string) (string, error) {
	ctx, cancel := context.WithTimeout(parctx, ah.conn.timeout)
	defer cancel()

	params := openai.ChatCompletionNewParams{
		Model: openai.ChatModel(ah.conn.model),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(""),
			openai.UserMessage(messageHistory),
		},
	}

	resp, err := ah.conn.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return "", err
	}
	if resp == nil || len(resp.Choices) == 0 {
		return "", nil
	}
	answer := resp.Choices[0].Message.Content
	return answer, nil
}

func (ah *StoryAIImpl) GetStructuredHeroes(parctx context.Context, messageHistory string) (*models.FantasyCharacters, error) {
	ctx, cancel := context.WithTimeout(parctx, ah.conn.timeout)
	defer cancel()

	schemaParam := openai.ResponseFormatJSONSchemaJSONSchemaParam{
		Name:        "fantasy_characters",
		Description: openai.String("Массив фэнтезийных персонажей"),
		Schema:      models.FantasyCharactersResponseSchema,
		Strict:      openai.Bool(true),
	}

	params := openai.ChatCompletionNewParams{
		Model: openai.ChatModel(ah.conn.model),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(""),
			openai.UserMessage(messageHistory),
		},
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
				JSONSchema: schemaParam,
			},
		},
	}

	resp, err := ah.conn.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, err
	}
	if resp == nil || len(resp.Choices) == 0 {
		return nil, nil
	}

	var fantasyCharacters models.FantasyCharacters
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &fantasyCharacters); err != nil {
		return nil, err
	}

	return &fantasyCharacters, nil
}
