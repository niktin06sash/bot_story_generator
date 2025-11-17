package repository

import (
	"bot_story_generator/internal/database"
	"bot_story_generator/internal/models"
	"fmt"

	"context"

	"github.com/jackc/pgx/v5"
)

type StoryDatabaseImpl struct {
	databaseclient *database.DBObject
}

func NewStoryDatabase(db *database.DBObject) *StoryDatabaseImpl {
	return &StoryDatabaseImpl{
		databaseclient: db,
	}
}

func (s *StoryDatabaseImpl) GetActiveStories(ctx context.Context, userID int64) ([]*models.Story, error) {
	query := `
    	SELECT ID, userID, data, createdAt
    	FROM stories
    	WHERE userID = $1 and isActive = TRUE
	`
	rows, err := s.databaseclient.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("server: database error: %w", err)
	}
	defer rows.Close()
	var stories []*models.Story
	for rows.Next() {
		story := &models.Story{}
		err := rows.Scan(&story.ID, &story.UserID, &story.Data, &story.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("server: database error: %w", err)
		}
		stories = append(stories, story)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("server: database error: %w", err)
	}
	return stories, nil
}

func (s *StoryDatabaseImpl) AddStory(ctx context.Context, tx pgx.Tx, story *models.Story) (int, error) {
	query := `
		INSERT INTO stories (userID, data)
    	VALUES ($1, $2)
		RETURNING ID
	`
	row := tx.QueryRow(ctx, query, story.UserID, story.Data)
	var storyID int
	err := row.Scan(&storyID)
	if err != nil {
		return 0, fmt.Errorf("server: database error: %w", err)
	}
	return storyID, nil
}

func (s *StoryDatabaseImpl) StopStory(ctx context.Context, userID int64) error {
	query := `
		UPDATE stories 
		SET isActive = FALSE 
		WHERE userID = $1 AND isActive = TRUE
	`
	_, err := s.databaseclient.Pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("server: database error: %w", err)
	}
	return nil
}
