package service

import (
	"bot_story_generator/internal/models"
	"bot_story_generator/internal/text_messages"
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

func (s *StoryServiceImpl) updateOrAddDailyLimit(ctx context.Context, tx pgx.Tx, limit *models.DailyLimit, step int, LogPlace string) error {
	var err error
	if limit.Count == 0 {
		limit.Count += step
		err = s.DBStory.AddDailyLimit(ctx, tx, limit)
		if err != nil {
			s.Logger.ZapLogger.Error("AddDailyLimit", zap.Error(err), zap.Any("userID", limit.UserID), zap.Any("place", LogPlace))
			rollbackErr := s.DBStory.RollbackTx(ctx, tx)
			if rollbackErr != nil {
				s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("userID", limit.UserID), zap.Any("place", LogPlace))
			}
			return errors.New(text_messages.TextErrorCreateTask)
		}
	} else {
		limit.Count += step
		err = s.DBStory.UpdateDailyLimit(ctx, tx, limit)
		if err != nil {
			s.Logger.ZapLogger.Error("UpdateDailyLimit", zap.Error(err), zap.Any("userID", limit.UserID), zap.Any("place", LogPlace))
			rollbackErr := s.DBStory.RollbackTx(ctx, tx)
			if rollbackErr != nil {
				s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("userID", limit.UserID), zap.Any("place", LogPlace))
			}
			return errors.New(text_messages.TextErrorCreateTask)
		}
	}
	return nil
}
func (s *StoryServiceImpl) checkDailyLimits(ctx context.Context, userID int64, LogPlace string) (*models.DailyLimit, error) {
	//Проверяем превышение лимита в кэше
	isExist, err := s.CStory.CheckExceededLimit(ctx, userID)
	if err != nil {
		s.Logger.ZapLogger.Warn("CheckExceededLimit", zap.Error(err), zap.Any("userID", userID), zap.Any("place", LogPlace))
	} else if isExist {
		s.Logger.ZapLogger.Warn("CheckExceededLimit", zap.Error(errors.New("cache: user has exceeded daily action limit")), zap.Any("userID", userID), zap.Any("place", LogPlace))
		return nil, errors.New(text_messages.TextErrorUserDailyLimit)
	} else if !isExist {
		s.Logger.ZapLogger.Info("CheckExceededLimit Exceeded Limits not in cache. Checking in database...", zap.Any("userID", userID), zap.Any("place", LogPlace))
	}
	// Проверяем в базе данных, есть ли дневные ходы у пользователя для создания новой истории
	limit, err := s.DBStory.GetDailyLimit(ctx, userID)
	if err != nil {
		s.Logger.ZapLogger.Error("GetDailyLimit", zap.Error(err), zap.Any("userID", userID), zap.Any("place", LogPlace))
		return nil, errors.New(text_messages.TextErrorCreateTask)
	}
	//TODO проверить дневные лимиты с учетом подписки
	if limit == nil {
		//создаем новый лимит
		baseDayLimitStr, err := s.CStory.GetSetting(ctx, "limit.day.base")
		if err != nil {
			s.Logger.ZapLogger.Error("Failed to get day limit from cache", zap.Error(err), zap.Any("userID", userID), zap.Any("place", LogPlace))
			return nil, errors.New(text_messages.TextErrorCreateTask)
		}
		baseDayLimit, convErr := strconv.Atoi(baseDayLimitStr)
		if convErr != nil {
			s.Logger.ZapLogger.Error("Atoi limit.day.base", zap.Error(convErr), zap.Any("userID", userID), zap.Any("place", LogPlace))
			return nil, errors.New(text_messages.TextErrorCreateTask)
		}
		limit = models.NewDailyLimit(userID, 0, baseDayLimit)
	}
	if limit.LimitCount <= limit.Count {
		s.Logger.ZapLogger.Warn("GetDailyLimit", zap.Error(errors.New("client: user has exceeded daily action limit")), zap.Any("userID", userID), zap.Any("place", LogPlace))
		//Добавляем превышение лимита в кэш
		err := s.CStory.AddExceededLimit(ctx, userID)
		if err != nil {
			s.Logger.ZapLogger.Warn("AddExceededLimit", zap.Error(err), zap.Any("userID", userID), zap.Any("place", LogPlace))
		}
		return nil, errors.New(text_messages.TextErrorUserDailyLimit)
	}
	return limit, nil
}

// getAllSettingsFromCache получает все настройки из кэша (Redis)
func (s *StoryServiceImpl) getAllSettingsFromCache(ctx context.Context) (map[string]string, error) {
	place := "getAllSettingsFromCache"
	if s.CStory == nil {
		return nil, errors.New("cache client is not initialized")
	}
	settings, err := s.CStory.GetAllSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get settings from cache: %w", err)
	}
	s.Logger.ZapLogger.Info("Settings loaded from cache successfully", zap.Any("count", len(settings)), zap.Any("place", place))
	return settings, nil
}

// getAllSettingsFromDB получает все настройки из базы данных (PostgreSQL)
func (s *StoryServiceImpl) getAllSettingsFromDB(ctx context.Context) ([]*models.Setting, error) {
	place := "getAllSettingsFromDB"
	if s.DBStory == nil {
		return nil, errors.New("database client is not initialized")
	}
	settings, err := s.DBStory.GetAllSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get settings from database: %w", err)
	}
	if settings == nil {
		return nil, errors.New("settings from database is nil")
	}
	s.Logger.ZapLogger.Info("Settings loaded from database successfully", zap.Any("count", len(settings)), zap.Any("place", place))
	return settings, nil
}
