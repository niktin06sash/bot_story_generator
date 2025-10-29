package service

import (
	"bot_story_generator/internal/models"
	"bot_story_generator/internal/text_messages"
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

func (s *StoryServiceImpl) updateOrAddDailyLimit(ctx context.Context, tx pgx.Tx, limit *models.DailyLimit, step int, LogPlace string) error {
	var err error
	if limit.Count == 0 {
		limit.Count += step
		err = s.DBStory.AddDailyLimit(ctx, tx, limit)
		if err != nil {
			msg := fmt.Sprintf("AddDailyLimit(%v)", LogPlace)
			s.Logger.ZapLogger.Error(msg, zap.Error(err), zap.Any("userID", limit.UserID))
			rollbackErr := s.DBStory.RollbackTx(ctx, tx)
			if rollbackErr != nil {
				msg := fmt.Sprintf("Rollback(%v)", LogPlace)
				s.Logger.ZapLogger.Error(msg, zap.Error(rollbackErr), zap.Any("userID", limit.UserID))
			}
			return errors.New(text_messages.TextErrorCreateTask)
		}
	} else {
		limit.Count += step
		err = s.DBStory.UpdateDailyLimit(ctx, tx, limit)
		if err != nil {
			msg := fmt.Sprintf("UpdateDailyLimit(%v)", LogPlace)
			s.Logger.ZapLogger.Error(msg, zap.Error(err), zap.Any("userID", limit.UserID))
			rollbackErr := s.DBStory.RollbackTx(ctx, tx)
			if rollbackErr != nil {
				msg := fmt.Sprintf("Rollback(%v)", LogPlace)
				s.Logger.ZapLogger.Error(msg, zap.Error(rollbackErr), zap.Any("userID", limit.UserID))
			}
			return errors.New(text_messages.TextErrorCreateTask)
		}
	}
	return nil
}
