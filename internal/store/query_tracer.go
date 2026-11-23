package store

import (
	"context"
	"errors"
	"log/slog"

	"github.com/Folsik/Property-Management-Inspections/internal/requestctx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Query tracing records SQLSTATE and request correlation without SQL, args or
// database error details that may contain credentials or personal information.
type errorQueryTracer struct{}

func (errorQueryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	return ctx
}

func (errorQueryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	if data.Err == nil {
		return
	}
	var pgErr *pgconn.PgError
	if errors.As(data.Err, &pgErr) {
		slog.Error("database query failed", "request_id", requestctx.RequestID(ctx),
			"sqlstate", pgErr.Code, "constraint", pgErr.ConstraintName, "table", pgErr.TableName)
		return
	}
	slog.Error("database query failed", "request_id", requestctx.RequestID(ctx), "error_kind", "driver")
}
