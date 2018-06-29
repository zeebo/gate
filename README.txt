PACKAGE DOCUMENTATION

package gate
    import "github.com/zeebo/gate"

    package gate helps with testing concurrent goroutines.

TYPES

type Gate struct {
    // contains filtered or unexported fields
}
    Gate helps manage and synchronize a pool of goroutines.

func New(t testing.TB) *Gate
    New constructs a Gate for some test.

func (g *Gate) Close()
    Close causes any goroutines blocked in Wait to silently exit, running
    defers. It also closes the done channel passed in to the workers
    allowing them to exit if they are not in Wait. Close is safe to call
    multiple times as well as concurrently with any other method. The test
    is failed if Stop or Start are called after Close.

func (g *Gate) Run(fn func(done chan struct{}))
    Run launches the function in a goroutine, recording that it will need to
    Wait for calls to Stop and Start. It should be called before any calls
    to Start or Stop. The function must exit normally in order for the test
    to pass. If the function does not exit normally, it behaves as if Close
    is called.

func (g *Gate) Start()
    Start should be called after Stop has returned.

func (g *Gate) Stop()
    Stop will block for the appropriate number of Wait calls. The Wait calls
    will remain blocked until a call to Start. It is not safe to call Stop
    and Start concurrently with each other, but it is safe to call
    concurrently with Wait and Close.

func (g *Gate) Wait()
    Wait will block for the next call to Stop, and continue until a call to
    Start. It is safe to call Wait concurrently with itself and Close.
