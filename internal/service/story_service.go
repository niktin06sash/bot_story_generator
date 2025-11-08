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
	UpdateDailyLimit(ctx context.Context, tx pgx.Tx, dailyLimit *models.DailyLimit) error

	AddStoryMessages(ctx context.Context, tx pgx.Tx, msgs []*models.StoryMessage) error
	GetAllStorySegments(ctx context.Context, storyID int) ([]*models.StoryMessage, error)

	AddSubscription(ctx context.Context, subscription *models.Subscription) error
	GetActiveSubscriptions(ctx context.Context, userID int64) ([]*models.Subscription, error)
	GetPendingSubscription(ctx context.Context, payload string, userID int64) (*models.Subscription, error)
	UpdatePendingSubscription(ctx context.Context, payload string, userID int64, start time.Time, end time.Time, changeID string) error
}
type StoryAI interface {
	GetStructuredHeroes(ctx context.Context) (*models.FantasyCharacters, error)
	GenerateNextStorySegment(parctx context.Context, storyData []*models.StoryMessage) (*models.StoryNode, error)
}
type StoryCache interface {
	AddCreatedUser(ctx context.Context, userID int64) error
	CheckCreatedUser(ctx context.Context, userID int64) (bool, error)
	AddExceededLimit(ctx context.Context, userID int64) error
	CheckExceededLimit(ctx context.Context, userID int64) (bool, error)
}
type StoryServiceImpl struct {
	priceBasicSubscription int
	currencySubscription   string
	baseDayLimit           int
	premiumDayLimit        int
	nameSub                string

	DBStory StoryDatabase
	AIStory StoryAI
	CStory  StoryCache
	Logger  *logger.Logger
}

// TODO в конфиге добавь недостающие данные
func NewStoryService(cfg *config.Config, db StoryDatabase, ai StoryAI, cache StoryCache, logger *logger.Logger) *StoryServiceImpl {
	return &StoryServiceImpl{priceBasicSubscription: cfg.Setting.PriceBasicSubscription, currencySubscription: cfg.Setting.Currency,
		baseDayLimit:    cfg.Setting.TokenDayLimit,
		premiumDayLimit: cfg.Setting.PremiumTokenDayLimit,
		nameSub:         cfg.Setting.NameBasicSubscription,
		DBStory:         db, AIStory: ai, CStory: cache, Logger: logger}
}

