package repository

import (
	"bot_story_generator/internal/cache"
	"context"
	"fmt"
	"time"
)

type StoryCacheImpl struct {
	cacheclient *cache.CacheObject
}

func NewStoryCache(cache *cache.CacheObject) *StoryCacheImpl {
	return &StoryCacheImpl{
		cacheclient: cache,
	}
}
func (s *StoryCacheImpl) AddExceededLimit(ctx context.Context, userID int64) error {
	key := fmt.Sprintf(s.cacheclient.ExceededLimitKey, userID)
	now := time.Now().UTC()
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, time.UTC)
	return s.cacheclient.Connect.Set(ctx, key, "true", endOfDay.Sub(now)).Err()
}
func (s *StoryCacheImpl) CheckExceededLimit(ctx context.Context, userID int64) (bool, error) {
	key := fmt.Sprintf(s.cacheclient.ExceededLimitKey, userID)
	exists, err := s.cacheclient.Connect.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return exists == 1, nil
}

// в дальнейшем можно будет хранить типо профиля или что-то такое, пока просто флаг присутствия после регистрации
// пусть сутки хранятся значения
func (s *StoryCacheImpl) AddCreatedUser(ctx context.Context, userID int64) error {
	key := fmt.Sprintf(s.cacheclient.UserCreatedKey, userID)
	return s.cacheclient.Connect.Set(ctx, key, "true", 24*time.Hour).Err()
}
func (s *StoryCacheImpl) CheckCreatedUser(ctx context.Context, userID int64) (bool, error) {
	key := fmt.Sprintf(s.cacheclient.UserCreatedKey, userID)
	exists, err := s.cacheclient.Connect.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return exists == 1, nil
}
