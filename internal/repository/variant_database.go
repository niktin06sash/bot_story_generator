package repository

import (
	"bot_story_generator/internal/database"
	"bot_story_generator/internal/models"
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type VariantDatabaseImpl struct {
	databaseclient *database.DBObject
}

func NewVariantDatabase(db *database.DBObject) *VariantDatabaseImpl {
	return &VariantDatabaseImpl{
		databaseclient: db,
	}
}
func (s *VariantDatabaseImpl) AddVariant(ctx context.Context, tx pgx.Tx, variant *models.StoryVariant) error {
	if variant == nil {
		return fmt.Errorf("server: variant is nil")
	}
	query := `
		INSERT INTO storiesVariants (storyID, data, type)
    	VALUES ($1, $2, $3)
	`
	_, err := tx.Exec(ctx, query, variant.StoryID, variant.Data, variant.Type)
	if err != nil {
		return fmt.Errorf("server: database error: %w", err)
	}
	return nil
}

func (s *VariantDatabaseImpl) UpdateVariant(ctx context.Context, tx pgx.Tx, variant *models.StoryVariant) error {
	if variant == nil {
		return fmt.Errorf("server: variant is nil")
	}
	query := `
		UPDATE storiesVariants
		SET data = $2, type = $3
		WHERE storyID = $1
	`
	_, err := tx.Exec(ctx, query, variant.StoryID, variant.Data, variant.Type)
	if err != nil {
		return fmt.Errorf("server: database error: %w", err)
	}
	return nil
}

func (s *VariantDatabaseImpl) GetActiveVariants(ctx context.Context, userID int64) ([]*models.StoryVariant, error) {
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

	variants := make([]*models.StoryVariant, 0)
	for rows.Next() {
		variant := &models.StoryVariant{}
		err := rows.Scan(&variant.StoryID, &variant.Data, &variant.Type)
		if err != nil {
			return nil, fmt.Errorf("server: database error: %w", err)
		}
		variants = append(variants, variant)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("server: database error: %w", err)
	}
	return variants, nil
}
