package CommonCrawl

import "sync"

type ChainedWorker[TIn, TOut any] struct {
	input     <-chan TIn
	submit    chan TIn // Non-nil only for a root stage.
	output    chan TOut
	workers   sync.WaitGroup
	done      chan struct{}
	closeOnce sync.Once
	stateMu   sync.Mutex
	enqueues  sync.WaitGroup
	closed    bool
}

func NewChainedWorker[TIn, TOut any](workerCount int, handler Handler2[TIn, TOut]) *ChainedWorker[TIn, TOut] {
	if workerCount < 1 {
		panic("workerCount must be positive")
	}

	input := make(chan TIn)

	stage := newChainedWorker(input, workerCount, handler)
	stage.submit = input

	return stage
}

// NewChainedWorkerFrom connects this stage to an upstream output channel.
func NewChainedWorkerFrom[TIn, TOut any](input <-chan TIn, workerCount int, handler Handler2[TIn, TOut]) *ChainedWorker[TIn, TOut] {
	if workerCount < 1 {
		panic("workerCount must be positive")
	}

	return newChainedWorker(input, workerCount, handler)
}

func newChainedWorker[TIn, TOut any](input <-chan TIn, workerCount int, handler Handler2[TIn, TOut]) *ChainedWorker[TIn, TOut] {
	stage := &ChainedWorker[TIn, TOut]{
		input:  input,
		output: make(chan TOut),
		done:   make(chan struct{}),
	}

	for i := 0; i < workerCount; i++ {
		stage.workers.Add(1)

		go func(id int) {
			defer stage.workers.Done()

			for item := range stage.input {
				handler(id, item, stage.output)
			}
		}(i + 1)
	}

	// Close output only after every worker has finished sending.
	go func() {
		stage.workers.Wait()
		close(stage.output)
		close(stage.done)
	}()

	return stage
}

// Enqueue submits input to a root stage.
func (s *ChainedWorker[TIn, TOut]) Enqueue(item TIn) {
	if s.submit == nil {
		panic("Enqueue is only valid on a root stage")
	}

	s.stateMu.Lock()
	if s.closed {
		s.stateMu.Unlock()
		panic("Enqueue called after CloseInput")
	}
	s.enqueues.Add(1)
	s.stateMu.Unlock()

	defer s.enqueues.Done()
	s.submit <- item
}

// CloseInput signals that no more input will arrive.
// Upstream-connected stages close automatically when their input closes.
func (s *ChainedWorker[TIn, TOut]) CloseInput() {
	if s.submit == nil {
		panic("CloseInput is only valid on a root stage")
	}

	s.closeOnce.Do(func() {
		s.stateMu.Lock()
		s.closed = true
		s.stateMu.Unlock()

		s.enqueues.Wait()
		close(s.submit)
	})
}

// Output exposes results for the next stage or final consumer.
func (s *ChainedWorker[TIn, TOut]) Output() <-chan TOut {
	return s.output
}

// Done closes after all workers finish and the output is closed.
func (s *ChainedWorker[TIn, TOut]) Done() <-chan struct{} {
	return s.done
}

// CloseAndWait gracefully drains a root stage.
func (s *ChainedWorker[TIn, TOut]) CloseAndWait() {
	s.CloseInput()
	<-s.done
}
