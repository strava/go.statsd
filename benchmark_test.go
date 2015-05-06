package statsd

import (
	"testing"
	"time"
)

func BenchmarkCountMultiple(b *testing.B) {
	c, _ := NewTestClient("default")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.CountMultiple("metric", 10)
	}
}

func BenchmarkCountMultipleWithRate(b *testing.B) {
	c, _ := NewTestClient("default")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.CountMultiple("metric", 10, 0.999999)
	}
}

func BenchmarkMeasure(b *testing.B) {
	c, _ := NewTestClient("default")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Measure("metric", 123*time.Millisecond)
	}
}

func BenchmarkMeasureWithRate(b *testing.B) {
	c, _ := NewTestClient("default")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Measure("metric", 123*time.Millisecond, 0.999999)
	}
}
