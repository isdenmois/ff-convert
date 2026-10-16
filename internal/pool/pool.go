// Package pool provides a bounded worker pool used to run conversions in
// parallel.
package pool

import (
	"os"
	"runtime"
	"strconv"
	"sync"
)

// EnvConcurrency is the environment variable overriding the pool size.
const EnvConcurrency = "FF_CONVERT_CONCURRENCY"

// Pool runs jobs with at most Limit running at once.
type Pool struct {
	limit int
	jobs  chan func()

	mu      sync.Mutex
	running int
	wg      sync.WaitGroup
}

// New creates a pool with the given concurrency limit (minimum 1).
func New(limit int) *Pool {
	if limit < 1 {
		limit = 1
	}
	p := &Pool{limit: limit, jobs: make(chan func())}
	for i := 0; i < limit; i++ {
		go p.worker()
	}
	return p
}

// DefaultLimit returns the default concurrency: $FF_CONVERT_CONCURRENCY or
// half the CPU count (minimum 1).
func DefaultLimit() int {
	if v := os.Getenv(EnvConcurrency); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	n := runtime.NumCPU() / 2
	if n < 1 {
		n = 1
	}
	return n
}

// NewDefault creates a pool sized by DefaultLimit.
func NewDefault() *Pool { return New(DefaultLimit()) }

// Limit returns the configured concurrency limit.
func (p *Pool) Limit() int { return p.limit }

// Go submits a job. The call blocks until a worker is available to accept
// it; the job itself runs asynchronously. Use Wait to wait for all submitted
// jobs.
func (p *Pool) Go(job func()) {
	p.wg.Add(1)
	p.jobs <- job
}

func (p *Pool) worker() {
	for job := range p.jobs {
		p.mu.Lock()
		p.running++
		p.mu.Unlock()
		job()
		p.mu.Lock()
		p.running--
		p.mu.Unlock()
		p.wg.Done()
	}
}

// Wait blocks until all submitted jobs have finished.
func (p *Pool) Wait() { p.wg.Wait() }
