package pg

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

func TestIsRetriablePgError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "Class 08 — connection exception",
			err:  &pgconn.PgError{Code: "08006"},
			want: true,
		},
		{
			name: "Class 08 — connection failure",
			err:  &pgconn.PgError{Code: "08001"},
			want: true,
		},
		{
			name: "Class 40 — deadlock detected",
			err:  &pgconn.PgError{Code: "40P01"},
			want: true,
		},
		{
			name: "Class 40 — serialization failure",
			err:  &pgconn.PgError{Code: "40001"},
			want: true,
		},
		{
			name: "Class 23 — not null violation (не retriable)",
			err:  &pgconn.PgError{Code: "23502"},
			want: false,
		},
		{
			name: "Class 23 — unique violation (не retriable)",
			err:  &pgconn.PgError{Code: "23505"},
			want: false,
		},
		{
			name: "обычная ошибка (не PgError)",
			err:  errors.New("something went wrong"),
			want: false,
		},
		{
			name: "обёрнутая Class 08 ошибка — errors.As разворачивает",
			err:  fmt.Errorf("exec failed: %w", &pgconn.PgError{Code: "08003"}),
			want: true,
		},
		{
			name: "обёрнутая обычная ошибка",
			err:  fmt.Errorf("query failed: %w", errors.New("timeout")),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isRetriablePgError(tt.err))
		})
	}
}
