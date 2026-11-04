package CommonCrawl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type chainedWorkerResult struct {
	workerID int
	value    int
}

func TestChainedWorker(t *testing.T) {
	tests := []struct {
		name        string
		workerCount int
		inputs      []int
	}{
		{
			name:        "no inputs",
			workerCount: 1,
		},
		{
			name:        "one input and worker",
			workerCount: 1,
			inputs:      []int{21},
		},
		{
			name:        "multiple inputs and workers",
			workerCount: 3,
			inputs:      []int{1, 2, 3, 4, 5, 6},
		},
		{
			name:        "duplicate inputs",
			workerCount: 2,
			inputs:      []int{7, 7, 9},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stage := NewChainedWorker[int, chainedWorkerResult](
				tt.workerCount,
				func(workerID int, input int, output chan chainedWorkerResult) {
					output <- chainedWorkerResult{workerID: workerID, value: input * 2}
				},
			)

			results := make(chan chainedWorkerResult, len(tt.inputs))
			go func() {
				for result := range stage.Output() {
					results <- result
				}
				close(results)
			}()

			for _, input := range tt.inputs {
				stage.Enqueue(input)
			}
			stage.CloseInput()
			stage.CloseInput()
			stage.CloseAndWait()
			assert.NotPanics(t, stage.CloseAndWait)

			gotValues := make([]int, 0, len(tt.inputs))
			for result := range results {
				gotValues = append(gotValues, result.value)
				assert.GreaterOrEqual(t, result.workerID, 1)
				assert.LessOrEqual(t, result.workerID, tt.workerCount)
			}

			wantValues := make([]int, 0, len(tt.inputs))
			for _, input := range tt.inputs {
				wantValues = append(wantValues, input*2)
			}
			assert.ElementsMatch(t, wantValues, gotValues)

			<-stage.Done()
			_, open := <-stage.Output()
			assert.False(t, open, "output channel should be closed after CloseAndWait")
		})
	}
}

func TestChainedWorkerFrom(t *testing.T) {
	tests := []struct {
		name        string
		workerCount int
		inputs      []int
	}{
		{
			name:        "no inputs",
			workerCount: 1,
		},
		{
			name:        "one input and worker",
			workerCount: 1,
			inputs:      []int{21},
		},
		{
			name:        "multiple inputs and workers",
			workerCount: 3,
			inputs:      []int{1, 2, 3, 4, 5, 6},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := make(chan int, len(tt.inputs))
			for _, value := range tt.inputs {
				input <- value
			}
			close(input)

			stage := NewChainedWorkerFrom[int, chainedWorkerResult](
				input,
				tt.workerCount,
				func(workerID int, value int, output chan chainedWorkerResult) {
					output <- chainedWorkerResult{workerID: workerID, value: value * 2}
				},
			)

			results := make([]chainedWorkerResult, 0, len(tt.inputs))
			for result := range stage.Output() {
				results = append(results, result)
			}

			gotValues := make([]int, 0, len(results))
			for _, result := range results {
				gotValues = append(gotValues, result.value)
				assert.GreaterOrEqual(t, result.workerID, 1)
				assert.LessOrEqual(t, result.workerID, tt.workerCount)
			}

			wantValues := make([]int, 0, len(tt.inputs))
			for _, value := range tt.inputs {
				wantValues = append(wantValues, value*2)
			}
			assert.ElementsMatch(t, wantValues, gotValues)

			<-stage.Done()
		})
	}
}

func TestChainedWorkerInvalidWorkerCount(t *testing.T) {
	tests := []struct {
		name    string
		workers int
	}{
		{name: "zero", workers: 0},
		{name: "negative", workers: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Panics(t, func() {
				NewChainedWorker[int, int](tt.workers, func(int, int, chan int) {})
			})

			assert.Panics(t, func() {
				NewChainedWorkerFrom[int, int](make(chan int), tt.workers, func(int, int, chan int) {})
			})
		})
	}
}

func TestChainedWorkerFromRootOnlyMethods(t *testing.T) {
	input := make(chan int)
	stage := NewChainedWorkerFrom[int, int](input, 1, func(int, int, chan int) {})

	tests := []struct {
		name string
		call func()
	}{
		{name: "Enqueue", call: func() { stage.Enqueue(1) }},
		{name: "CloseInput", call: stage.CloseInput},
		{name: "CloseAndWait", call: stage.CloseAndWait},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Panics(t, tt.call)
		})
	}

	close(input)
	<-stage.Done()
}

func TestChainedWorkerEnqueueAfterClose(t *testing.T) {
	stage := NewChainedWorker[int, int](1, func(int, int, chan int) {})
	stage.CloseAndWait()

	assert.Panics(t, func() {
		stage.Enqueue(1)
	})
}
