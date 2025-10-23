package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func (r *StoryDatabaseImpl) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.databaseclient.Pool.Begin(ctx)
}

func (r *StoryDatabaseImpl) RollbackTx(ctx context.Context, tx pgx.Tx) error {
	return tx.Rollback(ctx)
}

func (r *StoryDatabaseImpl) CommitTx(ctx context.Context, tx pgx.Tx) error {
	return tx.Commit(ctx)
}
