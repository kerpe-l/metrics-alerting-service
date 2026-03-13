package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithPgx5Scheme(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		want string
	}{
		{
			name: "postgres:// заменяется на pgx5://",
			dsn:  "postgres://user:pass@localhost:5432/metrics",
			want: "pgx5://user:pass@localhost:5432/metrics",
		},
		{
			name: "postgresql:// заменяется на pgx5://",
			dsn:  "postgresql://user:pass@localhost:5432/metrics",
			want: "pgx5://user:pass@localhost:5432/metrics",
		},
		{
			name: "pgx5:// остаётся без изменений",
			dsn:  "pgx5://user:pass@localhost:5432/metrics",
			want: "pgx5://user:pass@localhost:5432/metrics",
		},
		{
			name: "postgres:// с параметрами",
			dsn:  "postgres://localhost:5432/metrics?sslmode=disable",
			want: "pgx5://localhost:5432/metrics?sslmode=disable",
		},
		{
			name: "пустая строка остаётся пустой",
			dsn:  "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := withPgx5Scheme(tt.dsn)
			assert.Equal(t, tt.want, got)
		})
	}
}
