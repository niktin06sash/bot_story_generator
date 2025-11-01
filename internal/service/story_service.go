package service

import (
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/models"
	"bot_story_generator/internal/text_messages"
	"encoding/json"
	"errors"
	"fmt"
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

	GetActiveStories(ctx context.Context, userID int64) ([]*models.Story, error)
	StopStory(ctx context.Context, userID int64) error
	AddStory(ctx context.Context, tx pgx.Tx, story *models.Story) (int, error)
	GetActiveStoryID(ctx context.Context, userID int64) (int, error)

	AddVariant(ctx context.Context, tx pgx.Tx, variant *models.StoryVariant) error
	UpdateVariant(ctx context.Context, tx pgx.Tx, variant *models.StoryVariant) error
	GetActiveVariants(ctx context.Context, userID int64) ([]*models.StoryVariant, error)

	GetDailyLimit(ctx context.Context, userID int64) (*models.DailyLimit, error)
	AddDailyLimit(ctx context.Context, tx pgx.Tx, dailyLimit *models.DailyLimit) error
	UpdateDailyLimit(ctx context.Context, tx pgx.Tx, dailyLimit *models.DailyLimit) error

	AddStoryMessages(ctx context.Context, userID int64, data string) error
	GetAllStorySegments(ctx context.Context, userID int64) (*models.AllStorySegments, error)
}
type StoryAI interface {
	GetStructuredHeroes(ctx context.Context) (*models.FantasyCharacters, error)
	GenerateNextStorySegment(parctx context.Context, storyData string) (*models.StoryNode, error)
}
type StoryCache interface {
	AddCreatedUser(ctx context.Context, userID int64) error
	CheckCreatedUser(ctx context.Context, userID int64) (bool, error)
	AddExceededLimit(ctx context.Context, userID int64) error
	CheckExceededLimit(ctx context.Context, userID int64) (bool, error)
}
type StoryServiceImpl struct {
	DBStory StoryDatabase
	AIStory StoryAI
	CStory  StoryCache
	Logger  *logger.Logger
}

func NewStoryService(db StoryDatabase, ai StoryAI, cache StoryCache, logger *logger.Logger) *StoryServiceImpl {
	return &StoryServiceImpl{DBStory: db, AIStory: ai, CStory: cache, Logger: logger}
}

