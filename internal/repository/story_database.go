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
func (s *StoryDatabaseImpl) AddUser(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (ID, chatID)
		VALUES ($1, $2)
		ON CONFLICT(ID) DO NOTHING RETURNING ID 
	`
	_, err := s.databaseclient.Pool.Exec(
		ctx,
		query,
		user.ID,
		user.ChatID,
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

func (s *StoryDatabaseImpl) CheckActiveStories(ctx context.Context, userID int64) error {
	query := `
    	SELECT u.ID, COUNT(s.ID) AS count_active_story
    	FROM users u
    	LEFT JOIN stories s ON u.ID = s.userID AND s.isActive = TRUE
    	WHERE u.ID = $1
    	GROUP BY u.ID
	`
	row := s.databaseclient.Pool.QueryRow(
		ctx,
		query,
		userID,
	)
	var scanuserID int64
	var countActive int64
	err := row.Scan(&userID, &countActive)
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
	row := tx.QueryRow(
		ctx,
		query,
		story.UserID,
	)
	var storyID int
	err := row.Scan(&storyID)
	if err != nil {
		return 0, fmt.Errorf("server: database error: %w", err)
	}
	return storyID, nil
}
func (s *StoryDatabaseImpl) AddVariant(ctx context.Context, tx pgx.Tx, variant *models.StoryVariant) error {
	query := `
		INSERT INTO storiesVariants (storyID, data)
    	VALUES ($1, $2)
	`
	_, err := tx.Exec(
		ctx,
		query,
		variant.StoryID,
		variant.Data,
	)

	if err != nil {
		return fmt.Errorf("server: database error: %w", err)
	}
	return nil
}

func (s *StoryDatabaseImpl) GetAllStorySegments(ctx context.Context, chatID int64) (*models.AllStorySegments, error) {
	//! ТЕСТ
	return &models.AllStorySegments{StorySegments: []string{""}}, nil
}
