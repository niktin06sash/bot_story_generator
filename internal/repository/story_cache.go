package repository

import (
	"bot_story_generator/internal/cache"
	"bot_story_generator/internal/models"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
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
func (s *StoryCacheImpl) DeleteExceededLimit(ctx context.Context, userID int64) error {
	key := fmt.Sprintf(s.cacheclient.ExceededLimitKey, userID)
	_, err := s.cacheclient.Connect.Del(ctx, key).Result()
	if err != nil {
		return err
	}
	return err
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

// загрузка конфигурации кэша из бд
func (s *StoryCacheImpl) LoadCacheData(ctx context.Context, settings []*models.Setting) error {
	if settings == nil {
		return errors.New("settings is nil")
	}
	allName := models.NameSettingKeys()
	// Записываем каждую настройку в Redis с ключом settings:%s<key>
	for _, st := range settings {
		key := fmt.Sprintf(s.cacheclient.SettingsKey, st.Key)
		// Сохраняем значение как строку без TTL (пока)
		err := s.cacheclient.Connect.Set(ctx, key, st.Value, 0).Err()
		if err != nil {
			return err
		}
	}

	// Проверяем, что все ожидаемые ключи присутствуют в кэше
	var missing []string
	for _, name := range allName {
		key := fmt.Sprintf(s.cacheclient.SettingsKey, name)
		exists, err := s.cacheclient.Connect.Exists(ctx, key).Result()
		if err != nil {
			return err
		}
		if exists == 0 {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("some settings are missing in cache: %v", missing)
	}

	return nil
}

// SetSetting сохраняет отдельную настройку в кэше
func (s *StoryCacheImpl) SetSetting(ctx context.Context, key, value string) error {
	redisKey := fmt.Sprintf(s.cacheclient.SettingsKey, key)
	if err := s.cacheclient.Connect.Set(ctx, redisKey, value, 0).Err(); err != nil {
		return err
	}
	return nil
}

func (s *StoryCacheImpl) GetAllSettings(ctx context.Context) (map[string]string, error) {
	// Получаем все возможные ключи настроек
	keys := models.NameSettingKeys()
	res := make(map[string]string, len(keys))
	var missing []string

	for _, name := range keys {
		redisKey := fmt.Sprintf(s.cacheclient.SettingsKey, name)
		val, err := s.cacheclient.Connect.Get(ctx, redisKey).Result()
		if err != nil {
			if err == redis.Nil {
				missing = append(missing, name)
				continue
			}
			return nil, err
		}
		res[name] = val
	}

	if len(missing) > 0 {
		return res, fmt.Errorf("missing settings in cache: %v", missing)
	}
	return res, nil
}

func (s *StoryCacheImpl) GetSetting(ctx context.Context, key string) (string, error) {
	redisKey := fmt.Sprintf(s.cacheclient.SettingsKey, key)
	val, err := s.cacheclient.Connect.Get(ctx, redisKey).Result()
	if err != nil {
		return "", err
	}
	return val, nil
}
