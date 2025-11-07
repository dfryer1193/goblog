package db

import (
	"context"
	"database/sql"
	"fmt"
)

// txKey is the key type for storing transaction in context
type txKey struct{}

// WithTx returns a new context with the transaction attached
func WithTx(ctx context.Context, tx *sql.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// GetTx retrieves the transaction from context if it exists
func GetTx(ctx context.Context) (*sql.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(*sql.Tx)
	return tx, ok
}

// GetExecutor returns either a transaction from context or the base db connection
// This allows repositories to execute queries within a transaction if one exists
func GetExecutor(ctx context.Context, db *sql.DB) interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
} {
	if tx, ok := GetTx(ctx); ok {
		return tx
	}
	return db
}

// RunInTransaction executes a function within a database transaction
// If a transaction already exists in the context, it reuses that transaction
// and does not commit or rollback (delegating that to the outer transaction)
// If no transaction exists, it creates one, and commits or rolls back based on the result
func RunInTransaction(ctx context.Context, db *sql.DB, fn func(ctx context.Context) error) error {
	// Check if we're already in a transaction
	if _, ok := GetTx(ctx); ok {
		// Reuse existing transaction - no commit/rollback
		return fn(ctx)
	}

	// Start a new transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Create a new context with the transaction
	txCtx := WithTx(ctx, tx)

	// Execute the function
	if err := fn(txCtx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("failed to rollback transaction after error %v: %w", err, rbErr)
		}
		return err
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