func (s *StoryServiceImpl) CreateStory(ctx context.Context, userID int64) ([]string, error) {
	place := "CreateStory"
	// Проверяем в кэше, есть ли дневные ходы у пользователя для создания новой истории
	limit, err := s.checkDailyLimits(ctx, userID, place)
	if err != nil {
		return nil, err
	}
	// Проверяем, нет ли активных историй у пользователя в данный момент
	stories, err := s.DBStory.GetActiveStories(ctx, userID)
	if err != nil {
		s.Logger.ZapLogger.Error("GetActiveStories", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	if len(stories) > 0 {
		s.Logger.ZapLogger.Warn("GetActiveStories", zap.Error(errors.New("client: user already has an active history")), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorUserActiveStory)
	}

	// Запрос в ИИ
	fantasyCharacters, err := s.AIStory.GetStructuredHeroes(ctx)
	if err != nil {
		s.Logger.ZapLogger.Error("GetStructuredHeroes", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	//TODO в юзер чойз че то подобное сделай
	if len(fantasyCharacters.Characters) == 0 {
		s.Logger.ZapLogger.Error("GetStructuredHeroes", zap.Error(errors.New("Empty response from AI")), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	data, err := json.Marshal(fantasyCharacters)
	if err != nil {
		s.Logger.ZapLogger.Error("Marshal", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Создание транзакции для консистентности данных
	tx, err := s.DBStory.BeginTx(ctx)
	if err != nil {
		s.Logger.ZapLogger.Error("BeginTx", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Создаем историю с пустыми данными(так как ждем выбор в следующем действии пользователя)
	story := models.NewStory(userID, nil)
	storyId, err := s.DBStory.AddStory(ctx, tx, story)
	if err != nil {
		s.Logger.ZapLogger.Error("AddStory", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		rollbackErr := s.DBStory.RollbackTx(ctx, tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("userID", userID), zap.Any("place", place))
		}
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Создаем начальный вариант с данными из ИИ
	variant := models.NewStoryVariant(storyId, "characters", data)
	err = s.DBStory.AddVariant(ctx, tx, variant)
	if err != nil {
		s.Logger.ZapLogger.Error("AddVariant", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		rollbackErr := s.DBStory.RollbackTx(ctx, tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("userID", userID), zap.Any("place", place))
		}
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Создаем начальный дневной лимит или увеличиваем на один(он будет включать в себя как действия с созданием новых историй, так и последующий выбор действий)
	err = s.updateOrAddDailyLimit(ctx, tx, limit, 2, place)
	if err != nil {
		return nil, err
	}

	// Делаем подтверждение транзакции после изменения таблиц(+запись в истории, варианты, лимиты)
	err = s.DBStory.CommitTx(ctx, tx)
	if err != nil {
		s.Logger.ZapLogger.Error("CommitTx", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	s.Logger.ZapLogger.Info("Story created successfully", zap.Any("userID", userID), zap.Any("place", place))
	return text_messages.NewChouseHero(fantasyCharacters), nil
}

func (s *StoryServiceImpl) UserChoice(ctx context.Context, userID int64, num string) ([]string, error) {
	place := "UserChoice"
	number_choice, err := strconv.Atoi(num)
	if err != nil {
		s.Logger.ZapLogger.Error("Invalid user choice", zap.Error(err), zap.Any("userID", userID), zap.String("choice", num), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	// Проверяем в кэше, есть ли дневные ходы у пользователя для создания новой истории
	limit, err := s.checkDailyLimits(ctx, userID, place)
	if err != nil {
		return nil, err
	}

	// Получаем варианты (последний актвный storyVariant для пользователя)
	variants, dbErr := s.DBStory.GetActiveVariants(ctx, userID)
	if dbErr != nil {
		s.Logger.ZapLogger.Error("GetActiveVariants", zap.Error(dbErr), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	if len(variants) > 1 {
		s.Logger.ZapLogger.Error("GetActiveVariants", zap.Error(fmt.Errorf("server: more than one active story found")), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	if len(variants) == 0 {
		s.Logger.ZapLogger.Error("GetActiveVariants", zap.Error(fmt.Errorf("server: no active story found")), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	variant := variants[0]

	if number_choice < 1 {
		s.Logger.ZapLogger.Error("User choice is invalid", zap.Int("choice", number_choice), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	var msg string
	switch variant.Type {
	case "characters":
		var fantasyCharacters models.FantasyCharacters
		if err := json.Unmarshal(variant.Data, &fantasyCharacters); err != nil {
			s.Logger.ZapLogger.Error("Unmarshal", zap.Error(err), zap.Any("userID", userID))
			return nil, errors.New(text_messages.TextErrorCreateTask)
		}
		userVariant := fantasyCharacters.Characters[number_choice-1]
		msg = text_messages.CreateHeroMessage(&userVariant)
		s.Logger.ZapLogger.Info("Fetched story variant", zap.Any("variant", userVariant), zap.Any("userID", userID), zap.Any("place", place))
	case "actions":
		var choices []string
		if err := json.Unmarshal(variant.Data, &choices); err != nil {
			s.Logger.ZapLogger.Error("Unmarshal", zap.Error(err), zap.Any("userID", userID))
			return nil, errors.New(text_messages.TextErrorCreateTask)
		}
		// Преобразуем строку выбора в Extension
		userVariant := models.Extension{Narrative: choices[number_choice-1]}
		msg = text_messages.CreateExtensionMessage(&userVariant)
		s.Logger.ZapLogger.Info("Fetched action variant", zap.Any("variant", userVariant), zap.Any("userID", userID), zap.Any("place", place))
	default:
		s.Logger.ZapLogger.Error("Unknown variant type", zap.String("type", variant.Type), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	//TODO генерим ответ ии - вынести в другую функцию потом

	// Генерируем ответ ии
	allStory, dbErr := s.DBStory.GetAllStorySegments(ctx, userID)
	if dbErr != nil {
		s.Logger.ZapLogger.Error("GetAllStorySegments", zap.Error(dbErr), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	s.Logger.ZapLogger.Info("AllStory loaded from DB", zap.Any("userID", userID), zap.Any("allStory", allStory), zap.Any("place", place))
	fullStory := ""
	for _, segment := range allStory.StorySegments {
		fullStory += "\n" + segment
	}
	// Добавляем выбор пользователя
	fullStory += "\n" + msg
	s.Logger.ZapLogger.Info("FullStory for AI generation", zap.Any("userID", userID), zap.String("fullStory", fullStory), zap.Any("place", place))
	segment, aiErr := s.AIStory.GenerateNextStorySegment(ctx, fullStory)
	if aiErr != nil {
		s.Logger.ZapLogger.Error("GenerateNextStorySegment", zap.Error(aiErr), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	narrative := segment.Narrative
	choise := segment.Choices

	s.Logger.ZapLogger.Info("Generated next segment", zap.Any("userID", userID), zap.String("narrative", narrative), zap.Any("choices", choise), zap.Any("place", place))

	// Создание транзакции для консистентности данных
	tx, err := s.DBStory.BeginTx(ctx)
	if err != nil {
		s.Logger.ZapLogger.Error("BeginTx", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Сохраняем выбор пользователя
	if err := s.DBStory.AddStoryMessages(ctx, userID, msg); err != nil {
		_ = s.DBStory.RollbackTx(ctx, tx)
		s.Logger.ZapLogger.Error("AddStoryMessages (msg)", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	s.Logger.ZapLogger.Info("User choice and AI segment saved", zap.Any("userID", userID), zap.String("msg", msg), zap.String("narrative", narrative), zap.Any("choices", choise), zap.Any("place", place))

	// Сохраняем повестование
	if err := s.DBStory.AddStoryMessages(ctx, userID, narrative); err != nil {
		_ = s.DBStory.RollbackTx(ctx, tx)
		s.Logger.ZapLogger.Error("AddStoryMessages (narrative)", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	s.Logger.ZapLogger.Info("Narrative saved", zap.Any("userID", userID), zap.String("narrative", narrative), zap.Any("place", place))

	// Сохраняем варианты выбора
	choicesData, err := json.Marshal(choise)
	if err != nil {
		_ = s.DBStory.RollbackTx(ctx, tx)
		s.Logger.ZapLogger.Error("Marshal choices", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	storyId, err := s.DBStory.GetActiveStoryID(ctx, userID)
	if err != nil {
		_ = s.DBStory.RollbackTx(ctx, tx)
		s.Logger.ZapLogger.Error("GetActiveStoryID", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	addingVariant := models.NewStoryVariant(storyId, "actions", choicesData)
	if dbErr = s.DBStory.UpdateVariant(ctx, tx, addingVariant); dbErr != nil {
		_ = s.DBStory.RollbackTx(ctx, tx)
		s.Logger.ZapLogger.Error("UpdateVariant (choices)", zap.Error(dbErr), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	s.Logger.ZapLogger.Info("Choices updated for current story", zap.Any("userID", userID), zap.Int("storyID", storyId), zap.Any("choices", choise), zap.Any("place", place))

	// Обновляем дневной лимит
	err = s.updateOrAddDailyLimit(ctx, tx, limit, 1, place)
	if err != nil {
		_ = s.DBStory.RollbackTx(ctx, tx)
		return nil, err
	}

	// Делаем подтверждение транзакции после изменения таблиц
	err = s.DBStory.CommitTx(ctx, tx)
	if err != nil {
		s.Logger.ZapLogger.Error("CommitTx", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Формируем ответ
	resp := text_messages.TextNarrativeWithChoices(narrative, choise)
	return []string{msg, resp}, nil
}

func (s *StoryServiceImpl) CreateUser(ctx context.Context, userID int64) ([]string, error) {
	place := "CreateUser"
	isExist, err := s.CStory.CheckCreatedUser(ctx, userID)
	if err != nil {
		s.Logger.ZapLogger.Warn("CheckCreatedUser", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
	} else if isExist {
		s.Logger.ZapLogger.Warn("CheckCreatedUser", zap.Error(errors.New("cache: user is already registered")), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextGreeting)
	} else if !isExist {
		s.Logger.ZapLogger.Info("CheckCreatedUser Created user not in cache. Trying creating in database...", zap.Any("userID", userID), zap.Any("place", place))
	}
	user := models.NewUser(userID)
	err = s.DBStory.AddUser(ctx, user)
	if err != nil {
		if strings.HasPrefix(err.Error(), "client: ") {
			s.Logger.ZapLogger.Warn("AddUser", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
			err := s.CStory.AddCreatedUser(ctx, userID)
			if err != nil {
				s.Logger.ZapLogger.Warn("AddCreatedUser", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
			}
			return nil, errors.New(text_messages.TextGreeting)
		}
		s.Logger.ZapLogger.Error("AddUser", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	err = s.CStory.AddCreatedUser(ctx, userID)
	if err != nil {
		s.Logger.ZapLogger.Warn("AddCreatedUser", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
	}
	s.Logger.ZapLogger.Info("User created successfully", zap.Any("userID", userID), zap.Any("place", place))
	return []string{text_messages.TextGreeting}, nil
}

func (s *StoryServiceImpl) StopStory(ctx context.Context, userID int64) ([]string, error) {
	place := "StopStory"
	stories, err := s.DBStory.GetActiveStories(ctx, userID)
	if err != nil {
		s.Logger.ZapLogger.Error("GetActiveStories", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	if len(stories) > 1 {
		s.Logger.ZapLogger.Error("GetActiveStories", zap.Error(fmt.Errorf("server: more one active story found")), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	if len(stories) == 0 {
		s.Logger.ZapLogger.Warn("GetActiveStories", zap.Error(fmt.Errorf("client: user already has not an active history")), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextNoActiveStory)
	}
	s.Logger.ZapLogger.Info("Check active story successfully", zap.Any("userID", userID), zap.Any("place", place))
	return []string{text_messages.TextStopActiveStory}, nil
}

func (s *StoryServiceImpl) StopStoryChoice(ctx context.Context, userID int64, arg string) ([]string, error) {
	if arg == "❌" {
		return nil, nil
	}
	place := "StopStoryChoice"
	err := s.DBStory.StopStory(ctx, userID)
	if err != nil {
		s.Logger.ZapLogger.Error("StopStory", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	s.Logger.ZapLogger.Info("Stop active story successfully", zap.Any("userID", userID), zap.Any("place", place))
	return []string{text_messages.TextSuccessStopStory}, nil
}
