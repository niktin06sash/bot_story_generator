package repository

import (
	"bot_story_generator/internal/database"
	"bot_story_generator/internal/models"

	"context"
)

type StoryDatabaseImpl struct {
	databaseclient *database.DBObject
}

func NewStoryDatabase(db *database.DBObject) *StoryDatabaseImpl {
	return &StoryDatabaseImpl{databaseclient: db}
}

func (s *StoryDatabaseImpl) AddUser(ctx context.Context, user models.User) error {
	query := `
		INSERT INTO users (chat_id, is_sub)
		VALUES ($1, $2)
		ON CONFLICT(chat_id) DO UPDATE SET
			is_sub = EXCLUDED.is_sub
	`
	_, err := s.databaseclient.Pool.Exec(
		ctx,
		query,
		user.ChatID,
		user.IsSub,
	)
	return err
}

func (s *StoryDatabaseImpl) GetUser(ctx context.Context, chatID int64) (*models.User, error) {
	query := `
		SELECT id, chat_id, is_sub
		FROM users
		WHERE chat_id = $1
	`
	row := s.databaseclient.Pool.QueryRow(
		ctx,
		query,
		chatID,
	)

	var user models.User
	if err := row.Scan(&user.ID, &user.ChatID, &user.IsSub); err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *StoryDatabaseImpl) GetAllStorySegments(ctx context.Context, chatID int64) (*models.AllStorySegments, error) {
	//! ТЕСТ
	return &models.AllStorySegments{StorySegments: []string{""}}, nil
}