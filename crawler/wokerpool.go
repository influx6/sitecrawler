package crawler

import (
	"context"
	"sync"
	"sync/atomic"
)

// WorkerPool exposes a interface which provides the definition for a pool of
// workers that execute flat functions i.e functions with no arguments.
type WorkerPool interface {
	Stop()
	WaitOnStop()
	Add(func())
}

func NewWorkerPool(max int, ctx context.Context) WorkerPool {
	var pool workerPool
	pool.max = max
	pool.ctx = ctx
	pool.work = make(chan func(), 0)
	pool.close = make(chan struct{}, 0)
	pool.stopWorkers = make(chan struct{}, 0)
	return &pool
}

type workerPool struct {
	max           int
	totalWorkers  int64
	activeWorkers int64
	work          chan func()
	close         chan struct{}
	stopWorkers   chan struct{}
	ctx           context.Context
	wg            sync.WaitGroup
}

// WaitOnStop blocks till all workers have being closed.
func (w *workerPool) WaitOnStop() {
	w.wg.Wait()
}

// Stop sends a signal to close all workers within the pool.
func (w *workerPool) Stop() {
	total := int(atomic.LoadInt64(&w.totalWorkers))
	for i := 0; i < total; i++ {
		w.stopWorkers <- struct{}{}
	}

	close(w.close)
	w.wg.Wait()
}

func (w *workerPool) Add(fn func()) {
	total := int(atomic.LoadInt64(&w.totalWorkers))
	active := int(atomic.LoadInt64(&w.activeWorkers))

	var done <-chan struct{}
	if w.ctx != nil {
		done = w.ctx.Done()
	}

	if total < w.max {
		if active < total {
			select {
			case <-done:
				return
			case <-w.close:
				return
			case w.work <- fn:
				return
			}
		}

		w.wg.Add(1)
		go w.lunch()
	}

	select {
	case <-done:
		return
	case <-w.close:
		return
	case w.work <- fn:
		return
	}
}

// lunch sets up a worker for handling worker requests.
func (w *workerPool) lunch() {
	defer w.wg.Done()

	atomic.AddInt64(&w.totalWorkers, 1)
	defer atomic.AddInt64(&w.totalWorkers, -1)

	var done <-chan struct{}

	if w.ctx != nil {
		done = w.ctx.Done()
	}

	for {
		select {
		case <-done:
			return
		case <-w.stopWorkers:
			return
		case work, ok := <-w.work:
			if !ok {
				return
			}

			atomic.AddInt64(&w.activeWorkers, 1)
			work()
			atomic.AddInt64(&w.activeWorkers, -1)
		}
	}
}
