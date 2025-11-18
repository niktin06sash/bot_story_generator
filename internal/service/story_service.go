package service

import (
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/models"
	"bot_story_generator/internal/text_messages"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"go.uber.org/zap"
)

type StoryServiceImpl struct {
	SettingCache       SettingCache
	SubDatabase        SubscriptionDatabase
	DailyLimitCache    DailyLimitCache
	DailyLimitDatabase DailyLimitDatabase
	StoryDatabase      StoryDatabase
	StoryAI            StoryAI
	TxManager          TxManager
	VariantDatabase    VariantDatabase
	MsgDatabase        MessageDatabase
	Logger             *logger.Logger
}

func NewStoryService(settCache SettingCache, subdb SubscriptionDatabase, dcache DailyLimitCache, ddb DailyLimitDatabase, stdb StoryDatabase, stAi StoryAI, txman TxManager, vardb VariantDatabase, msgdb MessageDatabase, logger *logger.Logger) *StoryServiceImpl {
	return &StoryServiceImpl{
		SubDatabase:        subdb,
		SettingCache:       settCache,
		MsgDatabase:        msgdb,
		DailyLimitCache:    dcache,
		DailyLimitDatabase: ddb,
		StoryDatabase:      stdb,
		StoryAI:            stAi,
		VariantDatabase:    vardb,
		TxManager:          txman,
		Logger:             logger,
	}
}
func (s *StoryServiceImpl) CreateStory(ctx context.Context, userID int64) ([]string, error) {
	place := "CreateStory"
	trace := getTrace(ctx, s.Logger)
	// Проверяем, нет ли активных историй у пользователя в данный момент
	stories, err := s.StoryDatabase.GetActiveStories(ctx, userID)
	if err != nil {
		s.Logger.ZapLogger.Error("GetActiveStories", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	if len(stories) > 1 {
		s.Logger.ZapLogger.Error("GetActiveStories", zap.Error(fmt.Errorf("server: more than one active story found")), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	if len(stories) > 0 {
		s.Logger.ZapLogger.Warn("GetActiveStories", zap.Error(errors.New("client: user already has an active history")), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorUserActiveStory)
	}
	// Проверяем, есть ли дневные ходы у пользователя для создания новой истории + подписка
	limit, err := checkDailyLimits(ctx, userID, trace, place, s.DailyLimitCache, s.DailyLimitDatabase, s.SubDatabase, s.SettingCache, s.Logger)
	if err != nil {
		return nil, err
	}
	// Запрос в ИИ
	fantasyCharacters, err := s.StoryAI.GetStructuredHeroes(ctx)
	if err != nil {
		s.Logger.ZapLogger.Error("GetStructuredHeroes", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	if len(fantasyCharacters.Characters) == 0 {
		s.Logger.ZapLogger.Error("GetStructuredHeroes", zap.Error(errors.New("empty response from AI")), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	data, err := json.Marshal(fantasyCharacters)
	if err != nil {
		s.Logger.ZapLogger.Error("Marshal", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	// Создание транзакции для консистентности данных
	//создание контекста с таймаутом для изменения данных
	//TODO выставить таймер для всей операции, но пока не получится из-за ИИ(долгое выполнение)
	ctxTimeout, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	tx, err := s.TxManager.BeginTx(ctxTimeout)
	if err != nil {
		s.Logger.ZapLogger.Error("BeginTx", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Создаем историю с пустыми данными(так как ждем выбор в следующем действии пользователя)
	story := models.NewStory(userID, nil)
	storyId, err := s.StoryDatabase.AddStory(ctxTimeout, tx, story)
	if err != nil {
		s.Logger.ZapLogger.Error("AddStory", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		//в случае отмены контекста(при завершении исполнения у нас может не сделаться rollback или commit транзакции - возможное решение использовать отдельный контекст для данных операций)
		rollbackErr := s.TxManager.RollbackTx(context.Background(), tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		}
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Создаем начальный вариант с данными из ИИ
	variant := models.NewStoryVariant(storyId, "characters", data)
	err = s.VariantDatabase.AddVariant(ctxTimeout, tx, variant)
	if err != nil {
		s.Logger.ZapLogger.Error("AddVariant", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		rollbackErr := s.TxManager.RollbackTx(context.Background(), tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		}
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Создаем начальный дневной лимит или обновляем(он будет включать в себя как действия с созданием новых историй, так и последующий выбор действий)
	err = updateOrAddDailyLimit(ctxTimeout, tx, limit, 2, trace, place, s.DailyLimitDatabase, s.TxManager, s.Logger)
	if err != nil {
		return nil, err
	}

	// Делаем подтверждение транзакции после изменения таблиц(+запись в истории, варианты, лимиты)
	err = s.TxManager.CommitTx(ctxTimeout, tx)
	if err != nil {
		s.Logger.ZapLogger.Error("CommitTx", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		rollbackErr := s.TxManager.RollbackTx(context.Background(), tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		}
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	//3 лог
	s.Logger.ZapLogger.Info("Story created successfully", zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
	return text_messages.NewChouseHero(fantasyCharacters), nil
}
func (s *StoryServiceImpl) StopStory(ctx context.Context, userID int64) (string, error) {
	place := "StopStory"
	trace := getTrace(ctx, s.Logger)
	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	stories, err := s.StoryDatabase.GetActiveStories(ctxTimeout, userID)
	if err != nil {
		s.Logger.ZapLogger.Error("GetActiveStories", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return "", errors.New(text_messages.TextErrorCreateTask)
	}
	if len(stories) > 1 {
		s.Logger.ZapLogger.Error("GetActiveStories", zap.Error(fmt.Errorf("server: more one active story found")), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return "", errors.New(text_messages.TextErrorCreateTask)
	}
	if len(stories) == 0 {
		s.Logger.ZapLogger.Warn("GetActiveStories", zap.Error(fmt.Errorf("client: user already has not an active history")), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return "", errors.New(text_messages.TextNoActiveStory)
	}
	//3 лог
	s.Logger.ZapLogger.Info("Active story checked successfully", zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
	return text_messages.TextStopActiveStory, nil
}

func (s *StoryServiceImpl) StopStoryChoice(ctx context.Context, userID int64, arg string) (string, error) {
	trace := getTrace(ctx, s.Logger)
	if arg == "❌" {
		return "", nil
	}
	place := "StopStoryChoice"
	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := s.StoryDatabase.StopStory(ctxTimeout, userID)
	if err != nil {
		s.Logger.ZapLogger.Error("StopStory", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return "", errors.New(text_messages.TextErrorCreateTask)
	}
	//3 лог
	s.Logger.ZapLogger.Info("Active story stopped successfully", zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
	return text_messages.TextSuccessStopStory, nil
}
func (s *StoryServiceImpl) UserChoice(ctx context.Context, userID int64, num string) ([]string, error) {
	trace := getTrace(ctx, s.Logger)
	place := "UserChoice"
	number_choice, err := strconv.Atoi(num)
	if err != nil || number_choice < 1 {
		s.Logger.ZapLogger.Error("Invalid user choice", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.String("choice", num), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	// Получаем варианты (последний активный storyVariant для пользователя)
	variants, err := s.VariantDatabase.GetActiveVariants(ctx, userID)
	if err != nil {
		s.Logger.ZapLogger.Error("GetActiveVariants", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	if len(variants) > 1 {
		s.Logger.ZapLogger.Error("GetActiveVariants", zap.Error(fmt.Errorf("server: more than one active story found")), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	if len(variants) == 0 {
		s.Logger.ZapLogger.Warn("GetActiveVariants", zap.Error(fmt.Errorf("client: user already has not an active history")), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextNoActiveStory)
	}
	// Проверяем, есть ли дневные ходы у пользователя для создания новой истории + подписка
	limit, err := checkDailyLimits(ctx, userID, trace, place, s.DailyLimitCache, s.DailyLimitDatabase, s.SubDatabase, s.SettingCache, s.Logger)
	if err != nil {
		return nil, err
	}

	variant := variants[0]
	storyID := variant.StoryID
	var msg string
	switch variant.Type {
	case "characters":
		var fantasyCharacters models.FantasyCharacters
		if err := json.Unmarshal(variant.Data, &fantasyCharacters); err != nil {
			s.Logger.ZapLogger.Error("Unmarshal", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
			return nil, errors.New(text_messages.TextErrorCreateTask)
		}
		userVariant := fantasyCharacters.Characters[number_choice-1]
		msg = text_messages.CreateHeroMessage(&userVariant)
		//3 лог
		s.Logger.ZapLogger.Info("Fetched story variant", zap.Any("variant", userVariant), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
	case "actions":
		var choices []string
		if err := json.Unmarshal(variant.Data, &choices); err != nil {
			s.Logger.ZapLogger.Error("Unmarshal", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
			return nil, errors.New(text_messages.TextErrorCreateTask)
		}
		userVariant := models.Extension{Narrative: choices[number_choice-1]}
		msg = text_messages.CreateExtensionMessage(&userVariant)
		//3 лог
		s.Logger.ZapLogger.Info("Fetched action variant", zap.Any("variant", userVariant), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
	default:
		s.Logger.ZapLogger.Error("Unknown variant type", zap.String("type", variant.Type), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Получаем все сегменты истории
	allStory, err := s.MsgDatabase.GetAllStorySegments(ctx, storyID)
	if err != nil {
		s.Logger.ZapLogger.Error("GetAllStorySegments", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Добавляем выбор пользователя в формат истории, в бд добавляем потом, во время транзакции
	storySegment := &models.StoryMessage{Data: msg, Type: "user"}
	allStory = append(allStory, storySegment)

	// Генериуем ответ ии
	segment, err := s.StoryAI.GenerateNextStorySegment(ctx, allStory)
	if err != nil {
		s.Logger.ZapLogger.Error("GenerateNextStorySegment", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	narrative := segment.Narrative
	choice := segment.Choices

	isEndStory := segment.IsEnding
	shortNarrative := segment.ShortNarrative

	if isEndStory {
		//TODO сами завершаем историю
	}

	//создание контекста с таймаутом для изменения данных
	//TODO выставить таймер для всей операции, но пока не получится из-за ИИ(долгое выполнение)
	ctxTimeout, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	// Создание транзакции для консистентности данных
	tx, err := s.TxManager.BeginTx(ctxTimeout)
	if err != nil {
		s.Logger.ZapLogger.Error("BeginTx", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	newUserMsg := models.NewStoryMessage(storyID, msg, "user")
	newAssistantMsg := models.NewStoryMessage(storyID, shortNarrative, "assistant") // Сохраняем shortNarrative вместо narrative

	// Сохраняем сообщения
	err = s.MsgDatabase.AddStoryMessages(ctxTimeout, tx, []*models.StoryMessage{newUserMsg, newAssistantMsg})
	if err != nil {
		s.Logger.ZapLogger.Error("AddStoryMessages", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		rollbackErr := s.TxManager.RollbackTx(context.Background(), tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		}
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Обновляем вариант
	choicesData, err := json.Marshal(choice)
	if err != nil {
		s.Logger.ZapLogger.Error("Marshal", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		rollbackErr := s.TxManager.RollbackTx(context.Background(), tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		}
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	addingVariant := models.NewStoryVariant(storyID, "actions", choicesData)
	if err = s.VariantDatabase.UpdateVariant(ctxTimeout, tx, addingVariant); err != nil {
		s.Logger.ZapLogger.Error("UpdateVariant", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		rollbackErr := s.TxManager.RollbackTx(context.Background(), tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		}
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Обновляем дневной лимит
	err = updateOrAddDailyLimit(ctxTimeout, tx, limit, 1, trace, place, s.DailyLimitDatabase, s.TxManager, s.Logger)
	if err != nil {
		return nil, err
	}

	// Делаем подтверждение транзакции после изменения таблиц
	err = s.TxManager.CommitTx(ctxTimeout, tx)
	if err != nil {
		s.Logger.ZapLogger.Error("CommitTx", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		rollbackErr := s.TxManager.RollbackTx(context.Background(), tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		}
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Формируем ответ
	resp := text_messages.TextNarrativeWithChoices(narrative, choice)
	//4 лог
	s.Logger.ZapLogger.Info("User's choice made successfully", zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
	return []string{msg, resp}, nil
}
