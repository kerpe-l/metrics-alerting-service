package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kerpe-l/metrics-alerting-service/internal/model"
)

func TestMemStorage_GaugeRoundtrip(t *testing.T) {
	ctx := context.Background()
	st := NewMemStorage()

	require.NoError(t, st.UpdateGauge(ctx, "x", 1.5))
	require.NoError(t, st.UpdateGauge(ctx, "x", 2.5)) // перезапись
	v, ok := st.GetGauge(ctx, "x")
	require.True(t, ok)
	assert.InDelta(t, 2.5, v, 1e-9)

	_, ok = st.GetGauge(ctx, "missing")
	assert.False(t, ok)
}

func TestMemStorage_CounterAccumulates(t *testing.T) {
	ctx := context.Background()
	st := NewMemStorage()

	require.NoError(t, st.UpdateCounter(ctx, "c", 1))
	require.NoError(t, st.UpdateCounter(ctx, "c", 2))
	require.NoError(t, st.UpdateCounter(ctx, "c", 3))

	v, ok := st.GetCounter(ctx, "c")
	require.True(t, ok)
	assert.Equal(t, int64(6), v)

	_, ok = st.GetCounter(ctx, "missing")
	assert.False(t, ok)
}

func TestMemStorage_UpdateBatchMixed(t *testing.T) {
	ctx := context.Background()
	st := NewMemStorage()

	g := 4.2
	d := int64(7)
	err := st.UpdateBatch(ctx, []model.Metrics{
		{ID: "g1", MType: model.Gauge, Value: &g},
		{ID: "c1", MType: model.Counter, Delta: &d},
		{ID: "c1", MType: model.Counter, Delta: &d},
		{ID: "skipped", MType: model.Gauge, Value: nil},
	})
	require.NoError(t, err)

	gv, ok := st.GetGauge(ctx, "g1")
	require.True(t, ok)
	assert.InDelta(t, 4.2, gv, 1e-9)

	cv, ok := st.GetCounter(ctx, "c1")
	require.True(t, ok)
	assert.Equal(t, int64(14), cv)

	_, ok = st.GetGauge(ctx, "skipped")
	assert.False(t, ok)
}

func TestMemStorage_GetAllReturnsCopy(t *testing.T) {
	ctx := context.Background()
	st := NewMemStorage()
	require.NoError(t, st.UpdateGauge(ctx, "g", 1.0))
	require.NoError(t, st.UpdateCounter(ctx, "c", 5))

	gauges, counters := st.GetAll(ctx)
	assert.Len(t, gauges, 1)
	assert.Len(t, counters, 1)

	// Изменение возвращённой мапы не должно влиять на хранилище.
	gauges["g"] = 999
	v, _ := st.GetGauge(ctx, "g")
	assert.InDelta(t, 1.0, v, 1e-9)
}

func TestMemStorage_PingReturnsErrNoDB(t *testing.T) {
	st := NewMemStorage()
	assert.ErrorIs(t, st.Ping(context.Background()), ErrNoDB)
}
