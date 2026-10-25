package storage

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBTX is the database execution surface shared by *pgxpool.Pool and pgx.Tx.
// Repositories take it instead of the concrete pool so that callers can run
// the same statements inside a transaction (e.g. "insert delivery row + queue
// its River job atomically" via WithTx).
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
