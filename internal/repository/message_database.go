package repository

import (
	"bot_story_generator/internal/database"
	"bot_story_generator/internal/models"
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type MessageDatabaseImpl struct {
	databaseclient *database.DBObject
}

func NewMessageDatabase(db *database.DBObject) *MessageDatabaseImpl {
	return &MessageDatabaseImpl{
		databaseclient: db,
	}
}
func (s *MessageDatabaseImpl) AddStoryMessages(ctx context.Context, tx pgx.Tx, msgs []*models.StoryMessage) error {
	//делаем batch вместо двух запросов insert
	batch := &pgx.Batch{}
	query := `
		INSERT INTO storiesMessages (storyID, data, type)
		VALUES ($1, $2, $3)
	`
	for _, msg := range msgs {
		batch.Queue(query, msg.StoryID, msg.Data, msg.Type)
	}
	br := tx.SendBatch(ctx, batch)
	defer br.Close()
	for range msgs {
		_, err := br.Exec()
		if err != nil {
			return fmt.Errorf("server: database error: %w", err)
		}
	}
	return nil
}

func (s *MessageDatabaseImpl) GetAllStorySegments(ctx context.Context, storyID int) ([]*models.StoryMessage, error) {
	//TODO придумать, как не делать select + for по всем сообщениям истории(возможно, Redis пригодится)
	query := `
		SELECT data, type
		FROM storiesMessages WHERE storyID = $1
		ORDER BY createdAt ASC
	`
	rows, err := s.databaseclient.Pool.Query(ctx, query, storyID)
	if err != nil {
		return nil, fmt.Errorf("server: database error: %w", err)
	}
	defer rows.Close()
	msgs := make([]*models.StoryMessage, 0)
	for rows.Next() {
		msg := &models.StoryMessage{}
		if err := rows.Scan(&msg.Data, &msg.Type); err != nil {
			return nil, fmt.Errorf("server: database error: %w", err)
		}
		msgs = append(msgs, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("server: database error: %w", err)
	}
	return msgs, nil
}
