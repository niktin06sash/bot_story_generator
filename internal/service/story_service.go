package service

import (
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/models"
	"bot_story_generator/internal/text_messages"
	"strconv"

	"context"
	"os"

	"go.uber.org/zap"
)

type StoryDatabase interface {
	//методы для базы данных(пакет repository)
}
type StoryAI interface {
	GetChatCompletion(ctx context.Context, messageHistory string) (string, error)
	GetStructuredHeroes(ctx context.Context, messageHistory string) (*models.FantasyCharacters, error)
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
func (s *StoryServiceImpl) CreateStructuredHeroes(ctx context.Context, chatID int64) (string, error) {
	s.logger.ZapLogger.Info("Creating structured heroes", zap.Int64("chatID", chatID))

	// Читаем данные из файла create_hero.txt
	fileData, err := os.ReadFile("promts/create_hero.txt")

	if err != nil {
		s.logger.ZapLogger.Error("failed to read promt create_hero.txt", zap.Error(err))
		return "", err
	}
	promt := string(fileData)

	fantasyCharacters, aiErr := s.AIStory.GetStructuredHeroes(ctx, promt)
	if aiErr != nil {
		s.logger.ZapLogger.Error("GetStructuredHeroes failed", zap.Error(aiErr))
		return "", err
	}
	resp := text_messages.TextChooseHero(fantasyCharacters)
	return resp, nil
}
func (s *StoryServiceImpl) UserChoice(ctx context.Context, chatID int64, data string) (string, error) {
	_, err := strconv.Atoi(data)
	if err != nil {
		s.logger.ZapLogger.Error("invalid user choice", zap.String("choice", data), zap.Error(err), zap.Int64("chat_id", chatID))
		return "", err
	}
	//TODO записывем выбор в бд

	//TODO генерим ответ ии

	//TODO записываем в бд повестование

	//TODO записываем в бд варианты выборов

	//TODO отправляем сообщение юзеру с вариантами ответа
	return "", nil
}