func (s *StoryServiceImpl) CreateStory(ctx context.Context, userID int64) ([]string, error) {
	place := "CreateStory"
	// Проверяем, есть ли дневные ходы у пользователя для создания новой истории
	//TODO проверка на активную подписку
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
	if len(stories) > 1 {
		s.Logger.ZapLogger.Error("GetActiveStories", zap.Error(fmt.Errorf("server: more than one active story found")), zap.Any("userID", userID), zap.Any("place", place))
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

	// Создаем начальный дневной лимит или обновляем(он будет включать в себя как действия с созданием новых историй, так и последующий выбор действий)
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
	// Проверяем, есть ли дневные ходы у пользователя для создания новой истории
	limit, err := s.checkDailyLimits(ctx, userID, place)
	if err != nil {
		return nil, err
	}

	// Получаем варианты (последний актвный storyVariant для пользователя)
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

	//TODO генерим ответ ии - вынести в другую функцию потом
	//* А зачем выносить в другую функцию? Вроде немного места занимает?

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

	// Создание транзакции для консистентности данных
	tx, err := s.DBStory.BeginTx(ctx)
	if err != nil {
		s.Logger.ZapLogger.Error("BeginTx", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	newUserMsg := models.NewStoryMessage(storyID, msg, "user")
	newAssistantMsg := models.NewStoryMessage(storyID, narrative, "assistant")

	// Сохраняем сообщения
	err = s.DBStory.AddStoryMessages(ctx, tx, []*models.StoryMessage{newUserMsg, newAssistantMsg})
	if err != nil {
		s.Logger.ZapLogger.Error("AddStoryMessages", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		rollbackErr := s.DBStory.RollbackTx(ctx, tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("userID", userID), zap.Any("place", place))
		}
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Обновляем вариант
	choicesData, err := json.Marshal(choice)
	if err != nil {
		s.Logger.ZapLogger.Error("Marshal", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		rollbackErr := s.DBStory.RollbackTx(ctx, tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("userID", userID), zap.Any("place", place))
		}
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	addingVariant := models.NewStoryVariant(storyID, "actions", choicesData)
	if err = s.DBStory.UpdateVariant(ctx, tx, addingVariant); err != nil {
		s.Logger.ZapLogger.Error("UpdateVariant", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		rollbackErr := s.DBStory.RollbackTx(ctx, tx)
		if rollbackErr != nil {
			s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("userID", userID), zap.Any("place", place))
		}
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Обновляем дневной лимит
	err = s.updateOrAddDailyLimit(ctx, tx, limit, 1, place)
	if err != nil {
		return nil, err
	}

	// Делаем подтверждение транзакции после изменения таблиц
	err = s.DBStory.CommitTx(ctx, tx)
	if err != nil {
		s.Logger.ZapLogger.Error("CommitTx", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}

	// Формируем ответ
	resp := text_messages.TextNarrativeWithChoices(narrative, choice)
	//4 лог
	s.Logger.ZapLogger.Info("User's choice made successfully", zap.Any("userID", userID), zap.Any("place", place))
	return []string{msg, resp}, nil
}

func (s *StoryServiceImpl) CreateUser(ctx context.Context, userID int64) ([]string, error) {
	place := "CreateUser"
	isExist, err := s.CStory.CheckCreatedUser(ctx, userID)
	if err != nil {
		s.Logger.ZapLogger.Warn("CheckCreatedUser", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
	} else if isExist {
		//3 лог
		s.Logger.ZapLogger.Info("CheckCreatedUser User is already created. Returning response", zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextGreeting)
	} else if !isExist {
		//3 лог
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
	//4 лог
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
	//3 лог
	s.Logger.ZapLogger.Info("Active story checked successfully", zap.Any("userID", userID), zap.Any("place", place))
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
	//3 лог
	s.Logger.ZapLogger.Info("Active story stopped successfully", zap.Any("userID", userID), zap.Any("place", place))
	return []string{text_messages.TextSuccessStopStory}, nil
}

func (s *StoryServiceImpl) ValidatePreCheckout(ctx context.Context, pd models.PaymentData) error {
	place := "ValidatePreCheckout"
	subscriptions, err := s.DBStory.GetActiveSubscriptions(ctx, pd.UserID)
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
	sub, err := s.DBStory.GetPendingSubscription(ctx, pd.InvoicePayload, pd.UserID)
	if err != nil {
		if strings.HasPrefix(err.Error(), "client: ") {
			//сообщение об отсутствии данных pending sub
			s.Logger.ZapLogger.Warn("GetPendingSubscription", zap.Error(err), zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("place", place))
			return errors.New(text_messages.InvalidPaymentData)
		}
		s.Logger.ZapLogger.Error("GetPendingSubscription", zap.Error(err), zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("place", place))
		return errors.New(text_messages.TextErrorProcessPayment)
	}
	if sub.Currency != pd.Currency || sub.Price != pd.TotalAmount {
		//сообщение о некорректых данных при оплате
		return errors.New(text_messages.InvalidPaymentData)
	}
	return nil
}

// Обработка команды покупки подписки
// Проверяем, что нет активной подписки + добавляем в бд pending у подписки
func (s *StoryServiceImpl) BuySubscription(ctx context.Context, userID int64) (*models.Subscription, error) {
	place := "BuySubscription"
	subscriptions, err := s.DBStory.GetActiveSubscriptions(ctx, userID)
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
	payload := fmt.Sprintf("%s_%s_%d_%d", s.nameSub, s.currencySubscription, userID, time.Now().Unix())
	sub := models.NewSubscription(userID, s.nameSub, payload, status, s.currencySubscription, s.priceBasicSubscription)
	err = s.DBStory.AddSubscription(ctx, sub)
	if err != nil {
		s.Logger.ZapLogger.Error("AddSubscription", zap.Error(err), zap.Any("userID", userID), zap.Any("place", place))
		return nil, errors.New(text_messages.TextErrorProcessPayment)
	}
	s.Logger.ZapLogger.Info("Subscription pending successfully", zap.Any("userID", userID), zap.Any("place", place))
	return sub, nil
}
func (s *StoryServiceImpl) CommitSubscription(ctx context.Context, pd models.PaymentData) error {
	//TODO добавить логику для изменения дневного лимита
	place := "CommitSubscription"
	start := time.Now()
	end := time.Now().AddDate(0, 0, s.premiumDayLimit)
	err := s.DBStory.UpdatePendingSubscription(ctx, pd.InvoicePayload, pd.UserID, start, end, pd.ChargeID)
	if err != nil {
		s.Logger.ZapLogger.Error("UpdatePendingSubscription", zap.Error(err), zap.Any("userID", pd.UserID), zap.Any("payload", pd.InvoicePayload), zap.Any("place", place))
		return errors.New(text_messages.TextErrorActivateSubscription)
	}
	return nil
}
