package service

import (
	"bot_story_generator/internal/config"
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/models"
	"time"

	"context"

	"github.com/jackc/pgx/v5"
)

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
	StopStory(ctx context.Context, tx pgx.Tx, userID int64) error
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

//go:generate mockgen -source=service_implementation.go -destination=mocks/mock.go
type Service struct {
	AdminService   *AdminServiceImpl
	SettingService *SettingServiceImpl
	UserService    *UserServiceImpl
	StoryService   *StoryServiceImpl
	SubService     *SubscriptionServiceImpl
}

func NewService(cfg *config.Config, txMan TxManager, userDB UserDatabase, storyDB StoryDatabase, varDB VariantDatabase, daylimitDB DailyLimitDatabase, msgDB MessageDatabase, subDB SubscriptionDatabase,
	settingDB SettingDatabase, storyAI StoryAI, limitCache DailyLimitCache, settingCache SettingCache, userCache UserCache, logger *logger.Logger) *Service {
	return &Service{
		AdminService:   NewAdminService(subDB, logger),
		SettingService: NewSettingService(settingCache, settingDB, txMan, logger),
		UserService:    NewUserService(userDB, userCache, logger),
		StoryService:   NewStoryService(settingCache, subDB, limitCache, daylimitDB, storyDB, storyAI, txMan, varDB, msgDB, logger),
		SubService:     NewSubscriptionService(settingCache, subDB, limitCache, daylimitDB, logger),
	}
}
