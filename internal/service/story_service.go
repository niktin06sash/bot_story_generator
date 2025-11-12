package service

import (
	"bot_story_generator/internal/config"
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/models"
	"bot_story_generator/internal/text_messages"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"context"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
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
	UpdateVariant(ctx context.Context, tx pgx.Tx, variant *models.StoryVariant) error
	GetActiveVariants(ctx context.Context, userID int64) ([]*models.StoryVariant, error)

	GetDailyLimit(ctx context.Context, userID int64) (*models.DailyLimit, error)
	AddDailyLimit(ctx context.Context, tx pgx.Tx, dailyLimit *models.DailyLimit) error
	UpdateCountDailyLimit(ctx context.Context, tx pgx.Tx, dailyLimit *models.DailyLimit) error
	UpdateLimitCountDailyLimit(ctx context.Context, dailyLimit *models.DailyLimit) error

	AddStoryMessages(ctx context.Context, tx pgx.Tx, msgs []*models.StoryMessage) error
	GetAllStorySegments(ctx context.Context, storyID int) ([]*models.StoryMessage, error)

	AddSubscription(ctx context.Context, subscription *models.Subscription) error
	GetActiveSubscriptions(ctx context.Context, userID int64) ([]*models.Subscription, error)
	GetStatusSubscription(ctx context.Context, payload string, userID int64) (*models.Subscription, error)
	RejectedPendingSubscription(ctx context.Context, payload string, userID int64) error
	PayedPendingSubscription(ctx context.Context, payload string, userID int64, start time.Time, end time.Time, changeID string) error

	GetAllSettings(ctx context.Context) ([]*models.Setting, error)
	GetSetting(ctx context.Context, key string) (*models.Setting, error)
	SetSetting(ctx context.Context, tx pgx.Tx, setting *models.Setting) error
}
type StoryAI interface {
	GetStructuredHeroes(ctx context.Context) (*models.FantasyCharacters, error)
	GenerateNextStorySegment(parctx context.Context, storyData []*models.StoryMessage) (*models.StoryNode, error)
}
type StoryCache interface {
	AddCreatedUser(ctx context.Context, userID int64) error
	CheckCreatedUser(ctx context.Context, userID int64) (bool, error)
	AddExceededLimit(ctx context.Context, userID int64) error
	DeleteExceededLimit(ctx context.Context, userID int64) error
	CheckExceededLimit(ctx context.Context, userID int64) (bool, error)

	SetSetting(ctx context.Context, key, value string) error
	GetSetting(ctx context.Context, key string) (string, error)
	GetAllSettings(ctx context.Context) (map[string]string, error)
	LoadCacheData(ctx context.Context, settings []*models.Setting) error
}
type StoryServiceImpl struct {
	DBStory StoryDatabase
	AIStory StoryAI
	CStory  StoryCache
	Logger  *logger.Logger
}

// TODO в конфиге добавь недостающие данные
func NewStoryService(cfg *config.Config, db StoryDatabase, ai StoryAI, cache StoryCache, logger *logger.Logger) *StoryServiceImpl {
	return &StoryServiceImpl{
		DBStory: db,
		AIStory: ai,
		CStory:  cache,
		Logger:  logger,
	}
}

