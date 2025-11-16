package service

import (
	"bot_story_generator/internal/config"
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/models"
	"time"

	"context"

	"github.com/jackc/pgx/v5"
)

//go:generate mockgen -source=story_service.go -destination=mocks/mock.go
type TxManager interface {
	BeginTx(ctx context.Context) (pgx.Tx, error)
	RollbackTx(ctx context.Context, tx pgx.Tx) error
	CommitTx(ctx context.Context, tx pgx.Tx) error
}
type UserDatabase interface {
	AddUser(ctx context.Context, user *models.User) error
}
type StoryDatabase interface {
	GetActiveStories(ctx context.Context, userID int64) ([]*models.Story, error)
	StopStory(ctx context.Context, userID int64) error
	AddStory(ctx context.Context, tx pgx.Tx, story *models.Story) (int, error)
}
type VariantDatabase interface {
	AddVariant(ctx context.Context, tx pgx.Tx, variant *models.StoryVariant) error
	UpdateVariant(ctx context.Context, tx pgx.Tx, variant *models.StoryVariant) error
	GetActiveVariants(ctx context.Context, userID int64) ([]*models.StoryVariant, error)
}
type DailyLimitDatabase interface {
	GetDailyLimit(ctx context.Context, userID int64) (*models.DailyLimit, error)
	AddDailyLimit(ctx context.Context, tx pgx.Tx, dailyLimit *models.DailyLimit) error
	UpdateCountDailyLimit(ctx context.Context, tx pgx.Tx, dailyLimit *models.DailyLimit) error
	UpdateLimitCountDailyLimit(ctx context.Context, dailyLimit *models.DailyLimit) error
}
type MessageDatabase interface {
	AddStoryMessages(ctx context.Context, tx pgx.Tx, msgs []*models.StoryMessage) error
	GetAllStorySegments(ctx context.Context, storyID int) ([]*models.StoryMessage, error)
}
type SubscriptionDatabase interface {
	AddSubscription(ctx context.Context, subscription *models.Subscription) error
	UpdateSubscription(ctx context.Context, subscription *models.Subscription) error
	GetActiveSubscriptions(ctx context.Context, userID int64) ([]*models.Subscription, error)
	GetStatusSubscription(ctx context.Context, payload string, userID int64) (*models.Subscription, error)
	RejectedPendingSubscription(ctx context.Context, payload string, userID int64) error
	PayedPendingSubscription(ctx context.Context, payload string, userID int64, start time.Time, end time.Time, changeID string) error
}
type SettingDatabase interface {
	GetAllSettings(ctx context.Context) ([]*models.Setting, error)
	GetSetting(ctx context.Context, key string) (*models.Setting, error)
	SetSetting(ctx context.Context, tx pgx.Tx, setting *models.Setting) error
}
type StoryAI interface {
	GetStructuredHeroes(ctx context.Context) (*models.FantasyCharacters, error)
	GenerateNextStorySegment(parctx context.Context, storyData []*models.StoryMessage) (*models.StoryNode, error)
}
type DailyLimitCache interface {
	AddExceededLimit(ctx context.Context, userID int64) error
	DeleteExceededLimit(ctx context.Context, userID int64) error
	CheckExceededLimit(ctx context.Context, userID int64) (bool, error)
}
type SettingCache interface {
	SetSetting(ctx context.Context, key, value string) error
	GetSetting(ctx context.Context, key string) (string, error)
	GetAllSettings(ctx context.Context) (map[string]string, error)
	LoadCacheData(ctx context.Context, settings []*models.Setting) error
}
type UserCache interface {
	AddCreatedUser(ctx context.Context, userID int64) error
	CheckCreatedUser(ctx context.Context, userID int64) (bool, error)
}
type ServiceImpl struct {
	txManager        TxManager
	userDatabase     UserDatabase
	storyDatabase    StoryDatabase
	variantDatabase  VariantDatabase
	daylimitDatabase DailyLimitDatabase
	msgDatabase      MessageDatabase
	subDatabase      SubscriptionDatabase
	settingDatabase  SettingDatabase
	storyAI          StoryAI
	daylimitCache    DailyLimitCache
	settingCache     SettingCache
	userCache        UserCache
	Logger           *logger.Logger
}

func NewService(cfg *config.Config, txMan TxManager, userDB UserDatabase, storyDB StoryDatabase, varDB VariantDatabase, daylimitDB DailyLimitDatabase, msgDB MessageDatabase, subDB SubscriptionDatabase,
	settingDB SettingDatabase, storyAI StoryAI, limitCache DailyLimitCache, settingCache SettingCache, userCache UserCache, logger *logger.Logger) *ServiceImpl {
	return &ServiceImpl{
		txManager:        txMan,
		userDatabase:     userDB,
		storyDatabase:    storyDB,
		variantDatabase:  varDB,
		daylimitDatabase: daylimitDB,
		msgDatabase:      msgDB,
		subDatabase:      subDB,
		settingDatabase:  settingDB,
		storyAI:          storyAI,
		daylimitCache:    limitCache,
		settingCache:     settingCache,
		userCache:        userCache,
		Logger:           logger,
	}
}
