package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/kerpe-l/metrics-alerting-service/internal/model"
)

func benchBatch(n int) []model.Metrics {
	metrics := make([]model.Metrics, 0, n)
	for i := 0; i < n; i++ {
		v := float64(i)
		d := int64(i)
		if i%2 == 0 {
			metrics = append(metrics, model.Metrics{
				ID: fmt.Sprintf("g%d", i), MType: model.Gauge, Value: &v,
			})
		} else {
			metrics = append(metrics, model.Metrics{
				ID: fmt.Sprintf("c%d", i), MType: model.Counter, Delta: &d,
			})
		}
	}
	return metrics
}

func BenchmarkMemStorage_UpdateBatch(b *testing.B) {
	ctx := context.Background()
	st := NewMemStorage()
	batch := benchBatch(50)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = st.UpdateBatch(ctx, batch)
	}
}

func BenchmarkMemStorage_GetAll(b *testing.B) {
	ctx := context.Background()
	st := NewMemStorage()
	for i := 0; i < 200; i++ {
		_ = st.UpdateGauge(ctx, fmt.Sprintf("g%d", i), float64(i))
		_ = st.UpdateCounter(ctx, fmt.Sprintf("c%d", i), int64(i))
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = st.GetAll(ctx)
	}
}
