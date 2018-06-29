package gate

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestGate(t *testing.T) {
	t.Run("basic", Wrap(testGateBasic))
	t.Run("works", Wrap(testGateWorks))
	t.Run("fails", Wrap(testGateFails))
	t.Run("close", Wrap(testGateClose))
	t.Run("protect", Wrap(testGateProtect))
}

// testGateBasic tests basic properties of the Gate
func testGateBasic(t *test) {
	g := New(t)
	if g.Closed() {
		t.Fatal("gate already closed")
	}
	g.Close()
	if !g.Closed() {
		t.Fatal("gate didn't close")
	}
	select {
	case <-g.Done():
	default:
		t.Fatal("done channel not closed")
	}
}

// testGateWorks makes sure we can Wait.
func testGateWorks(t *test) {
	// construct a gate
	g := New(t)
	defer g.Close()

	// spawn the workers
	t.Run(g, func() { g.Wait(); g.Wait() })
	t.Run(g, func() { g.Wait(); g.Wait() })
	t.Run(g, func() { g.Wait(); g.Wait() })

	// cause all the goroutines to checkpoint
	g.Stop()
	g.Start()

	g.Stop()
	g.Start()
}

// testGateFails makes sure all the failures that can happen in a worker
// are captured and propogated up to the test.
func testGateFails(t *test) {
	// this test is expected to fail
	ft := &failingTest{TB: t}
	defer ft.Check()

	// construct a gate with the failing test wrapper
	g := New(ft)
	defer g.Close()

	// spawn the workers
	t.Run(g, g.Wait)
	t.Run(g, g.Wait)
	t.Run(g, g.Wait)

	// spawn extra workers that all fail. make sure it passes anyway
	t.Run(g, func() { ft.Fatal("error") })
	t.Run(g, func() { panic("some failure") })
	t.Run(g, func() { runtime.Goexit() })

	// cause all the goroutines to checkpoint
	g.Stop()
	g.Start()

	panic("unreachable")
}

// testGateClose makes sure that Close fails the test and cleans up appropriately.
func testGateClose(t *test) {
	// this test is expected to fail
	ft := &failingTest{TB: t}
	defer ft.Check()

	// construct a gate with the failing test wrapper
	g := New(ft)

	// spawn a worker trying to wait
	t.Run(g, func() {
		g.Wait()
		g.Wait()
	})

	// run a worker that Closes the gate
	t.Run(g, func() {
		g.Wait()
		g.Close()
		<-g.Done()
	})

	// wait for the first checkpoint
	g.Stop()
	g.Start()

	// start waiting for the second checkpoint
	g.Stop()

	panic("unreachable")
}

func testGateProtect(t *test) {
	// this test is expected to fail
	ft := &failingTest{TB: t}
	defer ft.Check()

	// construct a gate with the failing test wrapper
	g := New(ft)
	defer g.Close()

	// spawn extra workers that all fail. make sure it passes anyway
	t.Protect(g, func() { ft.Fatal("error") })
	t.Protect(g, func() { panic("some failure") })
	t.Protect(g, func() { runtime.Goexit() })

	// ensure all of the goroutines exit
	t.Check()
}

//
// helpers
//

// test is a helper wrapper for making tests succinct
type test struct {
	*testing.T
	*tracker
}

// wrap transforms a function from test to testing.T. it makes sure all of the
// goroutines are cleaned up when the test exits.
func Wrap(fn func(t *test)) func(*testing.T) {
	return func(t *testing.T) {
		t.Parallel()

		te := &test{
			T:       t,
			tracker: newTracker(t),
		}
		defer te.Check()

		fn(te)
	}
}

// Run is a helper for using Run, keeping track of the goroutine.
func (t *test) Run(g *Gate, fn func()) {
	t.Add()
	g.Run(func() { defer t.Done(); fn() })
}

// Protect is a helper for using Protect, keeping track of the goroutine.
func (t *test) Protect(g *Gate, fn func()) {
	t.Add()
	g.Protect(func() { defer t.Done(); fn() })
}

// failingTest wraps a TB and doesn't actually fail the test
// but instead fails if it hasn't failed!!
type failingTest struct {
	testing.TB
	failed bool
}

// failure is a unique type that we can check for
type failure struct{}

// check makes sure the test has failed.
func (n *failingTest) Check() {
	val := recover()
	if _, ok := val.(failure); val != nil && !ok {
		panic(val) // repanic anything not the failure type
	}

	if !n.failed {
		n.TB.Error("test expected to fail")
	}
}

// Failed returns if the test has "failed".
func (n *failingTest) Failed() bool {
	return n.failed
}

// Fail marks the "test" as "failed".
func (n *failingTest) Fail() {
	n.failed = true
}

// FailNow will "fail" the test and panic a unique type for check to catch.
func (n *failingTest) FailNow() {
	n.failed = true
	panic(failure{})
}

// Errorf "logs" an "error" and "fails" the test.
func (n *failingTest) Errorf(format string, args ...interface{}) {
	n.failed = true
}

// Error "logs" an "error" and "fails" the test.
func (n *failingTest) Error(args ...interface{}) {
	n.failed = true
}

// we use runtime.Goexit here since the Gate library does not call Fatal or
// Fatalf. This makes it so we can test these functions in a worker.

// Fatalf "logs" an "error" and "fails" thes test, exiting the goroutine.
func (n *failingTest) Fatalf(format string, args ...interface{}) {
	n.failed = true
	runtime.Goexit()
}

// Fatal "logs" an "error" and "fails" thes test, exiting the goroutine.
func (n *failingTest) Fatal(args ...interface{}) {
	n.failed = true
	runtime.Goexit()
}

// tracker keeps track of a number of goroutines
type tracker struct {
	t testing.TB

	mu     *sync.Mutex
	cond   *sync.Cond
	c      int
	failed bool
}

// newTracker constructs a tracker.
func newTracker(t testing.TB) *tracker {
	mu := new(sync.Mutex)
	return &tracker{
		t: t,

		mu:   mu,
		cond: sync.NewCond(mu),
	}
}

// add tells the tracker to keep track of a goroutine.
func (t *tracker) Add() {
	t.mu.Lock()
	t.c++
	t.mu.Unlock()
}

// done tells the tracker the goroutine is done.
func (t *tracker) Done() {
	t.mu.Lock()
	t.c--
	t.mu.Unlock()
	t.cond.Broadcast()
}

// check gives the runtime a minute to make sure all the goroutines clean up.
func (t *tracker) Check() {
	ti := time.AfterFunc(time.Minute, func() {
		t.mu.Lock()
		t.failed = true
		t.mu.Unlock()
		t.cond.Broadcast()
	})
	defer ti.Stop()

	t.mu.Lock()
	defer t.mu.Unlock()

	for t.c > 0 || t.failed {
		runtime.Gosched()
		t.cond.Wait()
	}

	if t.failed {
		t.t.Error("goroutines didn't clean up in time")
	}
}
