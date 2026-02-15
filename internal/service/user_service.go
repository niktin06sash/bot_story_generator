package service

import (
	"bot_story_generator/internal/logger"
	"bot_story_generator/internal/models"
	"bot_story_generator/internal/text_messages"
	"context"
	"errors"
	"strings"
	"time"

	"go.uber.org/zap"
)

type UserServiceImpl struct {
	UserDatabase UserDatabase
	UserCache    UserCache
	Logger       *logger.Logger
}

func NewUserService(udb UserDatabase, ucache UserCache, logger *logger.Logger) *UserServiceImpl {
	return &UserServiceImpl{
		UserDatabase: udb,
		UserCache:    ucache,
		Logger:       logger,
	}
}
func (s *UserServiceImpl) CreateUser(ctx context.Context, userID int64) (string, error) {
	trace := getTrace(ctx, s.Logger)
	place := "CreateUser"
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	isExist, err := s.UserCache.CheckCreatedUser(ctxTimeout, userID)
	if err != nil {
		s.Logger.ZapLogger.Warn("CheckCreatedUser", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
	} else if isExist {
		//3 лог
		s.Logger.ZapLogger.Info("CheckCreatedUser User is already created. Returning response", zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return "", errors.New(text_messages.TextGreeting)
	} else if !isExist {
		//3 лог
		s.Logger.ZapLogger.Info("CheckCreatedUser Created user not in cache. Trying creating in database...", zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
	}
	user := models.NewUser(userID)
	err = s.UserDatabase.AddUser(ctxTimeout, user)
	if err != nil {
		if strings.HasPrefix(err.Error(), "client: ") {
			s.Logger.ZapLogger.Warn("AddUser", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
			err := s.UserCache.AddCreatedUser(ctxTimeout, userID)
			if err != nil {
				s.Logger.ZapLogger.Warn("AddCreatedUser", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
			}
			return "", errors.New(text_messages.TextGreeting)
		}
		s.Logger.ZapLogger.Error("AddUser", zap.Error(err), zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
		return "", errors.New(text_messages.TextErrorCreateTask)
	}
	//4 лог
	s.Logger.ZapLogger.Info("User created successfully", zap.Any("userID", userID), zap.Any("traceID", trace.ID), zap.Any("place", place))
	return text_messages.TextGreeting, nil
}
