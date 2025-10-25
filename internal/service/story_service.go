package service

import (
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/models"
	"bot_story_generator/internal/text_messages"
	"encoding/json"
	"strconv"
	"strings"

	"context"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

//go:generate mockgen -source=story_service.go -destination=mocks/mock.go
type StoryDatabase interface {
	//транзакции для изменения данных в нескольких таблицах за одно действие в сервисе(можно будет в будущем вынести в отдельный интерфейс)
	BeginTx(ctx context.Context) (pgx.Tx, error)
	RollbackTx(ctx context.Context, tx pgx.Tx) error
	CommitTx(ctx context.Context, tx pgx.Tx) error

	AddUser(ctx context.Context, user *models.User) error
	CheckActiveStories(ctx context.Context, userID int64) error
	AddStory(ctx context.Context, tx pgx.Tx, story *models.Story) (int, error)
	AddVariant(ctx context.Context, tx pgx.Tx, variant *models.StoryVariant) error
	GetVariants(ctx context.Context, storyID int) (*models.StoryVariant, error)
	GetAllStorySegments(ctx context.Context, chatID int64) (*models.AllStorySegments, error)
}
type StoryAI interface {
	GetChatCompletion(ctx context.Context) (string, error)
	GetStructuredHeroes(ctx context.Context) (*models.FantasyCharacters, error)
	GenerateNextStorySegment(parctx context.Context, storyData string) (*models.StoryNode, error)
}

type StoryServiceImpl struct {
	DBStory StoryDatabase
	AIStory StoryAI
	Logger  *logger.Logger
}

func NewStoryService(db StoryDatabase, ai StoryAI, logger *logger.Logger) *StoryServiceImpl {
	return &StoryServiceImpl{DBStory: db, AIStory: ai, Logger: logger}
}

func (s *StoryServiceImpl) CreateStory(ctx context.Context, chatID int64, userID int64) ([]string, error) {
	s.Logger.ZapLogger.Info("Creating new story", zap.Any("chatID", chatID), zap.Any("userID", userID))
	// Проверяем, нет ли активных историй у пользователя в данный момент
	err := s.DBStory.CheckActiveStories(ctx, userID)
	if err != nil {
		if strings.HasPrefix(err.Error(), "client: ") {
			s.Logger.ZapLogger.Warn("Client error", zap.Error(err), zap.Any("chatID", chatID), zap.Any("userID", userID))
			return []string{text_messages.TextErrorUserActiveStory}, err
		}
		s.Logger.ZapLogger.Error("Server error", zap.Error(err), zap.Any("chatID", chatID), zap.Any("userID", userID))
		return []string{text_messages.TextErrorCreateTask}, err
	}
	// Запрос в ИИ
	fantasyCharacters, err := s.AIStory.GetStructuredHeroes(ctx)
	if err != nil {
		s.Logger.ZapLogger.Error("GetStructuredHeroes failed", zap.Error(err), zap.Any("chatID", chatID), zap.Any("userID", userID))
		return []string{text_messages.TextErrorCreateTask}, err
	}
	data, err := json.Marshal(fantasyCharacters)
	if err != nil {
		s.Logger.ZapLogger.Error("Marshal failed", zap.Error(err), zap.Any("chatID", chatID), zap.Any("userID", userID))
		return []string{text_messages.TextErrorCreateTask}, err
	}
	// Создание транзакции для консистентности данных
	tx, err := s.DBStory.BeginTx(ctx)
	if err != nil {
		s.Logger.ZapLogger.Error("BeginTx failed", zap.Error(err), zap.Any("chatID", chatID), zap.Any("userID", userID))
		return []string{text_messages.TextErrorCreateTask}, err
	}
	story := models.NewStory(userID, nil)
	// Создаем историю с пустыми данными(так как ждем выбор в следующем действии пользователя)
	storyId, err := s.DBStory.AddStory(ctx, tx, story)
	if err != nil {
		s.Logger.ZapLogger.Error("AddStory failed", zap.Error(err), zap.Any("chatID", chatID), zap.Any("userID", userID))
		rollbackErr := s.DBStory.RollbackTx(ctx, tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx failed", zap.Error(rollbackErr), zap.Any("chatID", chatID), zap.Any("userID", userID))
		}
		return []string{text_messages.TextErrorCreateTask}, err
	}
	variant := models.NewStoryVariant(storyId, data)
	// Создаем начальный вариант с данными из ИИ
	err = s.DBStory.AddVariant(ctx, tx, variant)
	if err != nil {
		s.Logger.ZapLogger.Error("AddVariant failed", zap.Error(err), zap.Any("chatID", chatID), zap.Any("userID", userID))
		rollbackErr := s.DBStory.RollbackTx(ctx, tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx failed", zap.Error(rollbackErr), zap.Any("chatID", chatID), zap.Any("userID", userID))
		}
		return []string{text_messages.TextErrorCreateTask}, err
	}
	// Делаем подтверждение транзакции после изменения таблиц(+запись в истории, варианты)
	err = s.DBStory.CommitTx(ctx, tx)
	if err != nil {
		s.Logger.ZapLogger.Error("CommitTx failed", zap.Error(err), zap.Any("chatID", chatID), zap.Any("userID", userID))
		return []string{text_messages.TextErrorCreateTask}, err
	}
	s.Logger.ZapLogger.Info("Story created successfully", zap.Any("chatID", chatID), zap.Any("userID", userID))
	return text_messages.NewChouseHero(fantasyCharacters), nil
}

func (s *StoryServiceImpl) UserChoice(ctx context.Context, chatID int64, data string) (string, string, error) {
	//TODO добавить проверку на токены и сделать что то с тем,
	//TODO что если токенов нет будет, но юзер сделает выбор, то кнопки пропадут
	//TODO можно убирать кнопки только после успешного исполнения задачи
	//TODO если даже на кнопку нажали повторно, то мьютекс заблочит задачу из первого нажатия и будет скипать последующие
	//TODO будто бы контекстом с ключом так же сообщить боту, но все упирается в айди сообщения телеграм
	s.Logger.ZapLogger.Info("User made a choice", zap.Int64("chatID", chatID), zap.String("choice", data))
	number_choise, err := strconv.Atoi(data)
	if err != nil {
		s.Logger.ZapLogger.Error("invalid user choice", zap.String("choice", data), zap.Error(err), zap.Int64("chat_id", chatID))
		return "", "", err
	}

	//* Получаем варианты выбора пользователя
	variant, dbErr := s.DBStory.GetVariants(ctx, int(chatID))
	if dbErr != nil {
		s.Logger.ZapLogger.Error("GetVariants failed", zap.Error(dbErr), zap.Int64("chatID", chatID))
		return "", "", dbErr
	}
	//TODO определить, что получаем - fantasyCharactres или storyVariants
	var fantasyCharacters models.FantasyCharacters
	err = json.Unmarshal(variant.Data, &fantasyCharacters)
	if err != nil {
		s.Logger.ZapLogger.Error("Failed to unmarshal fantasy characters", zap.Error(err), zap.Int64("chatID", chatID))
		return "", "", err
	}
	userVariant := fantasyCharacters.Characters[number_choise]
	s.Logger.ZapLogger.Info("Fetched story variant", zap.Any("variants", userVariant), zap.Int64("chatID", chatID))

	//TODO записывем выбор в бд

	//TODO генерим ответ ии - вынести в другую функцию потом

	// Генерируем ответ ии
	allStory, dbErr := s.DBStory.GetAllStorySegments(ctx, chatID)
	if dbErr != nil {
		s.Logger.ZapLogger.Error("Failed to get all story segments", zap.Error(dbErr), zap.Int64("chatID", chatID))
		// You may want to return here or handle the error appropriately
	}
	fullStory := ""
	for _, segment := range allStory.StorySegments {
		fullStory = "\n" + segment
	}
	segment, aiErr := s.AIStory.GenerateNextStorySegment(ctx, fullStory)
	if aiErr != nil {
		s.Logger.ZapLogger.Error("AI failed to generate next story segment", zap.Error(aiErr), zap.Int64("chatID", chatID))
		// You may want to return here or handle the error appropriately
	}
	narrative := segment.Narrative
	choise := segment.Choices

	s.Logger.ZapLogger.Info("Generated next segment", zap.String("narrative", narrative), zap.Any("choices", choise), zap.Int64("chatID", chatID))

	//TODO записываем в бд повестование

	//TODO записываем в бд варианты выборов

	//TODO отправляем сообщение юзеру с вариантами ответа

	resp := text_messages.TextNarrativeWithChoices(narrative, choise)
	//TODO сделать выбор вывода героя или истории
	user_variant := text_messages.FormatHeroDescription(userVariant)
	return resp, user_variant, nil
}

func (s *StoryServiceImpl) CreateUser(ctx context.Context, chatID int64, userID int64) (string, error) {
	s.Logger.ZapLogger.Info("Creating user", zap.Any("chatID", chatID), zap.Any("userID", userID))
	user := models.NewUser(chatID, userID)
	err := s.DBStory.AddUser(ctx, user)
	if err != nil {
		if strings.HasPrefix(err.Error(), "client: ") {
			s.Logger.ZapLogger.Warn("Client error", zap.Error(err), zap.Any("chatID", chatID), zap.Any("userID", userID))
			return text_messages.TextHelp(), err
		}
		s.Logger.ZapLogger.Error("Server error", zap.Error(err), zap.Any("chatID", chatID), zap.Any("userID", userID))
		return text_messages.TextErrorCreateTask, err
	}
	s.Logger.ZapLogger.Info("User created successfully", zap.Any("chatID", chatID), zap.Any("userID", userID))
	return text_messages.TextGreeting, nil
}
