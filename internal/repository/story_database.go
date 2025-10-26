package repository

import (
	"bot_story_generator/internal/database"
	"bot_story_generator/internal/models"
	"errors"
	"fmt"

	"context"

	"github.com/jackc/pgx/v5"
)

type StoryDatabaseImpl struct {
	databaseclient *database.DBObject
}

func NewStoryDatabase(db *database.DBObject) *StoryDatabaseImpl {
	return &StoryDatabaseImpl{databaseclient: db}
}

// USERS
func (s *StoryDatabaseImpl) AddUser(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (ID, chatID, isSub)
		VALUES ($1, $2, $3)
		ON CONFLICT(ID) DO NOTHING RETURNING ID 
	`
	_, err := s.databaseclient.Pool.Exec(
		ctx,
		query,
		user.ID,
		user.IsSub,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("client: user with user_id=%d is already registered", user.ID)
		}
		return fmt.Errorf("server: database error: %w", err)
	}
	return nil
}

// STORIES
func (s *StoryDatabaseImpl) CheckActiveStories(ctx context.Context, userID int64) error {
	query := `
    	SELECT u.ID, COUNT(s.ID) AS count_active_story
    	FROM users u
    	LEFT JOIN stories s ON u.ID = s.userID AND s.isActive = TRUE
    	WHERE u.ID = $1
    	GROUP BY u.ID
	`
	row := s.databaseclient.Pool.QueryRow(ctx, query, userID)
	var scanuserID int64
	var countActive int64
	err := row.Scan(&scanuserID, &countActive)
	if err != nil {
		return fmt.Errorf("server: database error: %w", err)
	}
	if countActive > 0 {
		return fmt.Errorf("client: user with user_id=%d already has an active history", scanuserID)
	}
	return nil
}
func (s *StoryDatabaseImpl) AddStory(ctx context.Context, tx pgx.Tx, story *models.Story) (int, error) {
	query := `
		INSERT INTO stories (userID)
    	VALUES ($1)
		RETURNING ID
	`
	row := tx.QueryRow(ctx, query, story.UserID)
	var storyID int
	err := row.Scan(&storyID)
	if err != nil {
		return 0, fmt.Errorf("server: database error: %w", err)
	}
	return storyID, nil
}

// VARIANTS
func (s *StoryDatabaseImpl) AddVariant(ctx context.Context, tx pgx.Tx, variant *models.StoryVariant) error {
	query := `
		INSERT INTO storiesVariants (storyID, data, type)
    	VALUES ($1, $, $3)
	`
	_, err := tx.Exec(ctx, query, variant.StoryID, variant.Data, variant.Type)

	if err != nil {
		return fmt.Errorf("server: database error: %w", err)
	}
	return nil
}
func (s *StoryDatabaseImpl) GetVariants(ctx context.Context, userID int64) (*models.StoryVariant, error) {
	query := `
		SELECT sv.storyid, sv.data, sv.type
		FROM storiesVariants sv
		INNER JOIN stories s ON sv.storyid = s.id
		WHERE s.userid = $1 and s.isactive = true
	`
	rows, err := s.databaseclient.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("server: database error: %w", err)
	}
	defer rows.Close()

	var variants []*models.StoryVariant
	for rows.Next() {
		variant := &models.StoryVariant{}
		err := rows.Scan(&variant.StoryID, &variant.Data, &variant.Type)
		if err != nil {
			return nil, fmt.Errorf("server: database error: %w", err)
		}
		variants = append(variants, variant)
	}

	if len(variants) > 1 {
		return nil, fmt.Errorf("server: database error: more one active story found for user_id=%d", userID)
	}
	if len(variants) == 0 {
		return nil, fmt.Errorf("server: database error: no active story found for user_id=%d", userID)
	}
	return variants[0], nil
}

// LIMITS
func (s *StoryDatabaseImpl) CheckDailyLimit(ctx context.Context, userID int64) (*models.DailyLimit, error) {
	query := `
		SELECT userID, date, count, limit FROM dailyLimits
		WHERE userID = $1 and date = CURRENT_DATE
	`
	row := s.databaseclient.Pool.QueryRow(ctx, query, userID)
	limit := &models.DailyLimit{}
	err := row.Scan(&limit.UserID, &limit.Date, &limit.Count, &limit.Limit)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.NewDailyLimit(userID, 1, 5), nil
		}
		return nil, fmt.Errorf("server: database error: %w", err)
	}
	if limit.Limit == limit.Count {
		return nil, fmt.Errorf("client: user with user_id=%d has exceeded daily action limit: %w", userID, err)
	}
	return limit, nil
}
func (s *StoryDatabaseImpl) AddDailyLimit(ctx context.Context, tx pgx.Tx, dailyLimit *models.DailyLimit) error {
	query := `
		INSERT INTO dailyLimits (userID, count, limit)
		VALUES ($1, $2, $3)
	`
	_, err := tx.Exec(ctx, query, dailyLimit.UserID, dailyLimit.Count, dailyLimit.Limit)
	if err != nil {
		return fmt.Errorf("server: database error: %w", err)
	}
	return nil
}
func (s *StoryDatabaseImpl) IncrementDailyLimit(ctx context.Context, tx pgx.Tx, userID int64) error {
	query := `
        UPDATE dailyLimits 
        SET count = count + 1
        WHERE userID = $1 AND date = CURRENT_DATE
    `
	_, err := tx.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("server: database error: %w", err)
	}
	return nil
}

// MESSAGES
func (s *StoryDatabaseImpl) GetAllStorySegments(ctx context.Context, userID int64) (*models.AllStorySegments, error) {
	//! ТЕСТ
	return &models.AllStorySegments{StorySegments: []string{""}}, nil
}
