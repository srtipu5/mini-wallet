// Package repository contains all SQL. Every function takes a DBTX so it can
// run either on the pool (simple reads) or inside a transaction (money moves).
package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBTX is satisfied by both *pgxpool.Pool and pgx.Tx.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Postgres error codes we react to.
const (
	pgUniqueViolation = "23505"
	pgCheckViolation  = "23514"
)

// pgErrorIs reports whether err is a Postgres error with the given code
// and (if constraint != "") on the given constraint.
func pgErrorIs(err error, code, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == code && (constraint == "" || pgErr.ConstraintName == constraint)
}
