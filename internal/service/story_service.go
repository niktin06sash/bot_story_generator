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
	AddUser(ctx context.Context, user models.User) error
	GetUser(ctx context.Context, chatID int64) (*models.User, error)
	GetAllStorySegments(ctx context.Context, chatID int64) (*models.AllStorySegments, error)
}
type StoryAI interface {
	GetChatCompletion(ctx context.Context, messageHistory string) (string, error)
	GetStructuredHeroes(ctx context.Context, messageHistory string) (*models.FantasyCharacters, error)
	GenerateNextStorySegment(parctx context.Context, messageHistory string) (*models.StoryNode, error)
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
	//TODO добавить проверку на токены и сделать что то с тем, 
	//TODO что если токенов нет будет, но юзер сделает выбор, то кнопки пропадут

	s.logger.ZapLogger.Info("User made a choice", zap.Int64("chatID", chatID), zap.String("choice", data))
	number_choise, err := strconv.Atoi(data)
	if err != nil {
		s.logger.ZapLogger.Error("invalid user choice", zap.String("choice", data), zap.Error(err), zap.Int64("chat_id", chatID))
		return "", err
	}
	_ = number_choise

	//TODO выводим выбор юзера

	//TODO записывем выбор в бд

	//TODO генерим ответ ии - вынести в другую функцию потом
	// Читаем данные из файла main_game_rules.txt
	fileData, err := os.ReadFile("promts/main_game_rules.txt")
	if err != nil {
		s.logger.ZapLogger.Error("failed to read promt create_hero.txt", zap.Error(err))
		return "", err
	}
	// Создаем промт
	mainPromt := string(fileData)
	allStory, dbErr := s.DBStory.GetAllStorySegments(ctx, chatID)
	if dbErr != nil {
		//TODO обработать
	}
	fullStory := ""
	for _, segment := range allStory.StorySegments {
		fullStory = "\n" + segment
	}
	promt := mainPromt + fullStory
	segment, aiErr := s.AIStory.GenerateNextStorySegment(ctx, promt)
	if aiErr != nil {
		//TODO обработать
	}
	narrative := segment.Narrative
	choise := segment.Choices

	//TODO записываем в бд повестование

	//TODO записываем в бд варианты выборов

	//TODO отправляем сообщение юзеру с вариантами ответа
	
	resp := text_messages.TextNarrativeWithChoices(narrative, choise)
	return resp, nil
}

func (s *StoryServiceImpl) CreateUser(ctx context.Context, chatID int64, isSub bool) error {
	s.logger.ZapLogger.Info("Creating user", zap.Int64("chatID", chatID), zap.Bool("isSub", isSub))

	user := models.NewUser(chatID, isSub)
	// Проверяем существует ли юзер перед добавлением
	existingUser, err := s.DBStory.GetUser(ctx, chatID)
	if err == nil && existingUser != nil {
		s.logger.ZapLogger.Info("User already exists, skipping creation", zap.Int64("chatID", chatID))
		return nil
	} else if err != nil {
		// Если ошибка не связана с отсутствием пользователя, логируем ошибку
		s.logger.ZapLogger.Error("Failed to check if user exists", zap.Error(err), zap.Int64("chatID", chatID))
		// В зависимости от логики репозитория, можно вернуть err или продолжить если это "not found"
	}
	err = s.DBStory.AddUser(ctx, user)
	if err != nil {
		s.logger.ZapLogger.Error("Failed to create user", zap.Error(err), zap.Int64("chatID", chatID))
		return err
	}

	s.logger.ZapLogger.Info("User created successfully", zap.Int64("chatID", chatID))
	return nil
}

func (s *StoryServiceImpl) GetUser(ctx context.Context, chatID int64) (*models.User, error) {
	s.logger.ZapLogger.Info("Getting user", zap.Int64("chatID", chatID))

	user, err := s.DBStory.GetUser(ctx, chatID)
	if err != nil {
		s.logger.ZapLogger.Error("Failed to get user", zap.Error(err), zap.Int64("chatID", chatID))
		return nil, err
	}

	s.logger.ZapLogger.Info("User retrieved successfully", zap.Int64("chatID", chatID))
	return user, nil
}
