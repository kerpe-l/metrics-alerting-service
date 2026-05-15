package agent

import (
	"testing"
)

func BenchmarkCollector_Collect(b *testing.B) {
	c := NewCollector()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		c.Collect()
	}
}

func BenchmarkCollector_Metrics(b *testing.B) {
	c := NewCollector()
	c.Collect()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = c.Metrics()
	}
}
