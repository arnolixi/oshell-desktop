package terminal

import (
	"io"
	"oshell-desktop/internal/terminal/internal/pty"
	"time"
)

// Drain output concurrently with process exit. ConPTY can keep its output
// pipe open after the child exits, until the console itself is closed.
func drainToolProcess(output io.Writer, proc *pty.PTY) int {
	drained := make(chan struct{})
	exited := make(chan int, 1)
	go func() {
		code, _ := proc.Wait()
		select {
		case <-drained:
		case <-time.After(100 * time.Millisecond):
		}
		proc.Close()
		exited <- code
	}()
	_, _ = io.Copy(output, proc)
	close(drained)
	return <-exited
}
