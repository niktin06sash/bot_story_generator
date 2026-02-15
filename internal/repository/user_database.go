package repository

import (
	"bot_story_generator/internal/database"
	"bot_story_generator/internal/models"
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type UserDatabaseImpl struct {
	databaseclient *database.DBObject
}

func NewUserDatabase(db *database.DBObject) *UserDatabaseImpl {
	return &UserDatabaseImpl{
		databaseclient: db,
	}
}
func (s *UserDatabaseImpl) AddUser(ctx context.Context, user *models.User) error {
	if user == nil {
		return fmt.Errorf("server: user is nil")
	}
	query := `
        INSERT INTO users (ID)
        VALUES ($1)
        ON CONFLICT(ID) DO NOTHING 
        RETURNING ID
    `
	var insertedID int64
	err := s.databaseclient.Pool.QueryRow(ctx, query, user.ID).Scan(&insertedID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("client: user is already registered")
		}
		return fmt.Errorf("server: database error: %w", err)
	}

	return nil
}
