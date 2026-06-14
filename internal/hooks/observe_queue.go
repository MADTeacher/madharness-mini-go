package hooks

import "sync"

type observeJob struct {
	provider Provider
	event    Event
}

type observeQueue struct {
	jobs   chan observeJob
	handle func(Provider, Event) Decision
	wg     sync.WaitGroup
	mu     sync.Mutex
	closed bool
}

func newObserveQueue(handle func(Provider, Event) Decision) *observeQueue {
	queue := &observeQueue{
		jobs:   make(chan observeJob, ObserveQueueSize),
		handle: handle,
	}
	go queue.run()
	return queue
}

func (q *observeQueue) Enqueue(job observeJob) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return false
	}
	q.wg.Add(1)
	select {
	case q.jobs <- job:
		return true
	default:
		q.wg.Done()
		return false
	}
}

func (q *observeQueue) Close() {
	q.mu.Lock()
	if !q.closed {
		q.closed = true
		close(q.jobs)
	}
	q.mu.Unlock()
	q.wg.Wait()
}

func (q *observeQueue) run() {
	for job := range q.jobs {
		q.handle(job.provider, job.event)
		q.wg.Done()
	}
}
