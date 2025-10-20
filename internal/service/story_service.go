package service

import (
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/models"

	"context"
	"os"

	"go.uber.org/zap"
)

type StoryDatabase interface {
	//методы для базы данных(пакет repository)
}
type StoryAI interface {
	GetChatCompletion(messageHistory string) (string, error)
	GetStructuredHeroes(messageHistory string) (*models.FantasyCharacters, error)
}

type StoryServiceImpl struct {
	DBStory StoryDatabase
	AIStory StoryAI
	logger  *logger.Logger
}

func NewStoryService(db StoryDatabase, ai StoryAI, logger *logger.Logger) *StoryServiceImpl {
	return &StoryServiceImpl{DBStory: db, AIStory: ai, logger: logger}
}

// CreateStructuredHeroes создает персонажей и возвращает типизированную структуру
func (s *StoryServiceImpl) CreateStructuredHeroes(ctx context.Context, chatID int64) (*models.FantasyCharacters, bool) {
	s.logger.ZapLogger.Info("Creating structured heroes", zap.Int64("chatID", chatID))

	// Читаем данные из файла create_hero.txt
	fileData, err := os.ReadFile("promts/create_hero.txt")

	if err != nil {
		s.logger.ZapLogger.Error("failed to read promt create_hero.txt", zap.Error(err))
		return nil, false
	}
	promt := string(fileData)

	fantasyCharacters, aiErr := s.AIStory.GetStructuredHeroes(promt)
	if aiErr != nil {
		s.logger.ZapLogger.Error("GetStructuredHeroes failed", zap.Error(aiErr))
		return nil, false
	}

	return fantasyCharacters, true
}
