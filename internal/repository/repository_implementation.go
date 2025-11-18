package repository

import (
	"bot_story_generator/internal/cache"
	"bot_story_generator/internal/database"
)

type Repository struct {
	DailyLimitDatabase   *DailyLimitDatabaseImpl
	DailyLimitCache      *DailyLimitCacheImpl
	MessageDatabase      *MessageDatabaseImpl
	SettingCache         *SettingCacheImpl
	SettingDatabase      *SettingDatabaseImpl
	StoryDatabase        *StoryDatabaseImpl
	SubscriptionDatabase *SubscriptionDatabaseImpl
	TxManager            *TxManagerImpl
	UserCache            *UserCacheImpl
	UserDatabase         *UserDatabaseImpl
	VariantDatabase      *VariantDatabaseImpl
}

func NewRepository(db *database.DBObject, cache *cache.CacheObject) *Repository {
	return &Repository{
		DailyLimitDatabase:   NewDailyLimitDatabase(db),
		DailyLimitCache:      NewDailyLimitCache(cache),
		MessageDatabase:      NewMessageDatabase(db),
		SettingCache:         NewSettingCache(cache),
		SettingDatabase:      NewSettingDatabase(db),
		StoryDatabase:        NewStoryDatabase(db),
		SubscriptionDatabase: NewSubscriptionDatabase(db),
		TxManager:            NewTxManager(db),
		UserCache:            NewUserCache(cache),
		UserDatabase:         NewUserDatabase(db),
		VariantDatabase:      NewVariantDatabase(db),
	}
}
