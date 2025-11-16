package service

import (
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

func (s *ServiceImpl) CreateStory(ctx context.Context, userID int64, trace models.Trace) ([]string, error) {
	place := "CreateStory"
	// Проверяем, нет ли активных историй у пользователя в данный момент
	stories, err := s.storyDatabase.GetActiveStories(ctx, userID)
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
	limit, err := s.checkDailyLimits(ctx, userID, trace, place)
	if err != nil {
		return nil, err
	}
	// Запрос в ИИ
	fantasyCharacters, err := s.storyAI.GetStructuredHeroes(ctx)
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
	tx, err := s.txManager.BeginTx(ctxTimeout)
	if err != nil {
		s.Logger.ZapLogger.Error("BeginTx", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Создаем историю с пустыми данными(так как ждем выбор в следующем действии пользователя)
	story := models.NewStory(userID, nil)
	storyId, err := s.storyDatabase.AddStory(ctxTimeout, tx, story)
	if err != nil {
		s.Logger.ZapLogger.Error("AddStory", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		//в случае отмены контекста(при завершении исполнения у нас может не сделаться rollback или commit транзакции - возможное решение использовать отдельный контекст для данных операций)
		rollbackErr := s.txManager.RollbackTx(context.Background(), tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		}
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Создаем начальный вариант с данными из ИИ
	variant := models.NewStoryVariant(storyId, "characters", data)
	err = s.variantDatabase.AddVariant(ctxTimeout, tx, variant)
	if err != nil {
		s.Logger.ZapLogger.Error("AddVariant", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		rollbackErr := s.txManager.RollbackTx(context.Background(), tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		}
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Создаем начальный дневной лимит или обновляем(он будет включать в себя как действия с созданием новых историй, так и последующий выбор действий)
	err = s.updateOrAddDailyLimit(ctxTimeout, tx, limit, 2, trace, place)
	if err != nil {
		return nil, err
	}

	// Делаем подтверждение транзакции после изменения таблиц(+запись в истории, варианты, лимиты)
	err = s.txManager.CommitTx(ctxTimeout, tx)
	if err != nil {
		s.Logger.ZapLogger.Error("CommitTx", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		rollbackErr := s.txManager.RollbackTx(context.Background(), tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		}
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	//3 лог
	s.Logger.ZapLogger.Info("Story created successfully", zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
	return text_messages.NewChouseHero(fantasyCharacters), nil
}
func (s *ServiceImpl) StopStory(ctx context.Context, userID int64, trace models.Trace) (string, error) {
	place := "StopStory"
	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	stories, err := s.storyDatabase.GetActiveStories(ctxTimeout, userID)
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

func (s *ServiceImpl) StopStoryChoice(ctx context.Context, userID int64, arg string, trace models.Trace) (string, error) {
	if arg == "❌" {
		return "", nil
	}
	place := "StopStoryChoice"
	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := s.storyDatabase.StopStory(ctxTimeout, userID)
	if err != nil {
		s.Logger.ZapLogger.Error("StopStory", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return "", errors.New(text_messages.TextErrorCreateTask)
	}
	//3 лог
	s.Logger.ZapLogger.Info("Active story stopped successfully", zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
	return text_messages.TextSuccessStopStory, nil
}
func (s *ServiceImpl) UserChoice(ctx context.Context, userID int64, num string, trace models.Trace) ([]string, error) {
	place := "UserChoice"
	number_choice, err := strconv.Atoi(num)
	if err != nil || number_choice < 1 {
		s.Logger.ZapLogger.Error("Invalid user choice", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.String("choice", num), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	// Получаем варианты (последний активный storyVariant для пользователя)
	variants, err := s.variantDatabase.GetActiveVariants(ctx, userID)
	if err != nil {
		s.Logger.ZapLogger.Error("GetActiveVariants", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	if len(variants) > 1 {
		s.Logger.ZapLogger.Error("GetActiveVariants", zap.Error(fmt.Errorf("server: more than one active story found")), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	if len(variants) == 0 {
		s.Logger.ZapLogger.Error("GetActiveVariants", zap.Error(fmt.Errorf("server: no active story found")), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	// Проверяем, есть ли дневные ходы у пользователя для создания новой истории + подписка
	limit, err := s.checkDailyLimits(ctx, userID, trace, place)
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
	allStory, err := s.msgDatabase.GetAllStorySegments(ctx, storyID)
	if err != nil {
		s.Logger.ZapLogger.Error("GetAllStorySegments", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Добавляем выбор пользователя в формат истории, в бд добавляем потом, во время транзакции
	storySegment := &models.StoryMessage{Data: msg, Type: "user"}
	allStory = append(allStory, storySegment)

	// Генериуем ответ ии
	segment, err := s.storyAI.GenerateNextStorySegment(ctx, allStory)
	if err != nil {
		s.Logger.ZapLogger.Error("GenerateNextStorySegment", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	narrative := segment.Narrative
	choice := segment.Choices
	//создание контекста с таймаутом для изменения данных
	//TODO выставить таймер для всей операции, но пока не получится из-за ИИ(долгое выполнение)
	ctxTimeout, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	// Создание транзакции для консистентности данных
	tx, err := s.txManager.BeginTx(ctxTimeout)
	if err != nil {
		s.Logger.ZapLogger.Error("BeginTx", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	newUserMsg := models.NewStoryMessage(storyID, msg, "user")
	newAssistantMsg := models.NewStoryMessage(storyID, narrative, "assistant")

	// Сохраняем сообщения
	err = s.msgDatabase.AddStoryMessages(ctxTimeout, tx, []*models.StoryMessage{newUserMsg, newAssistantMsg})
	if err != nil {
		s.Logger.ZapLogger.Error("AddStoryMessages", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		rollbackErr := s.txManager.RollbackTx(context.Background(), tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		}
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Обновляем вариант
	choicesData, err := json.Marshal(choice)
	if err != nil {
		s.Logger.ZapLogger.Error("Marshal", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		rollbackErr := s.txManager.RollbackTx(context.Background(), tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		}
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	addingVariant := models.NewStoryVariant(storyID, "actions", choicesData)
	if err = s.variantDatabase.UpdateVariant(ctxTimeout, tx, addingVariant); err != nil {
		s.Logger.ZapLogger.Error("UpdateVariant", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		rollbackErr := s.txManager.RollbackTx(context.Background(), tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		}
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Обновляем дневной лимит
	err = s.updateOrAddDailyLimit(ctxTimeout, tx, limit, 1, trace, place)
	if err != nil {
		return nil, err
	}

	// Делаем подтверждение транзакции после изменения таблиц
	err = s.txManager.CommitTx(ctxTimeout, tx)
	if err != nil {
		s.Logger.ZapLogger.Error("CommitTx", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		rollbackErr := s.txManager.RollbackTx(context.Background(), tx)
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
