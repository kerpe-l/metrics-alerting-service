package pg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kerpe-l/metrics-alerting-service/internal/model"
	"github.com/kerpe-l/metrics-alerting-service/internal/retry"
)

// Storage реализует интерфейс repository.Storage, используя PostgreSQL в качестве хранилища метрик.
type Storage struct {
	pool *pgxpool.Pool
}

// NewStorage создаёт новый Storage на базе пула соединений pgx.
func NewStorage(pool *pgxpool.Pool) *Storage {
	return &Storage{pool: pool}
}

// isRetriablePgError проверяет, является ли ошибка PostgreSQL retriable:
// Class 08 — Connection Exception, Class 40 — Transaction Rollback (deadlock, serialization failure)
func isRetriablePgError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return strings.HasPrefix(pgErr.Code, "08") || strings.HasPrefix(pgErr.Code, "40")
	}
	return false
}

func (d *Storage) Ping(ctx context.Context) error {
	return d.pool.Ping(ctx)
}

func (d *Storage) UpdateGauge(ctx context.Context, name string, value float64) error {
	return retry.Do(func() error {
		_, err := d.pool.Exec(ctx,
			`INSERT INTO metrics (id, type, value, updated_at)
			 VALUES ($1, 'gauge', $2, NOW())
			 ON CONFLICT (id) DO UPDATE SET value = $2, updated_at = NOW()`,
			name, value,
		)
		return err
	}, isRetriablePgError)
}

func (d *Storage) UpdateCounter(ctx context.Context, name string, value int64) error {
	return retry.Do(func() error {
		_, err := d.pool.Exec(ctx,
			`INSERT INTO metrics (id, type, delta, updated_at)
			 VALUES ($1, 'counter', $2, NOW())
			 ON CONFLICT (id) DO UPDATE SET delta = metrics.delta + $2, updated_at = NOW()`,
			name, value,
		)
		return err
	}, isRetriablePgError)
}

func (d *Storage) UpdateBatch(ctx context.Context, metrics []model.Metrics) error {
	return retry.Do(func() error {
		tx, err := d.pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx: %w", err)
		}
		defer tx.Rollback(ctx)

		for _, m := range metrics {
			switch m.MType {
			case model.Gauge:
				if m.Value != nil {
					_, err = tx.Exec(ctx,
						`INSERT INTO metrics (id, type, value, updated_at)
						 VALUES ($1, 'gauge', $2, NOW())
						 ON CONFLICT (id) DO UPDATE SET value = $2, updated_at = NOW()`,
						m.ID, *m.Value,
					)
				}
			case model.Counter:
				if m.Delta != nil {
					_, err = tx.Exec(ctx,
						`INSERT INTO metrics (id, type, delta, updated_at)
						 VALUES ($1, 'counter', $2, NOW())
						 ON CONFLICT (id) DO UPDATE SET delta = metrics.delta + $2, updated_at = NOW()`,
						m.ID, *m.Delta,
					)
				}
			}
			if err != nil {
				return fmt.Errorf("exec metric %s: %w", m.ID, err)
			}
		}

		return tx.Commit(ctx)
	}, isRetriablePgError)
}

func (d *Storage) GetGauge(ctx context.Context, name string) (float64, bool) {
	var val float64
	err := retry.Do(func() error {
		return d.pool.QueryRow(ctx,
			`SELECT value FROM metrics WHERE id = $1 AND type = 'gauge'`,
			name,
		).Scan(&val)
	}, isRetriablePgError)
	if err != nil {
		return 0, false
	}
	return val, true
}

func (d *Storage) GetCounter(ctx context.Context, name string) (int64, bool) {
	var val int64
	err := retry.Do(func() error {
		return d.pool.QueryRow(ctx,
			`SELECT delta FROM metrics WHERE id = $1 AND type = 'counter'`,
			name,
		).Scan(&val)
	}, isRetriablePgError)
	if err != nil {
		return 0, false
	}
	return val, true
}

func (d *Storage) GetAll(ctx context.Context) (map[string]float64, map[string]int64) {
	gauges := make(map[string]float64)
	counters := make(map[string]int64)

	err := retry.Do(func() error {
		rows, err := d.pool.Query(ctx,
			`SELECT id, type, delta, value FROM metrics`,
		)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var (
				id    string
				mType string
				delta *int64
				value *float64
			)
			if err := rows.Scan(&id, &mType, &delta, &value); err != nil {
				continue
			}
			switch mType {
			case "gauge":
				if value != nil {
					gauges[id] = *value
				}
			case "counter":
				if delta != nil {
					counters[id] = *delta
				}
			}
		}
		return rows.Err()
	}, isRetriablePgError)

	if err != nil {
		return make(map[string]float64), make(map[string]int64)
	}
	return gauges, counters
}
