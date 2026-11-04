package CommonCrawl

import (
	"sync"
)

type WorkerPool[TIn any] struct {
	queue     chan TIn
	handler   Handler[TIn]
	wg        sync.WaitGroup
	enqueues  sync.WaitGroup
	stateMu   sync.Mutex
	closed    bool
	closeOnce sync.Once
}

func NewWorkerPool[TIn any](workerCount int, work Handler[TIn]) *WorkerPool[TIn] {
	if workerCount < 1 {
		panic("workerCount must be positive")
	}

	pool := &WorkerPool[TIn]{
		queue:   make(chan TIn),
		handler: work,
	}
	for i := 0; i < workerCount; i++ {
		pool.wg.Add(1)
		go pool.worker(i+1, pool.queue, &pool.wg)
	}

	return pool
}

func (p *WorkerPool[TIn]) worker(id int, jobQ <-chan TIn, wg *sync.WaitGroup) {
	defer wg.Done()

	for job := range jobQ {
		p.handler(id, job)
	}
}

func (p *WorkerPool[TIn]) Enqueue(item TIn) {
	p.stateMu.Lock()
	if p.closed {
		p.stateMu.Unlock()
		panic("Enqueue called after CloseAndWait")
	}
	p.enqueues.Add(1)
	p.stateMu.Unlock()

	defer p.enqueues.Done()
	p.queue <- item
}

func (p *WorkerPool[TIn]) CloseAndWait() {
	p.closeOnce.Do(func() {
		p.stateMu.Lock()
		p.closed = true
		p.stateMu.Unlock()

		p.enqueues.Wait()
		close(p.queue)
		p.wg.Wait()
	})
}
