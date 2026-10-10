package pool

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBoundedConcurrency(t *testing.T) {
	var cur, max atomic.Int32
	var mu sync.Mutex
	done := make(chan struct{})

	p := New(3)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		p.Go(func() {
			defer wg.Done()
			n := cur.Add(1)
			mu.Lock()
			if n > max.Load() {
				max.Store(n)
			}
			mu.Unlock()
			time.Sleep(20 * time.Millisecond)
			cur.Add(-1)
		})
	}
	go func() {
		p.Wait()
		close(done)
	}()
	<-done
	wg.Wait()
	if max.Load() > 3 {
		t.Fatalf("observed %d concurrent jobs, limit is 3", max.Load())
	}
	if max.Load() < 2 {
		t.Fatalf("jobs did not overlap (max=%d), pool is serial", max.Load())
	}
}

func TestDefaultLimit(t *testing.T) {
	t.Setenv(EnvConcurrency, "5")
	if got := DefaultLimit(); got != 5 {
		t.Fatalf("DefaultLimit = %d, want 5", got)
	}
	t.Setenv(EnvConcurrency, "bogus")
	if got := DefaultLimit(); got < 1 {
		t.Fatalf("DefaultLimit = %d, want >= 1", got)
	}
	if got := New(0).Limit(); got != 1 {
		t.Fatalf("New(0).Limit = %d, want 1", got)
	}
}
