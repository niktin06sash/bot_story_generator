package service

import (
	"bot_story_generator/internal/models"
	"bot_story_generator/internal/text_messages"
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

func (s *StoryServiceImpl) incrementOrAddDailyLimit(ctx context.Context, tx pgx.Tx, limit *models.DailyLimit) error {
	var err error
	if limit.Count == 1 {
		err = s.DBStory.AddDailyLimit(ctx, tx, limit)
		if err != nil {
			s.Logger.ZapLogger.Error("AddDailyLimit failed", zap.Error(err), zap.Any("userID", limit.UserID))
			rollbackErr := s.DBStory.RollbackTx(ctx, tx)
			if rollbackErr != nil {
				s.Logger.ZapLogger.Error("RollbackTx failed", zap.Error(rollbackErr), zap.Any("userID", limit.UserID))
			}
			return errors.New(text_messages.TextErrorCreateTask)
		}
	} else {
		err = s.DBStory.IncrementDailyLimit(ctx, tx, limit.UserID)
		if err != nil {
			s.Logger.ZapLogger.Error("IncrementDailyLimit failed", zap.Error(err), zap.Any("userID", limit.UserID))
			rollbackErr := s.DBStory.RollbackTx(ctx, tx)
			if rollbackErr != nil {
				s.Logger.ZapLogger.Error("RollbackTx failed", zap.Error(rollbackErr), zap.Any("userID", limit.UserID))
			}
			return errors.New(text_messages.TextErrorCreateTask)
		}
	}
	return nil
}
