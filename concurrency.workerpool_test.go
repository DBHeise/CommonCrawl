package CommonCrawl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWorkerPool(t *testing.T) {
	tests := []struct {
		name        string
		workerCount int
		jobs        []int
	}{
		{
			name:        "no jobs",
			workerCount: 1,
			jobs:        nil,
		},
		{
			name:        "one job and worker",
			workerCount: 1,
			jobs:        []int{42},
		},
		{
			name:        "multiple jobs and workers",
			workerCount: 3,
			jobs:        []int{1, 2, 3, 4, 5, 6},
		},
		{
			name:        "duplicate jobs",
			workerCount: 2,
			jobs:        []int{7, 7, 9},
		},
	}

	type result struct {
		workerID int
		job      int
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := make(chan result, len(tt.jobs))
			pool := NewWorkerPool(tt.workerCount, func(workerID int, job int) {
				results <- result{workerID: workerID, job: job}
			})

			for _, job := range tt.jobs {
				pool.Enqueue(job)
			}
			pool.CloseAndWait()

			processedJobs := make([]int, 0, len(tt.jobs))
			for range tt.jobs {
				got := <-results
				processedJobs = append(processedJobs, got.job)
				assert.GreaterOrEqual(t, got.workerID, 1)
				assert.LessOrEqual(t, got.workerID, tt.workerCount)
			}

			assert.ElementsMatch(t, tt.jobs, processedJobs)
			assert.Empty(t, results)
		})
	}
}

func TestWorkerPoolInvalidWorkerCount(t *testing.T) {
	tests := []struct {
		name        string
		workerCount int
	}{
		{name: "zero", workerCount: 0},
		{name: "negative", workerCount: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Panics(t, func() {
				NewWorkerPool[int](tt.workerCount, func(int, int) {})
			})
		})
	}
}

func TestWorkerPoolCloseAndWait(t *testing.T) {
	pool := NewWorkerPool[int](1, func(int, int) {})
	pool.CloseAndWait()

	assert.NotPanics(t, pool.CloseAndWait)
	assert.Panics(t, func() {
		pool.Enqueue(1)
	})
}
