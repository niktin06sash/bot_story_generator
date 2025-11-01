package repository

import (
	"bot_story_generator/internal/config"
	"bot_story_generator/internal/database"
	"bot_story_generator/internal/models"
	"errors"
	"fmt"

	"context"

	"github.com/jackc/pgx/v5"
)

type StoryDatabaseImpl struct {
	databaseclient *database.DBObject
	tokenDayLimit  int
}

func NewStoryDatabase(cfg *config.Config, db *database.DBObject) *StoryDatabaseImpl {
	return &StoryDatabaseImpl{
		databaseclient: db,
		tokenDayLimit:  cfg.Setting.TokenDayLimit,
	}
}

// USERS
func (s *StoryDatabaseImpl) AddUser(ctx context.Context, user *models.User) error {
	query := `
        INSERT INTO users (ID, isSub)
        VALUES ($1, $2)
        ON CONFLICT(ID) DO NOTHING 
        RETURNING ID
    `
	var insertedID int64
	err := s.databaseclient.Pool.QueryRow(ctx, query, user.ID, user.IsSub).Scan(&insertedID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("client: user is already registered")
		}
		return fmt.Errorf("server: database error: %w", err)
	}

	return nil
}

// STORIES
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

func (s *StoryDatabaseImpl) GetActiveStoryID(ctx context.Context, userID int64) (int, error) {
	query := `
	    SELECT id
	    FROM stories
	    WHERE userID = $1 AND isActive = TRUE
	    LIMIT 1
	`
	var storyID int
	err := s.databaseclient.Pool.QueryRow(ctx, query, userID).Scan(&storyID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, errors.New("no active story found for given userID")
		}
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

// VARIANTS
func (s *StoryDatabaseImpl) AddVariant(ctx context.Context, tx pgx.Tx, variant *models.StoryVariant) error {
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

func (s *StoryDatabaseImpl) UpdateVariant(ctx context.Context, tx pgx.Tx, variant *models.StoryVariant) error {
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

func (s *StoryDatabaseImpl) GetActiveVariants(ctx context.Context, userID int64) ([]*models.StoryVariant, error) {
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
	return variants, nil
}

// LIMITS
func (s *StoryDatabaseImpl) GetDailyLimit(ctx context.Context, userID int64) (*models.DailyLimit, error) {
	query := `
		SELECT userID, date, count, limitCount FROM dailyLimits
		WHERE userID = $1 and date = CURRENT_DATE
	`
	row := s.databaseclient.Pool.QueryRow(ctx, query, userID)
	limit := &models.DailyLimit{}
	err := row.Scan(&limit.UserID, &limit.Date, &limit.Count, &limit.LimitCount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.NewDailyLimit(userID, 0, s.tokenDayLimit), nil
		}
		return nil, fmt.Errorf("server: database error: %w", err)
	}
	return limit, nil
}

func (s *StoryDatabaseImpl) AddDailyLimit(ctx context.Context, tx pgx.Tx, dailyLimit *models.DailyLimit) error {
	query := `
        INSERT INTO dailyLimits (userID, count, limitCount)
        VALUES ($1, $2 + 1, $3)
	`
	_, err := tx.Exec(ctx, query, dailyLimit.UserID, dailyLimit.Count, dailyLimit.LimitCount)
	if err != nil {
		return fmt.Errorf("server: database error: %w", err)
	}
	return nil
}

func (s *StoryDatabaseImpl) UpdateDailyLimit(ctx context.Context, tx pgx.Tx, dailyLimit *models.DailyLimit) error {
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

// MESSAGES
func (s *StoryDatabaseImpl) AddStoryMessages(ctx context.Context, userID int64, data string, msgType string) error {
	query := `
		INSERT INTO storiesMessages (storyID, data, type)
		SELECT id, $2, $3
		FROM stories
		WHERE userID = $1 AND isActive = true
		LIMIT 1;
	`
	_, err := s.databaseclient.Pool.Exec(ctx, query, userID, data, msgType)
	if err != nil {
		return fmt.Errorf("failed to add story message: %w", err)
	}
	return nil
}

func (s *StoryDatabaseImpl) GetAllStorySegments(ctx context.Context, userID int64) (*models.AllStorySegments, error) {
	query := `
		SELECT sm.data, sm.type
		FROM storiesMessages sm
		INNER JOIN stories s ON sm.storyID = s.ID
		WHERE s.userID = $1 AND s.isActive = TRUE
		ORDER BY sm.createdAt ASC
	`

	rows, err := s.databaseclient.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch story segments: %w", err)
	}
	defer rows.Close()

	var segments []models.StorySegment
	for rows.Next() {
		var data string
		var msgType string
		if err := rows.Scan(&data, &msgType); err != nil {
			return nil, fmt.Errorf("failed to scan story segment: %w", err)
		}
		segments = append(segments, models.StorySegment{Data: data, Type: msgType})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row error fetching story segments: %w", err)
	}

	return &models.AllStorySegments{StorySegments: segments}, nil
}
