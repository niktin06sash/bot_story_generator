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

type StoryDatabase interface {
	//транзакции для изменения данных в нескольких таблицах за одно действие в сервисе(можно будет в будущем вынести в отдельный интерфейс)
	BeginTx(ctx context.Context) (pgx.Tx, error)
	RollbackTx(ctx context.Context, tx pgx.Tx) error
	CommitTx(ctx context.Context, tx pgx.Tx) error

	AddUser(ctx context.Context, user *models.User) error
	GetActiveStories(ctx context.Context, userID int64) error
	AddStory(ctx context.Context, tx pgx.Tx, story *models.Story) (int, error)
	AddVariant(ctx context.Context, tx pgx.Tx, variant *models.StoryVariant) error
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
	logger  *logger.Logger
}

func NewStoryService(db StoryDatabase, ai StoryAI, logger *logger.Logger) *StoryServiceImpl {
	return &StoryServiceImpl{DBStory: db, AIStory: ai, logger: logger}
}

func (s *StoryServiceImpl) CreateStory(ctx context.Context, chatID int64, userID int64) (string, error) {
	s.logger.ZapLogger.Info("Creating new story", zap.Any("chatID", chatID), zap.Any("userID", userID))
	//проверяем нет ли активных историй у пользователя в данный момент
	err := s.DBStory.GetActiveStories(ctx, userID)
	if err != nil {
		if strings.HasPrefix(err.Error(), "client: ") {
			s.logger.ZapLogger.Warn("Client error", zap.Error(err), zap.Any("chatID", chatID), zap.Any("userID", userID))
			return text_messages.TextErrorUserActiveStory, err
		}
		s.logger.ZapLogger.Error("Server error", zap.Error(err), zap.Any("chatID", chatID), zap.Any("userID", userID))
		return text_messages.TextErrorCreateTask, err
	}
	//запрос в ии
	fantasyCharacters, err := s.AIStory.GetStructuredHeroes(ctx)
	if err != nil {
		s.logger.ZapLogger.Error("GetStructuredHeroes failed", zap.Error(err), zap.Any("chatID", chatID), zap.Any("userID", userID))
		return text_messages.TextErrorCreateTask, err
	}
	data, err := json.Marshal(fantasyCharacters)
	if err != nil {
		s.logger.ZapLogger.Error("Marshal failed", zap.Error(err), zap.Any("chatID", chatID), zap.Any("userID", userID))
		return text_messages.TextErrorCreateTask, err
	}
	//создание транзакции для консистентности данных
	tx, err := s.DBStory.BeginTx(ctx)
	if err != nil {
		s.logger.ZapLogger.Error("BeginTx failed", zap.Error(err), zap.Any("chatID", chatID), zap.Any("userID", userID))
		return text_messages.TextErrorCreateTask, err
	}
	story := models.NewStory(userID, nil)
	//создаем историю с пустыми данными(так как ждем выбор в следующем действии пользователя)
	storyId, err := s.DBStory.AddStory(ctx, tx, story)
	if err != nil {
		s.logger.ZapLogger.Error("AddStory failed", zap.Error(err), zap.Any("chatID", chatID), zap.Any("userID", userID))
		err := s.DBStory.RollbackTx(ctx, tx)
		if err != nil {
			s.logger.ZapLogger.Error("RollbackTx failed", zap.Error(err), zap.Any("chatID", chatID), zap.Any("userID", userID))
		}
		return text_messages.TextErrorCreateTask, err
	}
	variant := models.NewStoryVariant(storyId, data)
	//создаем начальный вариант с данными из ии
	err = s.DBStory.AddVariant(ctx, tx, variant)
	if err != nil {
		s.logger.ZapLogger.Error("AddVariant failed", zap.Error(err), zap.Any("chatID", chatID), zap.Any("userID", userID))
		err := s.DBStory.RollbackTx(ctx, tx)
		if err != nil {
			s.logger.ZapLogger.Error("RollbackTx failed", zap.Error(err), zap.Any("chatID", chatID), zap.Any("userID", userID))
		}
		return text_messages.TextErrorCreateTask, err
	}
	//делаем подтверждение транзакции после изменения таблиц(+запись в истории, варианты)
	err = s.DBStory.CommitTx(ctx, tx)
	if err != nil {
		s.logger.ZapLogger.Error("CommitTx failed", zap.Error(err), zap.Any("chatID", chatID), zap.Any("userID", userID))
		return text_messages.TextErrorCreateTask, err
	}
	s.logger.ZapLogger.Info("Story created successfully", zap.Any("chatID", chatID), zap.Any("userID", userID))
	return text_messages.TextChooseHero(fantasyCharacters), nil
}
func (s *StoryServiceImpl) UserChoice(ctx context.Context, chatID int64, data string) (string, error) {
	//TODO добавить проверку на токены и сделать что то с тем,
	//TODO что если токенов нет будет, но юзер сделает выбор, то кнопки пропадут
	//можно убирать кнопки только после успешного исполнения задачи
	//если даже на кнопку нажали повторно, то мьютекс заблочит задачу из первого нажатия и будет скипать последующие
	//будто бы контекстом с ключом так же сообщить боту, но все упирается в айди сообщения телеграм
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

	allStory, dbErr := s.DBStory.GetAllStorySegments(ctx, chatID)
	if dbErr != nil {
		//TODO обработать
	}
	fullStory := ""
	for _, segment := range allStory.StorySegments {
		fullStory = "\n" + segment
	}
	segment, aiErr := s.AIStory.GenerateNextStorySegment(ctx, fullStory)
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

func (s *StoryServiceImpl) CreateUser(ctx context.Context, chatID int64, userID int64) (string, error) {
	s.logger.ZapLogger.Info("Creating user", zap.Any("chatID", chatID), zap.Any("userID", userID))
	user := models.NewUser(chatID, userID)
	err := s.DBStory.AddUser(ctx, user)
	if err != nil {
		if strings.HasPrefix(err.Error(), "client: ") {
			s.logger.ZapLogger.Warn("Client error", zap.Error(err), zap.Any("chatID", chatID), zap.Any("userID", userID))
			return text_messages.TextHelp(), err
		}
		s.logger.ZapLogger.Error("Server error", zap.Error(err), zap.Any("chatID", chatID), zap.Any("userID", userID))
		return text_messages.TextErrorCreateTask, err
	}
	s.logger.ZapLogger.Info("User created successfully", zap.Any("chatID", chatID), zap.Any("userID", userID))
	return text_messages.TextGreeting, nil
}
