package postgres

import (
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/oliverthies/fiapx-video-processor/internal/domain"
)

func TestIsUndefinedColumn(t *testing.T) {
	err := &pgconn.PgError{Code: "42703"}
	if !IsUndefinedColumn(err) {
		t.Fatal("bare 42703 should match")
	}
	if !IsUndefinedColumn(fmt.Errorf("find: %w", err)) {
		t.Fatal("wrapped 42703 should match")
	}
	if IsUndefinedColumn(domain.ErrJobNotFound) {
		t.Fatal("missing job is not a schema mismatch")
	}
	if IsUndefinedColumn(&pgconn.PgError{Code: "23505"}) {
		t.Fatal("unique violation is not a schema mismatch")
	}
}
