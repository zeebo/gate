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
    Close causes any goroutines blocked in Wait, Start or Stop to exit.
    Close is safe to call multiple times as well as concurrently with any
    other method. The test is failed unless Close is called after every call
    to Wait, Stop or Start.

func (g *Gate) Closed() bool
    Closed returns if the Gate has been Closed.

func (g *Gate) Done() <-chan struct{}
    Done returns a channel that is closed whenever the Gate is Closed.

func (g *Gate) Protect(fn func())
    Protect launches the function in a goroutine. The function must exit
    normally in order for the test to pass. If the function does not exit
    normally, the test is failed, and the Gate behaves as if Close is
    called.

func (g *Gate) Run(fn func())
    Run launches the function in a goroutine, recording that calls to Stop
    should wait for it to call Wait. It must be called before any calls to
    Start or Stop. The function must exit normally in order for the test to
    pass. If the function does not exit normally, the test is failed, and
    the Gate behaves as if Close is called.

func (g *Gate) Start()
    Start should be called after Stop has returned. It is safe to call
    concurrently with Protect and Close.

func (g *Gate) Stop()
    Stop will block for the appropriate number of Wait calls. The Wait calls
    will remain blocked until a call to Start. It is not safe to call Stop
    and Start concurrently with each other, but it is safe to call
    concurrently with Wait, Protect, and Close.

func (g *Gate) Wait()
    Wait will block for the next call to Stop, and continue until a call to
    Start. It is safe to call Wait concurrently with itself, Protect, and
    Close.


