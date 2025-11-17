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
	"golang.org/x/sync/errgroup"
)

func (s *ServiceImpl) updateOrAddDailyLimit(ctx context.Context, tx pgx.Tx, limit *models.DailyLimit, step int, trace models.Trace, LogPlace string) error {
	var err error
	if limit.Count == 0 {
		limit.Count += step
		err = s.daylimitDatabase.AddDailyLimit(ctx, tx, limit)
		if err != nil {
			s.Logger.ZapLogger.Error("AddDailyLimit", zap.Error(err), zap.Any("userID", limit.UserID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
			rollbackErr := s.txManager.RollbackTx(context.Background(), tx)
			if rollbackErr != nil {
				s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("userID", limit.UserID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
			}
			return errors.New(text_messages.TextErrorCreateTask)
		}
	} else {
		limit.Count += step
		err = s.daylimitDatabase.UpdateCountDailyLimit(ctx, tx, limit)
		if err != nil {
			s.Logger.ZapLogger.Error("UpdateDailyLimit", zap.Error(err), zap.Any("userID", limit.UserID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
			rollbackErr := s.txManager.RollbackTx(context.Background(), tx)
			if rollbackErr != nil {
				s.Logger.ZapLogger.Error("RollbackTx", zap.Error(rollbackErr), zap.Any("userID", limit.UserID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
			}
			return errors.New(text_messages.TextErrorCreateTask)
		}
	}
	return nil
}
func (s *ServiceImpl) checkDailyLimits(ctx context.Context, userID int64, trace models.Trace, LogPlace string) (*models.DailyLimit, error) {
	//Проверяем превышение лимита в кэше
	isExist, err := s.daylimitCache.CheckExceededLimit(ctx, userID)
	if err != nil {
		s.Logger.ZapLogger.Warn("CheckExceededLimit", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
	} else if isExist {
		s.Logger.ZapLogger.Warn("CheckExceededLimit", zap.Error(errors.New("cache: user has exceeded daily action limit")), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
		return nil, errors.New(text_messages.TextErrorUserDailyLimit)
	} else if !isExist {
		s.Logger.ZapLogger.Info("CheckExceededLimit Exceeded Limits not in cache. Checking in database...", zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
	}
	// Проверяем в базе данных, есть ли дневные ходы у пользователя для создания новой истории
	var limitv *models.DailyLimit
	var subb *models.Subscription
	g, ctxG := errgroup.WithContext(ctx)
	//параллельно делаем запросы для получения данных дневного лимита и подписки(если есть хоть одна ошибка - прекращаем выполнение)
	g.Go(func() error {
		limit, err := s.daylimitDatabase.GetDailyLimit(ctxG, userID)
		if err != nil {
			s.Logger.ZapLogger.Error("GetDailyLimit", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
			return errors.New(text_messages.TextErrorCreateTask)
		}
		limitv = limit
		return nil
	})
	g.Go(func() error {
		subscriptions, err := s.subDatabase.GetActiveSubscriptions(ctxG, userID)
		if err != nil {
			s.Logger.ZapLogger.Error("GetActiveSubscriptions", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
			return errors.New(text_messages.TextErrorCreateTask)
		}
		if len(subscriptions) > 1 {
			s.Logger.ZapLogger.Error("GetActiveSubscriptions", zap.Error(fmt.Errorf("server: more than one active subscription found")), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
			return errors.New(text_messages.TextErrorCreateTask)
		}
		if len(subscriptions) == 1 {
			subb = subscriptions[0]
		}
		return nil
	})
	err = g.Wait()
	if err != nil {
		return nil, err
	}
	//сегодня ходов еще не было + подписка неактивна
	if limitv == nil && subb == nil {
		baseDayLimitStr, err := s.settingCache.GetSetting(ctx, models.SettingKeyLimitBaseDay)
		if err != nil {
			s.Logger.ZapLogger.Error("GetSetting", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
			return nil, errors.New(text_messages.TextErrorCreateTask)
		}
		baseDayLimit, convErr := strconv.Atoi(baseDayLimitStr)
		if convErr != nil {
			s.Logger.ZapLogger.Error("Atoi limit.day.base", zap.Error(convErr), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
			return nil, errors.New(text_messages.TextErrorCreateTask)
		}
		limitv = models.NewDailyLimit(userID, 0, baseDayLimit)
	}
	//сегодня ходов еще не было + подписка активна
	if limitv == nil && subb != nil {
		premDayLimitStr, err := s.settingCache.GetSetting(ctx, models.SettingKeyLimitPremiumDay)
		if err != nil {
			s.Logger.ZapLogger.Error("GetSetting", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
			return nil, errors.New(text_messages.TextErrorCreateTask)
		}
		premDayLimit, convErr := strconv.Atoi(premDayLimitStr)
		if convErr != nil {
			s.Logger.ZapLogger.Error("Atoi limit.day.premium", zap.Error(convErr), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
			return nil, errors.New(text_messages.TextErrorCreateTask)
		}
		limitv = models.NewDailyLimit(userID, 0, premDayLimit)
	}
	if limitv.LimitCount <= limitv.Count {
		s.Logger.ZapLogger.Warn("GetDailyLimit", zap.Error(errors.New("client: user has exceeded daily action limit")), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
		//Добавляем превышение лимита в кэш
		err := s.daylimitCache.AddExceededLimit(ctx, userID)
		if err != nil {
			s.Logger.ZapLogger.Warn("AddExceededLimit", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", LogPlace))
		}
		return nil, errors.New(text_messages.TextErrorUserDailyLimit)
	}
	return limitv, nil
}
func (s *ServiceImpl) getSubPrice(ctx context.Context, userID int64, trace models.Trace, logPlace string) (int, error) {
	price, err := s.settingCache.GetSetting(ctx, models.SettingKeyPriceBasicSubscription)
	if err != nil {
		s.Logger.ZapLogger.Error("GetSetting", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", logPlace))
		return 0, errors.New(text_messages.TextErrorProcessPayment)
	}

	priceInt, err := strconv.Atoi(price)
	if err != nil {
		s.Logger.ZapLogger.Error("Atoi sub.basic.price", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", logPlace))
		return 0, errors.New(text_messages.TextErrorProcessPayment)
	}
	return priceInt, nil
}
func (s *ServiceImpl) getTrace(ctx context.Context) models.Trace {
	trace, ok := ctx.Value(models.TraceKey).(models.Trace)
	if !ok {
		s.Logger.ZapLogger.Warn("Context value for 'trace' is not a models.Trace", zap.Any("trace", trace))
		return models.Trace{}
	}
	return trace
}
