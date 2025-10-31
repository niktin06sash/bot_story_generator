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

func (ah *StoryAIImpl) GetStructuredHeroes(parctx context.Context) (*models.FantasyCharacters, error) {
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
			openai.SystemMessage(ah.conn.main_game_rules_promt),
			openai.UserMessage(ah.conn.create_hero_promt),
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

func (ah *StoryAIImpl) GenerateNextStorySegment(parctx context.Context, currentData string) (*models.StoryNode, error) {
	ctx, cancel := context.WithTimeout(parctx, ah.conn.timeout)
	defer cancel()

	schemaParam := openai.ResponseFormatJSONSchemaJSONSchemaParam{
		Name:        "story_segment",
		Description: openai.String("Повествование + варианты ответов"),
		Schema:      models.StoryScriptResponseSchema,
		Strict:      openai.Bool(true),
	}

	params := openai.ChatCompletionNewParams{
		Model: openai.ChatModel(ah.conn.model),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(ah.conn.main_game_rules_promt),
			openai.UserMessage(currentData),
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

	var StoryNode models.StoryNode
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &StoryNode); err != nil {
		return nil, err
	}

	return &StoryNode, nil

}
