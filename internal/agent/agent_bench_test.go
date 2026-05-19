package agent

import (
	"testing"
)

func BenchmarkCollector_Collect(b *testing.B) {
	c := NewCollector()
	b.ReportAllocs()
	for b.Loop() {
		c.Collect()
	}
}

func BenchmarkCollector_Metrics(b *testing.B) {
	c := NewCollector()
	c.Collect()

	b.ReportAllocs()
	for b.Loop() {
		_ = c.Metrics()
	}
}
