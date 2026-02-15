package service

import (
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/models"
	"bot_story_generator/internal/text_messages"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

type AdminServiceImpl struct {
	SubDatabase SubscriptionDatabase
	Logger      *logger.Logger
}

func NewAdminService(subdb SubscriptionDatabase, logger *logger.Logger) *AdminServiceImpl {
	return &AdminServiceImpl{
		SubDatabase: subdb,
		Logger:      logger,
	}
}
func (s *AdminServiceImpl) AddSubscriptionByAdmin(ctx context.Context, userID int64, subType, currency string, price int, durationDays int) (string, error) {
	trace := getTrace(ctx, s.Logger)
	place := "AddSubscriptionByAdmin"
	now := time.Now()
	end := now.Add(time.Duration(durationDays) * 24 * time.Hour)
	payload := fmt.Sprintf("adminmanual-%d-%d", userID, time.Now().UnixNano())
	sub := &models.Subscription{
		Payload:       payload,
		ChargeId:      "adminmanual",
		UserID:        userID,
		Type:          subType,
		Status:        "paid",
		StartDate:     now,
		EndDate:       end,
		IsAutoRenewal: false,
		Currency:      currency,
		Price:         price,
	}
	err := s.SubDatabase.AddSubscription(ctx, sub)
	if err != nil {
		s.Logger.ZapLogger.Error("AddSubscription", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return "", errors.New(text_messages.TextErrorSettings)
	}
	s.Logger.ZapLogger.Info("Subscription added by admin", zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
	return fmt.Sprintf(text_messages.SuccessActivateSub, userID), nil
}

func (s *AdminServiceImpl) UpdateSubscriptionByAdmin(ctx context.Context, userID int64, durationDays int) (string, error) {
	trace := getTrace(ctx, s.Logger)
	place := "UpdateSubscriptionByAdmin"
	activeSubs, err := s.SubDatabase.GetActiveSubscriptions(ctx, userID)
	if err != nil {
		s.Logger.ZapLogger.Error("GetActiveSubscriptions", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return "", errors.New(text_messages.TextErrorSettings)
	}
	if len(activeSubs) == 0 {
		s.Logger.ZapLogger.Error("GetActiveSubscriptions", zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return "", errors.New(text_messages.TextErrorSettings)
	}
	curSub := activeSubs[0]
	if curSub == nil {
		s.Logger.ZapLogger.Error("GetActiveSubscriptions", zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return "", errors.New(text_messages.TextErrorSettings)
	}
	now := time.Now()
	end := now.Add(time.Duration(durationDays) * 24 * time.Hour)
	updatedSub := &models.Subscription{
		Payload:       curSub.Payload,
		UserID:        curSub.UserID,
		Type:          curSub.Type,
		Status:        "paid",
		StartDate:     now,
		EndDate:       end,
		IsAutoRenewal: curSub.IsAutoRenewal,
		Currency:      curSub.Currency,
		Price:         curSub.Price,
		ChargeId:      "adminmanual",
	}
	err = s.SubDatabase.UpdateSubscription(ctx, updatedSub)
	if err != nil {
		s.Logger.ZapLogger.Error("UpdateSubscription", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("payload", curSub.Payload), zap.Any("place", place))
		return "", errors.New(text_messages.TextErrorSettings)
	}

	s.Logger.ZapLogger.Info("Subscription updated by admin", zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("payload", curSub.Payload), zap.Any("place", place))
	return fmt.Sprintf(text_messages.SuccessUpdateSub, userID), nil
}

func (s *AdminServiceImpl) AdminCommands(ctx context.Context, command string) (string, error) {
	fields := strings.Fields(command)
	if len(fields) < 1 {
		return "", errors.New(text_messages.TextErrorSettings)
	}
	switch fields[0] {
	case "addsub":
		if len(fields) != 6 {
			return "", errors.New(text_messages.TextErrorSettings)
		}
		userID, err1 := strconv.ParseInt(fields[1], 10, 64)
		subType := fields[2]
		currency := fields[3]
		price, err2 := strconv.Atoi(fields[4])
		durationDays, err3 := strconv.Atoi(fields[5])
		if err1 != nil || err2 != nil || err3 != nil {
			return "", errors.New(text_messages.TextErrorSettings)
		}
		return s.AddSubscriptionByAdmin(ctx, userID, subType, currency, price, durationDays)
	case "updatesub":
		if len(fields) != 3 {
			return "", errors.New(text_messages.TextErrorSettings)
		}
		userID, err1 := strconv.ParseInt(fields[1], 10, 64)
		durationDays, err2 := strconv.Atoi(fields[2])
		if err1 != nil || err2 != nil {
			return "", errors.New(text_messages.TextErrorSettings)
		}
		return s.UpdateSubscriptionByAdmin(ctx, userID, durationDays)
	case "getsub":
		if len(fields) != 2 {
			return "", errors.New(text_messages.TextErrorSettings)
		}
		userID, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return "", errors.New(text_messages.TextErrorSettings)
		}
		subs, err := s.SubDatabase.GetActiveSubscriptions(ctx, userID)
		if err != nil {
			return "", err
		}
		return text_messages.FormatActiveSubscriptionsText(subs), nil
	default:
		return "", errors.New(text_messages.TextErrorSettings)
	}
}
