package repository

import (
	"bot_story_generator/internal/database"
	"bot_story_generator/internal/models"
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type SettingDatabaseImpl struct {
	databaseclient *database.DBObject
}

func NewSettingDatabase(db *database.DBObject) *SettingDatabaseImpl {
	return &SettingDatabaseImpl{
		databaseclient: db,
	}
}
func (s *SettingDatabaseImpl) GetAllSettings(ctx context.Context) ([]*models.Setting, error) {
	query := `
		SELECT key, value, updated_at, updated_by
		FROM settings
	`
	rows, err := s.databaseclient.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("server: database error: %w", err)
	}
	defer rows.Close()

	res := make([]*models.Setting, 0)
	for rows.Next() {
		var st models.Setting
		if err := rows.Scan(&st.Key, &st.Value, &st.UpdatedAt, &st.UpdatedBy); err != nil {
			return nil, fmt.Errorf("server: database error: %w", err)
		}
		res = append(res, &st)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("server: database error: %w", err)
	}
	return res, nil
}

func (s *SettingDatabaseImpl) GetSetting(ctx context.Context, key string) (*models.Setting, error) {
	query := `
		SELECT key, value, updated_at, updated_by
		FROM settings
		WHERE key = $1
	`
	row := s.databaseclient.Pool.QueryRow(ctx, query, key)
	st := &models.Setting{}
	if err := row.Scan(&st.Key, &st.Value, &st.UpdatedAt, &st.UpdatedBy); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("server: database error: %w", err)
	}
	return st, nil
}

func (s *SettingDatabaseImpl) SetSetting(ctx context.Context, tx pgx.Tx, setting *models.Setting) error {
	if setting == nil {
		return fmt.Errorf("server: setting is nil")
	}
	query := `
		INSERT INTO settings (key, value, updated_at, updated_by)
		VALUES ($1, $2, now(), $3)
		ON CONFLICT (key) DO UPDATE
		SET value = $2,
			updated_at = now(),
			updated_by = $3
	`
	_, err := tx.Exec(ctx, query, setting.Key, setting.Value, setting.UpdatedBy)
	if err != nil {
		return fmt.Errorf("server: database error: %w", err)
	}
	return nil
}
