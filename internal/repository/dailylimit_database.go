package repository

import (
	"bot_story_generator/internal/database"
	"bot_story_generator/internal/models"
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type DailyLimitDatabaseImpl struct {
	databaseclient *database.DBObject
}

func NewDailyLimitDatabase(db *database.DBObject) *DailyLimitDatabaseImpl {
	return &DailyLimitDatabaseImpl{
		databaseclient: db,
	}
}
func (s *DailyLimitDatabaseImpl) GetDailyLimit(ctx context.Context, userID int64) (*models.DailyLimit, error) {
	query := `
		SELECT userID, date, count, limitCount FROM dailyLimits
		WHERE userID = $1 and date = CURRENT_DATE
	`
	row := s.databaseclient.Pool.QueryRow(ctx, query, userID)
	limit := &models.DailyLimit{}
	err := row.Scan(&limit.UserID, &limit.Date, &limit.Count, &limit.LimitCount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("server: database error: %w", err)
	}
	return limit, nil
}

func (s *DailyLimitDatabaseImpl) AddDailyLimit(ctx context.Context, tx pgx.Tx, dailyLimit *models.DailyLimit) error {
	if dailyLimit == nil {
		return fmt.Errorf("server: dailyLimit is nil")
	}
	query := `
        INSERT INTO dailyLimits (userID, count, limitCount)
        VALUES ($1, $2, $3)
	`
	_, err := tx.Exec(ctx, query, dailyLimit.UserID, dailyLimit.Count, dailyLimit.LimitCount)
	if err != nil {
		return fmt.Errorf("server: database error: %w", err)
	}
	return nil
}

func (s *DailyLimitDatabaseImpl) UpdateCountDailyLimit(ctx context.Context, tx pgx.Tx, dailyLimit *models.DailyLimit) error {
	if dailyLimit == nil {
		return fmt.Errorf("server: dailyLimit is nil")
	}
	query := `
        UPDATE dailyLimits 
        SET count = $1
        WHERE userID = $2 AND date = CURRENT_DATE
    `
	_, err := tx.Exec(ctx, query, dailyLimit.Count, dailyLimit.UserID)
	if err != nil {
		return fmt.Errorf("server: database error: %w", err)
	}
	return nil
}
func (s *DailyLimitDatabaseImpl) UpdateLimitCountDailyLimit(ctx context.Context, dailyLimit *models.DailyLimit) error {
	if dailyLimit == nil {
		return fmt.Errorf("server: dailyLimit is nil")
	}
	query := `
        UPDATE dailyLimits 
        SET limitCount = $1
        WHERE userID = $2 AND date = CURRENT_DATE
    `
	_, err := s.databaseclient.Pool.Exec(ctx, query, dailyLimit.LimitCount, dailyLimit.UserID)
	return err
}
