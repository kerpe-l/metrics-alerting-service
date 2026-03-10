package pg

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Storage реализует интерфейс repository.Storage, используя PostgreSQL в качестве хранилища метрик.
type Storage struct {
	pool *pgxpool.Pool
}

// NewStorage создаёт новый Storage на базе пула соединений pgx.
func NewStorage(pool *pgxpool.Pool) *Storage {
	return &Storage{pool: pool}
}

func (d *Storage) UpdateGauge(name string, value float64) {
	_, _ = d.pool.Exec(context.Background(),
		`INSERT INTO metrics (id, type, value)
		 VALUES ($1, 'gauge', $2)
		 ON CONFLICT (id) DO UPDATE SET value = $2`,
		name, value,
	)
}

func (d *Storage) UpdateCounter(name string, value int64) {
	_, _ = d.pool.Exec(context.Background(),
		`INSERT INTO metrics (id, type, delta)
		 VALUES ($1, 'counter', $2)
		 ON CONFLICT (id) DO UPDATE SET delta = metrics.delta + $2`,
		name, value,
	)
}

func (d *Storage) GetGauge(name string) (float64, bool) {
	var val float64
	err := d.pool.QueryRow(context.Background(),
		`SELECT value FROM metrics WHERE id = $1 AND type = 'gauge'`,
		name,
	).Scan(&val)
	if err != nil {
		return 0, false
	}
	return val, true
}

func (d *Storage) GetCounter(name string) (int64, bool) {
	var val int64
	err := d.pool.QueryRow(context.Background(),
		`SELECT delta FROM metrics WHERE id = $1 AND type = 'counter'`,
		name,
	).Scan(&val)
	if err != nil {
		return 0, false
	}
	return val, true
}

func (d *Storage) GetAll() (map[string]float64, map[string]int64) {
	gauges := make(map[string]float64)
	counters := make(map[string]int64)

	rows, err := d.pool.Query(context.Background(),
		`SELECT id, type, delta, value FROM metrics`,
	)
	if err != nil {
		return gauges, counters
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

	return gauges, counters
}
