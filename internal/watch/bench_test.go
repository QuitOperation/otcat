package watch

import (
	"math/rand"
	"testing"
	"time"
)

func BenchmarkFullJitterBackoff(b *testing.B) {
	rng := rand.New(rand.NewSource(1))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = fullJitterBackoff(rng, 100*time.Millisecond, 30*time.Second, (i%15)+1)
	}
}
