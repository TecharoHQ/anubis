package lib

import (
	"fmt"
	"sync"
	"testing"
)

func TestMetricHostBound(t *testing.T) {
	labels := &metricHostLabels{}
	var wg sync.WaitGroup
	seen := sync.Map{}
	for n := 0; n < 1000; n++ {
		wg.Go(func() { seen.Store(labels.label(fmt.Sprintf("host-%d.example", n)), true) })
	}
	wg.Wait()
	count := 0
	seen.Range(func(_, _ any) bool { count++; return true })
	if count > 65 {
		t.Fatalf("unbounded labels: %d", count)
	}
	if labels.label("another.example") != "_other" {
		t.Fatal("overflow host not aggregated")
	}
}
