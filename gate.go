// package gate helps with testing concurrent goroutines.
package gate

import (
	"runtime"
	"sync"
	"testing"
)

// Gate helps manage and synchronize a pool of goroutines.
type Gate struct {
	t       testing.TB
	workers int

	mu     *sync.Mutex
	cond   *sync.Cond
	queue  chan struct{}
	done   chan struct{}
	run    bool
	closed bool
}

// New constructs a Gate for some test.
func New(t testing.TB) *Gate {
	mu := new(sync.Mutex)
	return &Gate{
		t: t,

		mu:   mu,
		cond: sync.NewCond(mu),
		done: make(chan struct{}),
	}
}

// Done returns a channel that is closed whenever the Gate is Closed.
func (g *Gate) Done() <-chan struct{} { return g.done }

// Close causes any goroutines blocked in Wait, Start or Stop to exit. Close is safe to call
// multiple times as well as concurrently with any other method. The test is failed unless
// Close is called after every call to Wait, Stop or Start.
func (g *Gate) Close() {
	g.mu.Lock()
	g.closeLocked()
	g.mu.Unlock()
	g.cond.Broadcast()
}

// Closed returns if the Gate has been Closed.
func (g *Gate) Closed() bool {
	g.mu.Lock()
	out := g.closed
	g.mu.Unlock()
	return out
}

// closeLocked implements the logic for closing that should happen under the mutex.
func (g *Gate) closeLocked() {
	if !g.closed {
		close(g.done)
	}
	g.closed = true
	if g.queue != nil {
		defer func() { recover() }()
		close(g.queue)
	}
}

// Run launches the function in a goroutine, recording that calls to Stop should wait for
// it to call Wait. It must be called before any calls to Start or Stop.
// The function must exit normally in order for the test to pass. If the function does
// not exit normally, the test is failed, and the Gate behaves as if Close is called.
func (g *Gate) Run(fn func()) {
	g.workers++
	g.Protect(fn)
}

// Protect launches the function in a goroutine. The function must exit normally in order for
// the test to pass. If the function does not exit normally, the test is failed, and the Gate
// behaves as if Close is called.
func (g *Gate) Protect(fn func()) {
	go func() {
		normal := false
		defer func() {
			// if we returned normally, no problem.
			if normal {
				return
			}

			// always recover any panics
			rec := recover()

			g.mu.Lock()
			defer g.mu.Unlock()

			// make sure the test is flagged as failed, and error if there was a panic
			if !g.t.Failed() || rec != nil {
				g.t.Errorf("error: goroutine exited abnormally. recover=%v", rec)
			}

			// close the gate
			g.closeLocked()
			g.cond.Broadcast()
		}()

		fn()
		normal = true
	}()
}

// Wait will block for the next call to Stop, and continue until a call to Start. It is
// safe to call Wait concurrently with itself, Protect, and Close.
func (g *Gate) Wait() {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Wait for a queue to be present and run to be false.
	for !g.closed && (g.queue == nil || g.run) {
		g.cond.Wait()
	}
	if g.closed {
		g.t.Fail()
		runtime.Goexit()
	}
	g.queue <- struct{}{}

	// Wait for run and signal we're ready to start.
	for !g.closed && !g.run {
		g.cond.Wait()
	}
	if g.closed {
		g.t.Fail()
		runtime.Goexit()
	}
	g.queue <- struct{}{}
}

// checkFailedAnd acquires the lock, checks if either the test is failed or if the gate has
// been closed. If either are true, the goroutine is exited with FailNow. Otherwise, the
// provided function is called if non-nil.
func (g *Gate) checkFailedAnd(fn func()) {
	g.mu.Lock()
	if g.t.Failed() || g.closed {
		g.mu.Unlock()
		g.t.FailNow()
	}
	if fn != nil {
		fn()
	}
	g.mu.Unlock()
}

// Stop will block for the appropriate number of Wait calls. The Wait calls will remain blocked
// until a call to Start. It is not safe to call Stop and Start concurrently with each other,
// but it is safe to call concurrently with Wait, Protect, and Close.
func (g *Gate) Stop() {
	// make sure Stop will eventually proceed because some workers can possibly exist.
	g.checkFailedAnd(func() {
		g.run = false
		g.queue = make(chan struct{}, g.workers)
	})

	// signal and wait for enough calls to Wait.
	g.cond.Broadcast()
	for i := 0; i < g.workers; i++ {
		<-g.queue
	}

	// make sure we aren't just worken up because the queue has been closed.
	g.checkFailedAnd(nil)
}

// Start should be called after Stop has returned. It is safe to call concurrently with
// Protect and Close.
func (g *Gate) Start() {
	// start up the waiting workers
	g.checkFailedAnd(func() {
		g.run = true
	})

	// signal and wait for them to be woken up.
	g.cond.Broadcast()
	for i := 0; i < g.workers; i++ {
		<-g.queue
	}

	// clean up our mess with the queue. because some workers may be running,
	// Close may have happened.
	g.checkFailedAnd(func() {
		g.run = false
		g.queue = nil
	})
}
