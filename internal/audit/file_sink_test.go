package audit

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readEvents(t *testing.T, path string) []Event {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	var out []Event
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev Event
		require.NoError(t, json.Unmarshal(line, &ev), "строка: %s", string(line))
		out = append(out, ev)
	}
	require.NoError(t, sc.Err())
	return out
}

func TestFileSink_WritesEventsAsJSONLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	sink, err := NewFileSink(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sink.Close() })

	events := []Event{
		{Timestamp: 1, Metrics: []string{"Alloc"}, IPAddress: "1.1.1.1"},
		{Timestamp: 2, Metrics: []string{"Frees", "PauseTotalNs"}, IPAddress: "2.2.2.2"},
	}
	for _, ev := range events {
		require.NoError(t, sink.Notify(context.Background(), ev))
	}
	require.NoError(t, sink.Close())

	assert.Equal(t, events, readEvents(t, path))
}

func TestFileSink_AppendsToExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	require.NoError(t, os.WriteFile(path, []byte("{\"ts\":0,\"metrics\":null,\"ip_address\":\"0\"}\n"), 0o644))

	sink, err := NewFileSink(path)
	require.NoError(t, err)
	require.NoError(t, sink.Notify(context.Background(), Event{Timestamp: 9, IPAddress: "9.9.9.9"}))
	require.NoError(t, sink.Close())

	got := readEvents(t, path)
	require.Len(t, got, 2)
	assert.Equal(t, int64(0), got[0].Timestamp)
	assert.Equal(t, int64(9), got[1].Timestamp)
}

func TestFileSink_ConcurrentWritesDoNotInterleave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	sink, err := NewFileSink(path)
	require.NoError(t, err)

	const writers = 16
	const perWriter = 32
	var wg sync.WaitGroup
	wg.Add(writers)
	for i := 0; i < writers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < perWriter; j++ {
				_ = sink.Notify(context.Background(), Event{
					Timestamp: int64(id*1000 + j),
					Metrics:   []string{"m"},
					IPAddress: "1.1.1.1",
				})
			}
		}(i)
	}
	wg.Wait()
	require.NoError(t, sink.Close())

	got := readEvents(t, path)
	assert.Len(t, got, writers*perWriter)
}

func TestFileSink_NotifyAfterCloseReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	sink, err := NewFileSink(path)
	require.NoError(t, err)
	require.NoError(t, sink.Close())

	err = sink.Notify(context.Background(), Event{Timestamp: 1})
	assert.ErrorIs(t, err, ErrSinkClosed)
}

func TestFileSink_RespectsCancelledContext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	sink, err := NewFileSink(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sink.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = sink.Notify(ctx, Event{Timestamp: 1})
	assert.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, readEvents(t, path))
}