func (s *StoryServiceImpl) CreateStory(ctx context.Context, userID int64) ([]string, error) {
	place := "CreateStory"
	// Проверяем, нет ли активных историй у пользователя в данный момент
	stories, err := s.DBStory.GetActiveStories(ctx, userID)
	if err != nil {
		s.Logger.ZapLogger.Error("GetActiveStories", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	if len(stories) > 1 {
		s.Logger.ZapLogger.Error("GetActiveStories", zap.Error(fmt.Errorf("server: more than one active story found")), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	if len(stories) > 0 {
		s.Logger.ZapLogger.Warn("GetActiveStories", zap.Error(errors.New("client: user already has an active history")), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorUserActiveStory)
	}
	// Проверяем, есть ли дневные ходы у пользователя для создания новой истории + подписка
	limit, err := s.checkDailyLimits(ctx, userID, place)
	if err != nil {
		return nil, err
	}
	// Запрос в ИИ
	fantasyCharacters, err := s.AIStory.GetStructuredHeroes(ctx)
	if err != nil {
		s.Logger.ZapLogger.Error("GetStructuredHeroes", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	if len(fantasyCharacters.Characters) == 0 {
		s.Logger.ZapLogger.Error("GetStructuredHeroes", zap.Error(errors.New("empty response from AI")), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	data, err := json.Marshal(fantasyCharacters)
	if err != nil {
		s.Logger.ZapLogger.Error("Marshal", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	// Создание транзакции для консистентности данных
	//создание контекста с таймаутом для изменения данных
	//TODO выставить таймер для всей операции, но пока не получится из-за ИИ(долгое выполнение)
	ctxTimeout, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	tx, err := s.DBStory.BeginTx(ctxTimeout)
	if err != nil {
		s.Logger.ZapLogger.Error("BeginTx", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Создаем историю с пустыми данными(так как ждем выбор в следующем действии пользователя)
	story := models.NewStory(userID, nil)
	storyId, err := s.DBStory.AddStory(ctxTimeout, tx, story)
	if err != nil {
		s.Logger.ZapLogger.Error("AddStory", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		//в случае отмены контекста(при завершении исполнения у нас может не сделаться rollback или commit транзакции - возможное решение использовать отдельный контекст для данных операций)
		rollbackErr := s.DBStory.RollbackTx(context.Background(), tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("userID", userID), zap.Any("place", place))
		}
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Создаем начальный вариант с данными из ИИ
	variant := models.NewStoryVariant(storyId, "characters", data)
	err = s.DBStory.AddVariant(ctxTimeout, tx, variant)
	if err != nil {
		s.Logger.ZapLogger.Error("AddVariant", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		rollbackErr := s.DBStory.RollbackTx(context.Background(), tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("userID", userID), zap.Any("place", place))
		}
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Создаем начальный дневной лимит или обновляем(он будет включать в себя как действия с созданием новых историй, так и последующий выбор действий)
	err = s.updateOrAddDailyLimit(ctxTimeout, tx, limit, 2, place)
	if err != nil {
		return nil, err
	}

	// Делаем подтверждение транзакции после изменения таблиц(+запись в истории, варианты, лимиты)
	err = s.DBStory.CommitTx(ctxTimeout, tx)
	if err != nil {
		s.Logger.ZapLogger.Error("CommitTx", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		rollbackErr := s.DBStory.RollbackTx(context.Background(), tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("userID", userID), zap.Any("place", place))
		}
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	//3 лог
	s.Logger.ZapLogger.Info("Story created successfully", zap.Any("userID", userID), zap.Any("place", place))
	return text_messages.NewChouseHero(fantasyCharacters), nil
}

func (s *StoryServiceImpl) UserChoice(ctx context.Context, userID int64, num string) ([]string, error) {
	place := "UserChoice"
	number_choice, err := strconv.Atoi(num)
	if err != nil || number_choice < 1 {
		s.Logger.ZapLogger.Error("Invalid user choice", zap.Error(err), zap.Any("userID", userID), zap.String("choice", num), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	// Получаем варианты (последний активный storyVariant для пользователя)
	variants, err := s.DBStory.GetActiveVariants(ctx, userID)
	if err != nil {
		s.Logger.ZapLogger.Error("GetActiveVariants", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
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
	// Проверяем, есть ли дневные ходы у пользователя для создания новой истории + подписка
	limit, err := s.checkDailyLimits(ctx, userID, place)
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
			s.Logger.ZapLogger.Error("Unmarshal", zap.Error(err), zap.Any("userID", userID))
			return nil, errors.New(text_messages.TextErrorCreateTask)
		}
		userVariant := fantasyCharacters.Characters[number_choice-1]
		msg = text_messages.CreateHeroMessage(&userVariant)
		//3 лог
		s.Logger.ZapLogger.Info("Fetched story variant", zap.Any("variant", userVariant), zap.Any("userID", userID), zap.Any("place", place))
	case "actions":
		var choices []string
		if err := json.Unmarshal(variant.Data, &choices); err != nil {
			s.Logger.ZapLogger.Error("Unmarshal", zap.Error(err), zap.Any("userID", userID))
			return nil, errors.New(text_messages.TextErrorCreateTask)
		}
		userVariant := models.Extension{Narrative: choices[number_choice-1]}
		msg = text_messages.CreateExtensionMessage(&userVariant)
		//3 лог
		s.Logger.ZapLogger.Info("Fetched action variant", zap.Any("variant", userVariant), zap.Any("userID", userID), zap.Any("place", place))
	default:
		s.Logger.ZapLogger.Error("Unknown variant type", zap.String("type", variant.Type), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	// Получаем все сегменты истории
	allStory, err := s.DBStory.GetAllStorySegments(ctx, storyID)
	if err != nil {
		s.Logger.ZapLogger.Error("GetAllStorySegments", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Добавляем выбор пользователя в формат истории, в бд добавляем потом, во время транзакции
	storySegment := &models.StoryMessage{Data: msg, Type: "user"}
	allStory = append(allStory, storySegment)

	// Генериуем ответ ии
	segment, err := s.AIStory.GenerateNextStorySegment(ctx, allStory)
	if err != nil {
		s.Logger.ZapLogger.Error("GenerateNextStorySegment", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	narrative := segment.Narrative
	choice := segment.Choices
	//создание контекста с таймаутом для изменения данных
	//TODO выставить таймер для всей операции, но пока не получится из-за ИИ(долгое выполнение)
	ctxTimeout, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	// Создание транзакции для консистентности данных
	tx, err := s.DBStory.BeginTx(ctxTimeout)
	if err != nil {
		s.Logger.ZapLogger.Error("BeginTx", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	newUserMsg := models.NewStoryMessage(storyID, msg, "user")
	newAssistantMsg := models.NewStoryMessage(storyID, narrative, "assistant")

	// Сохраняем сообщения
	err = s.DBStory.AddStoryMessages(ctxTimeout, tx, []*models.StoryMessage{newUserMsg, newAssistantMsg})
	if err != nil {
		s.Logger.ZapLogger.Error("AddStoryMessages", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		rollbackErr := s.DBStory.RollbackTx(context.Background(), tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("userID", userID), zap.Any("place", place))
		}
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Обновляем вариант
	choicesData, err := json.Marshal(choice)
	if err != nil {
		s.Logger.ZapLogger.Error("Marshal", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		rollbackErr := s.DBStory.RollbackTx(context.Background(), tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("userID", userID), zap.Any("place", place))
		}
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	addingVariant := models.NewStoryVariant(storyID, "actions", choicesData)
	if err = s.DBStory.UpdateVariant(ctxTimeout, tx, addingVariant); err != nil {
		s.Logger.ZapLogger.Error("UpdateVariant", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		rollbackErr := s.DBStory.RollbackTx(context.Background(), tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("userID", userID), zap.Any("place", place))
		}
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Обновляем дневной лимит
	err = s.updateOrAddDailyLimit(ctxTimeout, tx, limit, 1, place)
	if err != nil {
		return nil, err
	}

	// Делаем подтверждение транзакции после изменения таблиц
	err = s.DBStory.CommitTx(ctxTimeout, tx)
	if err != nil {
		s.Logger.ZapLogger.Error("CommitTx", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		rollbackErr := s.DBStory.RollbackTx(context.Background(), tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("userID", userID), zap.Any("place", place))
		}
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Формируем ответ
	resp := text_messages.TextNarrativeWithChoices(narrative, choice)
	//4 лог
	s.Logger.ZapLogger.Info("User's choice made successfully", zap.Any("userID", userID), zap.Any("place", place))
	return []string{msg, resp}, nil
}

func (s *StoryServiceImpl) CreateUser(ctx context.Context, userID int64) (string, error) {
	place := "CreateUser"
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	isExist, err := s.CStory.CheckCreatedUser(ctxTimeout, userID)
	if err != nil {
		s.Logger.ZapLogger.Warn("CheckCreatedUser", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
	} else if isExist {
		//3 лог
		s.Logger.ZapLogger.Info("CheckCreatedUser User is already created. Returning response", zap.Any("userID", userID), zap.Any("place", place))
		return "", errors.New(text_messages.TextGreeting)
	} else if !isExist {
		//3 лог
		s.Logger.ZapLogger.Info("CheckCreatedUser Created user not in cache. Trying creating in database...", zap.Any("userID", userID), zap.Any("place", place))
	}
	user := models.NewUser(userID)
	err = s.DBStory.AddUser(ctxTimeout, user)
	if err != nil {
		if strings.HasPrefix(err.Error(), "client: ") {
			s.Logger.ZapLogger.Warn("AddUser", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
			err := s.CStory.AddCreatedUser(ctxTimeout, userID)
			if err != nil {
				s.Logger.ZapLogger.Warn("AddCreatedUser", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
			}
			return "", errors.New(text_messages.TextGreeting)
		}
		s.Logger.ZapLogger.Error("AddUser", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		return "", errors.New(text_messages.TextErrorCreateTask)
	}
	//4 лог
	s.Logger.ZapLogger.Info("User created successfully", zap.Any("userID", userID), zap.Any("place", place))
	return text_messages.TextGreeting, nil
}

func (s *StoryServiceImpl) StopStory(ctx context.Context, userID int64) (string, error) {
	place := "StopStory"
	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	stories, err := s.DBStory.GetActiveStories(ctxTimeout, userID)
	if err != nil {
		s.Logger.ZapLogger.Error("GetActiveStories", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		return "", errors.New(text_messages.TextErrorCreateTask)
	}
	if len(stories) > 1 {
		s.Logger.ZapLogger.Error("GetActiveStories", zap.Error(fmt.Errorf("server: more one active story found")), zap.Any("userID", userID), zap.Any("place", place))
		return "", errors.New(text_messages.TextErrorCreateTask)
	}
	if len(stories) == 0 {
		s.Logger.ZapLogger.Warn("GetActiveStories", zap.Error(fmt.Errorf("client: user already has not an active history")), zap.Any("userID", userID), zap.Any("place", place))
		return "", errors.New(text_messages.TextNoActiveStory)
	}
	//3 лог
	s.Logger.ZapLogger.Info("Active story checked successfully", zap.Any("userID", userID), zap.Any("place", place))
	return text_messages.TextStopActiveStory, nil
}

func (s *StoryServiceImpl) StopStoryChoice(ctx context.Context, userID int64, arg string) (string, error) {
	if arg == "❌" {
		return "", nil
	}
	place := "StopStoryChoice"
	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := s.DBStory.StopStory(ctxTimeout, userID)
	if err != nil {
		s.Logger.ZapLogger.Error("StopStory", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		return "", errors.New(text_messages.TextErrorCreateTask)
	}
	//3 лог
	s.Logger.ZapLogger.Info("Active story stopped successfully", zap.Any("userID", userID), zap.Any("place", place))
	return text_messages.TextSuccessStopStory, nil
}

func (s *StoryServiceImpl) ValidatePreCheckout(ctx context.Context, pd *models.PaymentData) error {
	place := "ValidatePreCheckout"
	ctxTimeout, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	price, err := s.getSubPrice(ctxTimeout, pd.UserID, place)
	if err != nil {
		return err
	}
	g, ctxTimeout := errgroup.WithContext(ctxTimeout)
	var subb *models.Subscription
	//параллельно делаем запросы для получения данных транзакции(если есть хоть одна ошибка - помечаем статус транзакции на rejected)
	g.Go(func() error {
		subscriptions, err := s.DBStory.GetActiveSubscriptions(ctxTimeout, pd.UserID)
		if err != nil {
			s.Logger.ZapLogger.Error("GetActiveSubscriptions", zap.Error(err), zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("place", place))
			return errors.New(text_messages.TextErrorProcessPayment)
		}
		if len(subscriptions) > 1 {
			s.Logger.ZapLogger.Error("GetActiveSubscriptions", zap.Error(fmt.Errorf("server: more than one active subscription found")), zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("place", place))
			return errors.New(text_messages.TextErrorProcessPayment)
		}
		if len(subscriptions) > 0 {
			s.Logger.ZapLogger.Warn("GetActiveSubscriptions", zap.Error(fmt.Errorf("client: user already has active subscription")), zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("place", place))
			return errors.New(text_messages.TextAlreadyActiveSubscription)
		}
		return nil
	})
	g.Go(func() error {
		sub, err := s.DBStory.GetStatusSubscription(ctxTimeout, pd.InvoicePayload, pd.UserID)
		if err != nil {
			if strings.HasPrefix(err.Error(), "client: ") {
				s.Logger.ZapLogger.Warn("GetStatusSubscription", zap.Error(err), zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("place", place))
				return errors.New(text_messages.InvalidPaymentData)
			}
			s.Logger.ZapLogger.Error("GetStatusSubscription", zap.Error(err), zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("place", place))
			return errors.New(text_messages.TextErrorProcessPayment)
		}
		subb = sub
		if sub.Status == "rejected" {
			s.Logger.ZapLogger.Warn("GetStatusSubscription", zap.Error(errors.New("Attempt to repeat send a rejected transaction")), zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("place", place))
			return errors.New(text_messages.InvalidPaymentData)
		}
		return nil
	})
	err = g.Wait()
	if err != nil {
		if subb.Status != "rejected" {
			rejerr := s.DBStory.RejectedPendingSubscription(ctxTimeout, pd.InvoicePayload, pd.UserID)
			if rejerr != nil {
				s.Logger.ZapLogger.Error("RejectedPendingSubscription", zap.Error(err), zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("place", place))
				return errors.New(text_messages.TextErrorProcessPayment)
			}
		}
		return err
	}
	//сверяем цены
	if price != pd.TotalAmount {
		s.Logger.ZapLogger.Warn("Check Subscription Price", zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("place", place))
		err = s.DBStory.RejectedPendingSubscription(ctxTimeout, pd.InvoicePayload, pd.UserID)
		if err != nil {
			s.Logger.ZapLogger.Error("RejectedPendingSubscription", zap.Error(err), zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("place", place))
			return errors.New(text_messages.TextErrorProcessPayment)
		}
		return errors.New(text_messages.InvalidPaymentData)
	}
	s.Logger.ZapLogger.Info("Subscription validated successfully", zap.Any("userID", pd.UserID), zap.Any("place", place))
	return nil
}

// Обработка команды покупки подписки
// Проверяем, что нет активной подписки + добавляем в бд pending у подписки
func (s *StoryServiceImpl) BuySubscription(ctx context.Context, userID int64) (*models.Subscription, error) {
	place := "BuySubscription"
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	subscriptions, err := s.DBStory.GetActiveSubscriptions(ctxTimeout, userID)
	if err != nil {
		s.Logger.ZapLogger.Error("GetActiveSubscriptions", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorProcessPayment)
	}
	if len(subscriptions) > 1 {
		s.Logger.ZapLogger.Error("GetActiveSubscriptions", zap.Error(fmt.Errorf("server: more than one active subscription found")), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorProcessPayment)
	}
	if len(subscriptions) > 0 {
		s.Logger.ZapLogger.Warn("GetActiveSubscriptions", zap.Error(fmt.Errorf("client: user already has active subscription")), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextAlreadyActiveSubscription)
	}
	status := "pending"

	// Есил будем добавлять другие типы подписок, то тут нужно будет менять currency и price в зависимости от типа
	// Например, можно будет передавать тип подписки в аргументах функции
	// и в зависимости от этого выбирать нужные параметры
	// Но пока у нас только один тип подписки, поэтому оставляем так
	currencySubscription := "XTR"

	nameSub := text_messages.NameBasicSubscription

	payload := fmt.Sprintf("%s_%s_%d_%d", nameSub, currencySubscription, userID, time.Now().Unix())
	price, err := s.getSubPrice(ctxTimeout, userID, place)
	if err != nil {
		return nil, err
	}
	sub := models.NewSubscription(userID, nameSub, payload, status, currencySubscription, price)
	err = s.DBStory.AddSubscription(ctxTimeout, sub)
	if err != nil {
		s.Logger.ZapLogger.Error("AddSubscription", zap.Error(err), zap.Any("userID", userID), zap.Any("payload", payload), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorProcessPayment)
	}
	s.Logger.ZapLogger.Info("Subscription pending successfully", zap.Any("userID", userID), zap.Any("payload", payload), zap.Any("place", place))
	return sub, nil
}
func (s *StoryServiceImpl) CommitSubscription(ctx context.Context, pd *models.PaymentData) error {
	place := "CommitSubscription"
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	// Подписка на 30 дней
	start := time.Now()
	end := start.AddDate(0, 0, 30)
	err := s.DBStory.PayedPendingSubscription(ctxTimeout, pd.InvoicePayload, pd.UserID, start, end, pd.ChargeID)
	if err != nil {
		s.Logger.ZapLogger.Error("UpdatePendingSubscription", zap.Error(err), zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("place", place))
		return errors.New(text_messages.TextErrorActivateSubscription)
	}
	premiumDayLimitStr, err := s.CStory.GetSetting(ctx, models.SettingKeyLimitPremiumDay)
	if err != nil {
		s.Logger.ZapLogger.Error("GetSetting", zap.Error(err), zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("place", place))
		return errors.New(text_messages.TextErrorActivateSubscription)
	}
	premiumDayLimit, convErr := strconv.Atoi(premiumDayLimitStr)
	if convErr != nil {
		s.Logger.ZapLogger.Error("Atoi limit.day.premium", zap.Error(convErr), zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("place", place))
		return errors.New(text_messages.TextErrorActivateSubscription)
	}
	err = s.CStory.DeleteExceededLimit(ctxTimeout, pd.UserID)
	if err != nil {
		s.Logger.ZapLogger.Error("DeleteExceededLimit", zap.Error(err), zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("place", place))
		return errors.New(text_messages.TextErrorActivateSubscription)
	}
	limit := models.NewDailyLimit(pd.UserID, 0, premiumDayLimit)
	err = s.DBStory.UpdateLimitCountDailyLimit(ctxTimeout, limit)
	if err != nil {
		s.Logger.ZapLogger.Error("UpdateLimitCountDailyLimit", zap.Error(err), zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("place", place))
		return errors.New(text_messages.TextErrorActivateSubscription)
	}
	s.Logger.ZapLogger.Info("Subscription commited successfully", zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("place", place))
	return nil
}

func (s *StoryServiceImpl) GetSubscriptionStatus(ctx context.Context, userID int64) (string, error) {
	place := "GetSubscriptionStatus"
	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	subscriptions, err := s.DBStory.GetActiveSubscriptions(ctxTimeout, userID)
	if err != nil {
		s.Logger.ZapLogger.Error("GetActiveSubscriptions", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		return "", errors.New(text_messages.TextErrorGetSubscriptionStatus)
	}
	if len(subscriptions) > 1 {
		s.Logger.ZapLogger.Error("GetActiveSubscriptions", zap.Error(fmt.Errorf("server: more than one active subscription found")), zap.Any("userID", userID), zap.Any("place", place))
		return "", errors.New(text_messages.TextErrorGetSubscriptionStatus)
	}
	if len(subscriptions) == 0 {
		return text_messages.CreateNoSubscriptionMessage(), nil
	}

	sub := subscriptions[0]
	typeSub, startData, endData := sub.Type, sub.StartDate, sub.EndDate
	s.Logger.ZapLogger.Info("Subscription received successfully", zap.Any("userID", userID), zap.Any("place", place))
	return text_messages.CreateSubscriptionStatusMessage(typeSub, startData, endData), nil
}

// SetSetting изменяет значение настройки (только для админов)
func (s *StoryServiceImpl) SetSetting(ctx context.Context, key string, value string, updatedBy int64) (string, error) {
	place := "SetSetting"
	if key == "" {
		s.Logger.ZapLogger.Warn("Empty Key", zap.Any("key", key), zap.Any("place", place))
		return "", errors.New(text_messages.TextErrorSettings)
	}
	switch key {
	case models.SettingKeyPriceBasicSubscription:
		price, err := strconv.Atoi(value)
		if err != nil || price <= 0 {
			s.Logger.ZapLogger.Warn("Invalid Price", zap.Any("key", key), zap.Any("place", place))
			return "", errors.New(text_messages.TextErrorSettings)
		}

	case models.SettingKeyLimitBaseDay:
		limit, err := strconv.Atoi(value)
		if err != nil || limit < 0 {
			s.Logger.ZapLogger.Warn("Invalid LimitDay", zap.Any("key", key), zap.Any("place", place))
			return "", errors.New(text_messages.TextErrorSettings)
		}
		//какой в жопу limit.day.adventure+, если там limit.day.premium
	case models.SettingKeyLimitPremiumDay:
		limit, err := strconv.Atoi(value)
		if err != nil || limit < 0 {
			s.Logger.ZapLogger.Warn("Invalid LimitDay", zap.Any("key", key), zap.Any("place", place))
			return "", errors.New(text_messages.TextErrorSettings)
		}
	default:
		s.Logger.ZapLogger.Warn("Unknown setting key", zap.Any("key", key), zap.Any("place", place))
		return "", errors.New(text_messages.TextErrorSettings)
	}
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := s.DBStory.BeginTx(ctxTimeout)
	if err != nil {
		s.Logger.ZapLogger.Error("BeginTx", zap.Error(err), zap.Any("key", key), zap.Any("place", place))
		return "", errors.New(text_messages.TextErrorSettings)
	}
	setting := models.NewSetting(key, value, updatedBy)
	err = s.DBStory.SetSetting(ctxTimeout, tx, setting)
	if err != nil {
		rollbackErr := s.DBStory.RollbackTx(context.Background(), tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("key", key), zap.Any("place", place))
		}
		return "", errors.New(text_messages.TextErrorSettings)
	}

	err = s.CStory.SetSetting(ctxTimeout, key, value)
	if err != nil {
		s.Logger.ZapLogger.Error("SetSetting", zap.Error(err), zap.Any("key", key), zap.Any("place", place))
		rollbackErr := s.DBStory.RollbackTx(context.Background(), tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("key", key), zap.Any("place", place))
		}
		return "", errors.New(text_messages.TextErrorSettings)
	}

	if err := s.DBStory.CommitTx(ctxTimeout, tx); err != nil {
		s.Logger.ZapLogger.Error("CommitTx", zap.Error(err), zap.Any("key", key), zap.Any("place", place))
		rollbackErr := s.DBStory.RollbackTx(context.Background(), tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("key", key), zap.Any("place", place))
		}
		return "", errors.New(text_messages.TextErrorSettings)
	}

	s.Logger.ZapLogger.Info("Setting updated successfully", zap.Any("key", key), zap.Any("place", place))

	return text_messages.TextSuccessSetSetting, nil
}

func (s *StoryServiceImpl) ViewSetting(ctx context.Context) (string, error) {
	place := "ViewSetting"
	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cacheSettings, err := s.getAllSettingsFromCache(ctxTimeout)
	if err != nil {
		s.Logger.ZapLogger.Error("Failed to get settings from cache", zap.Error(err), zap.Any("place", place))
		return "", errors.New(text_messages.TextErrorSettings)
	}

	dbSettings, err := s.getAllSettingsFromDB(ctxTimeout)
	if err != nil {
		s.Logger.ZapLogger.Error("Failed to get settings from database", zap.Error(err), zap.Any("place", place))
		return "", errors.New(text_messages.TextErrorSettings)
	}

	dbSettingsMap := make(map[string]string)
	for _, setting := range dbSettings {
		if setting == nil {
			continue
		}
		dbSettingsMap[setting.Key] = setting.Value
	}

	formattedMessage := text_messages.FormatSettingsComparison(cacheSettings, dbSettingsMap)
	s.Logger.ZapLogger.Info("Setting received successfully", zap.Any("place", place))
	return formattedMessage, nil
}

func (s *StoryServiceImpl) RebootCacheData(ctx context.Context) error {
	place := "RebootCacheData"
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	settings, err := s.DBStory.GetAllSettings(ctxTimeout)
	if err != nil {
		s.Logger.ZapLogger.Error("GetAllSettings", zap.Error(err), zap.Any("place", place))
		return errors.New(text_messages.TextErrorSettings)
	}
	err = s.CStory.LoadCacheData(ctxTimeout, settings)
	if err != nil {
		s.Logger.ZapLogger.Error("LoadCacheData", zap.Error(err), zap.Any("place", place))
		return errors.New(text_messages.TextErrorSettings)
	}
	s.Logger.ZapLogger.Info("Setting rebooted successfully", zap.Any("place", place))
	return nil
}
