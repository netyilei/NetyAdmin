package database

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// TestIsUniqueViolation 表驱动验证 23505 判定（含错误链包裹场景）。
func TestIsUniqueViolation(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "direct 23505", err: &pgconn.PgError{Code: "23505"}, want: true},
		{name: "other pg code", err: &pgconn.PgError{Code: "23503"}, want: false},
		{name: "wrapped 23505", err: fmt.Errorf("create user: %w", &pgconn.PgError{Code: "23505"}), want: true},
		{name: "plain error", err: errors.New("boom"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsUniqueViolation(tt.err); got != tt.want {
				t.Errorf("IsUniqueViolation(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
