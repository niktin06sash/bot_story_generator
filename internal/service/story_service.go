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

	AddVariant(ctx context.Context, tx pgx.Tx, variant *models.StoryVariant) error
	GetActiveVariants(ctx context.Context, userID int64) ([]*models.StoryVariant, error)

	GetDailyLimit(ctx context.Context, userID int64) (*models.DailyLimit, error)
	AddDailyLimit(ctx context.Context, tx pgx.Tx, dailyLimit *models.DailyLimit) error
	UpdateDailyLimit(ctx context.Context, tx pgx.Tx, dailyLimit *models.DailyLimit) error

	GetAllStorySegments(ctx context.Context, userID int64) (*models.AllStorySegments, error)
}
type StoryAI interface {
	GetChatCompletion(ctx context.Context) (string, error)
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
	// Проверяем в кэше, есть ли дневные ходы у пользователя для создания новой истории
	isExist, err := s.CStory.CheckExceededLimit(ctx, userID)
	if err != nil {
		s.Logger.ZapLogger.Warn("CheckExceededLimit(CreateStory)", zap.Error(err), zap.Any("userID", userID))
	} else if isExist {
		s.Logger.ZapLogger.Warn("CheckExceededLimit(CreateStory)", zap.Error(errors.New("cache: user has exceeded daily action limit")), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextErrorUserDailyLimit)
	} else if !isExist {
		s.Logger.ZapLogger.Info("CheckExceededLimit(CreateStory) Exceeded Limits not in cache. Checking in database...", zap.Any("userID", userID))
	}
	// Проверяем в базе данных, есть ли дневные ходы у пользователя для создания новой истории
	limit, err := s.DBStory.GetDailyLimit(ctx, userID)
	if err != nil {
		s.Logger.ZapLogger.Error("GetDailyLimit(CreateStory)", zap.Error(err), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	if limit.LimitCount <= limit.Count {
		s.Logger.ZapLogger.Warn("GetDailyLimit(CreateStory)", zap.Error(errors.New("client: user has exceeded daily action limit")), zap.Any("userID", userID))
		//Добавляем превышение лимита в кэш
		err := s.CStory.AddExceededLimit(ctx, userID)
		if err != nil {
			s.Logger.ZapLogger.Warn("AddExceededLimit(CreateStory)", zap.Error(err), zap.Any("userID", userID))
		}
		return nil, errors.New(text_messages.TextErrorUserDailyLimit)
	}

	// Проверяем, нет ли активных историй у пользователя в данный момент
	stories, err := s.DBStory.GetActiveStories(ctx, userID)
	if err != nil {
		s.Logger.ZapLogger.Error("GetActiveStories(CreateStory)", zap.Error(err), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	if len(stories) > 0 {
		s.Logger.ZapLogger.Warn("GetActiveStories(CreateStory)", zap.Error(errors.New("client: user already has an active history")), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextErrorUserActiveStory)
	}

	// Запрос в ИИ
	fantasyCharacters, err := s.AIStory.GetStructuredHeroes(ctx)
	if err != nil {
		s.Logger.ZapLogger.Error("GetStructuredHeroes(CreateStory)", zap.Error(err), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	data, err := json.Marshal(fantasyCharacters)
	if err != nil {
		s.Logger.ZapLogger.Error("Marshal(CreateStory)", zap.Error(err), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Создание транзакции для консистентности данных
	tx, err := s.DBStory.BeginTx(ctx)
	if err != nil {
		s.Logger.ZapLogger.Error("BeginTx(CreateStory)", zap.Error(err), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Создаем историю с пустыми данными(так как ждем выбор в следующем действии пользователя)
	story := models.NewStory(userID, nil)
	storyId, err := s.DBStory.AddStory(ctx, tx, story)
	if err != nil {
		s.Logger.ZapLogger.Error("AddStory(CreateStory)", zap.Error(err), zap.Any("userID", userID))
		rollbackErr := s.DBStory.RollbackTx(ctx, tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx(CreateStory)", zap.Error(rollbackErr), zap.Any("userID", userID))
		}
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Создаем начальный вариант с данными из ИИ
	variant := models.NewStoryVariant(storyId, "characters", data)
	err = s.DBStory.AddVariant(ctx, tx, variant)
	if err != nil {
		s.Logger.ZapLogger.Error("AddVariant(CreateStory)", zap.Error(err), zap.Any("userID", userID))
		rollbackErr := s.DBStory.RollbackTx(ctx, tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx(CreateStory)", zap.Error(rollbackErr), zap.Any("userID", userID))
		}
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Создаем начальный дневной лимит или увеличиваем на один(он будет включать в себя как действия с созданием новых историй, так и последующий выбор действий)
	err = s.updateOrAddDailyLimit(ctx, tx, limit, 2, "CreateStory")
	if err != nil {
		return nil, err
	}

	// Делаем подтверждение транзакции после изменения таблиц(+запись в истории, варианты, лимиты)
	err = s.DBStory.CommitTx(ctx, tx)
	if err != nil {
		s.Logger.ZapLogger.Error("CommitTx(CreateStory)", zap.Error(err), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	s.Logger.ZapLogger.Info("Story created successfully(CreateStory)", zap.Any("userID", userID))
	return text_messages.NewChouseHero(fantasyCharacters), nil
}

func (s *StoryServiceImpl) UserChoice(ctx context.Context, userID int64, num string) ([]string, error) {
	number_choise, err := strconv.Atoi(num)
	if err != nil {
		s.Logger.ZapLogger.Error("Invalid user choice(UserChoice)", zap.Error(err), zap.Any("userID", userID), zap.String("choice", num))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Проверяем лимиты
	limit, err := s.DBStory.GetDailyLimit(ctx, userID)
	if err != nil {
		s.Logger.ZapLogger.Error("GetDailyLimit(UserChoice)", zap.Error(err), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	if limit.LimitCount <= limit.Count {
		s.Logger.ZapLogger.Warn("GetDailyLimit(UserChoice)", zap.Error(errors.New("client: user has exceeded daily action limit")), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextErrorUserDailyLimit)
	}

	// Получаем варианты выбора пользователя
	variants, dbErr := s.DBStory.GetActiveVariants(ctx, userID)
	if dbErr != nil {
		s.Logger.ZapLogger.Error("GetActiveVariants(UserChoice)", zap.Error(dbErr), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	if len(variants) > 1 {
		s.Logger.ZapLogger.Error("GetActiveVariants(UserChoice)", zap.Error(fmt.Errorf("server: more one active story found")), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	if len(variants) == 0 {
		s.Logger.ZapLogger.Error("GetActiveVariants(UserChoice)", zap.Error(fmt.Errorf("server: no one active story found")), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	variant := variants[0]

	// Определяем текст выбора пользователя
	var msg string
	switch variant.Type {
	case "characters":
		var fantasyCharacters models.FantasyCharacters
		err = json.Unmarshal(variant.Data, &fantasyCharacters)
		if err != nil {
			s.Logger.ZapLogger.Error("Unmarshal(UserChoice)", zap.Error(err), zap.Any("userID", userID))
			return nil, errors.New(text_messages.TextErrorCreateTask)
		}
		userVariant := fantasyCharacters.Characters[number_choise]
		msg = text_messages.CreateHeroMessage(&userVariant)
		s.Logger.ZapLogger.Info("Fetched story variant(UserChoice)", zap.Any("variants", userVariant), zap.Any("userID", userID))
	case "actions":
		var storyActions models.StoryChoise
		err = json.Unmarshal(variant.Data, &storyActions)
		if err != nil {
			s.Logger.ZapLogger.Error("Unmarshal(UserChoice)", zap.Error(err), zap.Any("userID", userID))
			return nil, errors.New(text_messages.TextErrorCreateTask)
		}
		userVariant := storyActions.Story[number_choise]
		msg = text_messages.CreateExtensionMessage(&userVariant)
		s.Logger.ZapLogger.Info("Fetched action variant(UserChoice)", zap.Any("variant", userVariant), zap.Any("userID", userID))
	}

	//для начала сделай всю работу с ии и только потом пиши в базу данных, чтобы транзакция не висела и не ждала когда ии нагенерит

	//TODO генерим ответ ии - вынести в другую функцию потом

	// Генерируем ответ ии
	allStory, dbErr := s.DBStory.GetAllStorySegments(ctx, userID)
	if dbErr != nil {
		s.Logger.ZapLogger.Error("GetAllStorySegments(UserChoice)", zap.Error(dbErr), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	fullStory := ""
	for _, segment := range allStory.StorySegments {
		fullStory += "\n" + segment
	}
	segment, aiErr := s.AIStory.GenerateNextStorySegment(ctx, fullStory)
	if aiErr != nil {
		s.Logger.ZapLogger.Error("GenerateNextStorySegment(UserChoice)", zap.Error(aiErr), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	narrative := segment.Narrative
	choise := segment.Choices

	s.Logger.ZapLogger.Info("Generated next segment(UserChoice)", zap.Any("userID", userID), zap.String("narrative", narrative), zap.Any("choices", choise))

	// Создание транзакции для консистентности данных
	tx, err := s.DBStory.BeginTx(ctx)
	if err != nil {
		s.Logger.ZapLogger.Error("BeginTx(UserChoice)", zap.Error(err), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	//TODO записывем выбор в бд

	//TODO записываем в бд повестование

	//TODO записываем в бд варианты выборов

	//TODO отправляем сообщение юзеру с вариантами ответа

	// Создаем начальный дневной лимит или увеличиваем на один(он будет включать в себя как действия с созданием новых историй, так и последующий выбор действий)
	err = s.updateOrAddDailyLimit(ctx, tx, limit, 1, "UserChoice")
	if err != nil {
		return nil, err
	}

	// Делаем подтверждение транзакции после изменения таблиц
	err = s.DBStory.CommitTx(ctx, tx)
	if err != nil {
		s.Logger.ZapLogger.Error("CommitTx(UserChoice)", zap.Error(err), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Возвращаем ответ
	resp := text_messages.TextNarrativeWithChoices(narrative, choise)
	return []string{msg, resp}, nil
}

func (s *StoryServiceImpl) CreateUser(ctx context.Context, userID int64) ([]string, error) {
	isExist, err := s.CStory.CheckCreatedUser(ctx, userID)
	if err != nil {
		s.Logger.ZapLogger.Warn("CheckCreatedUser(CreateUser)", zap.Error(err), zap.Any("userID", userID))
	} else if isExist {
		s.Logger.ZapLogger.Warn("CheckCreatedUser(CreateUser)", zap.Error(errors.New("cache: user is already registered")), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextGreeting)
	} else if !isExist {
		s.Logger.ZapLogger.Info("CheckCreatedUser(CreateUser) Created user not in cache. Trying creating in database...", zap.Any("userID", userID))
	}
	user := models.NewUser(userID)
	err = s.DBStory.AddUser(ctx, user)
	if err != nil {
		if strings.HasPrefix(err.Error(), "client: ") {
			s.Logger.ZapLogger.Warn("AddUser(CreateUser)", zap.Error(err), zap.Any("userID", userID))
			err := s.CStory.AddCreatedUser(ctx, userID)
			if err != nil {
				s.Logger.ZapLogger.Warn("AddCreatedUser(CreateUser)", zap.Error(err), zap.Any("userID", userID))
			}
			return nil, errors.New(text_messages.TextGreeting)
		}
		s.Logger.ZapLogger.Error("AddUser(CreateUser)", zap.Error(err), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	err = s.CStory.AddCreatedUser(ctx, userID)
	if err != nil {
		s.Logger.ZapLogger.Warn("AddCreatedUser(Createuser)", zap.Error(err), zap.Any("userID", userID))
	}
	s.Logger.ZapLogger.Info("User created successfully", zap.Any("userID", userID))
	return []string{text_messages.TextGreeting}, nil
}

func (s *StoryServiceImpl) StopStory(ctx context.Context, userID int64) ([]string, error) {
	s.Logger.ZapLogger.Info("Checking active story...", zap.Any("userID", userID))
	stories, err := s.DBStory.GetActiveStories(ctx, userID)
	if err != nil {
		s.Logger.ZapLogger.Error("GetActiveStories(StopStory)", zap.Error(err), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	if len(stories) > 1 {
		s.Logger.ZapLogger.Error("GetActiveStories(StopStory)", zap.Error(fmt.Errorf("server: more one active story found")), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	if len(stories) == 0 {
		s.Logger.ZapLogger.Warn("GetActiveStories(StopStory)", zap.Error(fmt.Errorf("client: user already has not an active history")), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextNoActiveStory)
	}
	s.Logger.ZapLogger.Info("Check active story successfully", zap.Any("userID", userID))
	return []string{text_messages.TextStopActiveStory}, nil
}

func (s *StoryServiceImpl) StopStoryChoice(ctx context.Context, userID int64, arg string) ([]string, error) {
	if arg == "❌" {
		return nil, nil
	}
	s.Logger.ZapLogger.Info("Stopping active story...", zap.Any("userID", userID))
	err := s.DBStory.StopStory(ctx, userID)
	if err != nil {
		s.Logger.ZapLogger.Error("StopStory(StopStoryChoice)", zap.Error(err), zap.Any("userID", userID))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	s.Logger.ZapLogger.Info("Stop active story successfully", zap.Any("userID", userID))
	return []string{text_messages.TextSuccessStopStory}, nil
}
