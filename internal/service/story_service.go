package service

import (
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/models"
	"bot_story_generator/internal/text_messages"
	"encoding/json"
	"errors"
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
	//разделить интерфейс на множество маленьких для каждой таблицы
	AddUser(ctx context.Context, user *models.User) error

	CheckActiveStories(ctx context.Context, userID int64) error
	AddStory(ctx context.Context, tx pgx.Tx, story *models.Story) (int, error)

	AddVariant(ctx context.Context, tx pgx.Tx, variant *models.StoryVariant) error
	GetVariants(ctx context.Context, userID int64) (*models.StoryVariant, error)

	CheckDailyLimit(ctx context.Context, userID int64) (*models.DailyLimit, error)
	AddDailyLimit(ctx context.Context, tx pgx.Tx, dailyLimit *models.DailyLimit) error
	IncrementDailyLimit(ctx context.Context, tx pgx.Tx, userID int64) error

	GetAllStorySegments(ctx context.Context, userID int64) (*models.AllStorySegments, error)
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
func (s *StoryServiceImpl) CreateStory(ctx context.Context, userID int64) ([]string, error) {
	s.Logger.ZapLogger.Info("Creating new story", zap.Any("userID", userID))
	// Проверяем, есть ли дневные ходы у пользователя для создания новой истории
	limit, err := s.DBStory.CheckDailyLimit(ctx, userID)
	if err != nil {
		if strings.HasPrefix(err.Error(), "client: ") {
			s.Logger.ZapLogger.Warn("Client error", zap.Error(err), zap.Any("userID", userID))
			return nil, errors.New(text_messages.TextErrorUserDailyLimit)
		}
		s.Logger.ZapLogger.Error("Server error", zap.Error(err), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	// Проверяем, нет ли активных историй у пользователя в данный момент
	err = s.DBStory.CheckActiveStories(ctx, userID)
	if err != nil {
		if strings.HasPrefix(err.Error(), "client: ") {
			s.Logger.ZapLogger.Warn("Client error", zap.Error(err), zap.Any("userID", userID))
			return nil, errors.New(text_messages.TextErrorUserActiveStory)
		}
		s.Logger.ZapLogger.Error("Server error", zap.Error(err), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	// Запрос в ИИ
	fantasyCharacters, err := s.AIStory.GetStructuredHeroes(ctx)
	if err != nil {
		s.Logger.ZapLogger.Error("GetStructuredHeroes failed", zap.Error(err), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	data, err := json.Marshal(fantasyCharacters)
	if err != nil {
		s.Logger.ZapLogger.Error("Marshal failed", zap.Error(err), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	// Создание транзакции для консистентности данных
	tx, err := s.DBStory.BeginTx(ctx)
	if err != nil {
		s.Logger.ZapLogger.Error("BeginTx failed", zap.Error(err), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	// Создаем историю с пустыми данными(так как ждем выбор в следующем действии пользователя)
	story := models.NewStory(userID, nil)
	storyId, err := s.DBStory.AddStory(ctx, tx, story)
	if err != nil {
		s.Logger.ZapLogger.Error("AddStory failed", zap.Error(err), zap.Any("userID", userID))
		rollbackErr := s.DBStory.RollbackTx(ctx, tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx failed", zap.Error(rollbackErr), zap.Any("userID", userID))
		}
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	// Создаем начальный вариант с данными из ИИ
	variant := models.NewStoryVariant(storyId, "characters", data)
	err = s.DBStory.AddVariant(ctx, tx, variant)
	if err != nil {
		s.Logger.ZapLogger.Error("AddVariant failed", zap.Error(err), zap.Any("userID", userID))
		rollbackErr := s.DBStory.RollbackTx(ctx, tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx failed", zap.Error(rollbackErr), zap.Any("userID", userID))
		}
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	// Создаем начальный дневной лимит или увеличиваем на один(он будет включать в себя как действия с созданием новых историй, так и последующий выбор действий)
	err = s.incrementOrAddDailyLimit(ctx, tx, limit)
	if err != nil {
		return nil, err
	}
	// Делаем подтверждение транзакции после изменения таблиц(+запись в истории, варианты, лимиты)
	err = s.DBStory.CommitTx(ctx, tx)
	if err != nil {
		s.Logger.ZapLogger.Error("CommitTx failed", zap.Error(err), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	s.Logger.ZapLogger.Info("Story created successfully", zap.Any("userID", userID))
	return text_messages.NewChouseHero(fantasyCharacters), nil
}

func (s *StoryServiceImpl) UserChoice(ctx context.Context, userID int64, num string) ([]string, error) {
	//TODO добавить проверку на токены и сделать что то с тем,
	//TODO что если токенов нет будет, но юзер сделает выбор, то кнопки пропадут
	//TODO можно убирать кнопки только после успешного исполнения задачи
	//TODO если даже на кнопку нажали повторно, то мьютекс заблочит задачу из первого нажатия и будет скипать последующие
	//TODO будто бы контекстом с ключом так же сообщить боту, но все упирается в айди сообщения телеграм
	s.Logger.ZapLogger.Info("User made a choice", zap.Any("userID", userID), zap.String("choice", num))
	number_choise, err := strconv.Atoi(num)
	if err != nil {
		s.Logger.ZapLogger.Error("Invalid user choice", zap.Error(err), zap.Any("userID", userID), zap.String("choice", num))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	limit, err := s.DBStory.CheckDailyLimit(ctx, userID)
	if err != nil {
		if strings.HasPrefix(err.Error(), "client: ") {
			s.Logger.ZapLogger.Warn("Client error", zap.Error(err), zap.Any("userID", userID))
			return nil, errors.New(text_messages.TextErrorUserDailyLimit)
		}
		s.Logger.ZapLogger.Error("Server error", zap.Error(err), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	//* Получаем варианты выбора пользователя
	variant, dbErr := s.DBStory.GetVariants(ctx, userID)
	if dbErr != nil {
		s.Logger.ZapLogger.Error("GetVariants failed", zap.Error(dbErr), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	//TODO определить, что получаем - fantasyCharactres или storyVariants
	switch variant.Type {
	case "characters":
		var fantasyCharacters models.FantasyCharacters
		err = json.Unmarshal(variant.Data, &fantasyCharacters)
		if err != nil {
			s.Logger.ZapLogger.Error("Failed to unmarshal fantasy characters", zap.Error(err), zap.Any("userID", userID))
			return nil, errors.New(text_messages.TextErrorCreateTask)
		}
		userVariant := fantasyCharacters.Characters[number_choise]
		s.Logger.ZapLogger.Info("Fetched story variant", zap.Any("variants", userVariant), zap.Any("userID", userID))
	case "actions":
		//че тут к чему в итоге приводить - нет такого типа storyVariants в models
		var fantasyActions models.StoryNode
		err = json.Unmarshal(variant.Data, &fantasyActions)
		if err != nil {
			s.Logger.ZapLogger.Error("Failed to unmarshal fantasy actions", zap.Error(err), zap.Any("userID", userID))
			return nil, errors.New(text_messages.TextErrorCreateTask)
		}
	}
	//для начала сделай всю работу с ии и только потом пиши в базу данных, чтобы транзакция не висела и не ждала когда ии нагенерит

	//TODO генерим ответ ии - вынести в другую функцию потом

	// Генерируем ответ ии
	allStory, dbErr := s.DBStory.GetAllStorySegments(ctx, userID)
	if dbErr != nil {
		s.Logger.ZapLogger.Error("Failed to get all story segments", zap.Error(dbErr), zap.Any("userID", userID))
		// You may want to return here or handle the error appropriately
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	fullStory := ""
	for _, segment := range allStory.StorySegments {
		fullStory = "\n" + segment
	}
	segment, aiErr := s.AIStory.GenerateNextStorySegment(ctx, fullStory)
	if aiErr != nil {
		s.Logger.ZapLogger.Error("AI failed to generate next story segment", zap.Error(aiErr), zap.Any("userID", userID))
		// You may want to return here or handle the error appropriately
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	narrative := segment.Narrative
	choise := segment.Choices

	s.Logger.ZapLogger.Info("Generated next segment", zap.Any("userID", userID), zap.String("narrative", narrative), zap.Any("choices", choise))
	// Создание транзакции для консистентности данных
	tx, err := s.DBStory.BeginTx(ctx)
	if err != nil {
		s.Logger.ZapLogger.Error("BeginTx failed", zap.Error(err), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	//TODO записывем выбор в бд
	//TODO записываем в бд повестование

	//TODO записываем в бд варианты выборов

	//TODO отправляем сообщение юзеру с вариантами ответа

	resp := text_messages.TextNarrativeWithChoices(narrative, choise)
	//TODO сделать выбор вывода героя или истории
	user_variant := text_messages.FormatHeroDescription(userVariant)
	// Создаем начальный дневной лимит или увеличиваем на один(он будет включать в себя как действия с созданием новых историй, так и последующий выбор действий)
	err = s.incrementOrAddDailyLimit(ctx, tx, limit)
	if err != nil {
		return nil, err
	}
	// Делаем подтверждение транзакции после изменения таблиц
	err = s.DBStory.CommitTx(ctx, tx)
	if err != nil {
		s.Logger.ZapLogger.Error("CommitTx failed", zap.Error(err), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	return []string{user_variant, resp}, nil
}

func (s *StoryServiceImpl) CreateUser(ctx context.Context, userID int64) (string, error) {
	s.Logger.ZapLogger.Info("Creating user", zap.Any("userID", userID))
	user := models.NewUser(userID)
	err := s.DBStory.AddUser(ctx, user)
	if err != nil {
		if strings.HasPrefix(err.Error(), "client: ") {
			s.Logger.ZapLogger.Warn("Client error", zap.Error(err), zap.Any("userID", userID))
			return text_messages.TextHelp(), err
		}
		s.Logger.ZapLogger.Error("Server error", zap.Error(err), zap.Any("userID", userID))
		return text_messages.TextErrorCreateTask, err
	}
	s.Logger.ZapLogger.Info("User created successfully", zap.Any("userID", userID))
	return text_messages.TextGreeting, nil
}
