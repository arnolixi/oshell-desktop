package terminal

import (
	"os"
	"time"
)

func init() {
	if os.Getenv("OSHELL_TOOL_STATE_HELPER") != "1" {
		return
	}
	time.Sleep(200 * time.Millisecond)
	os.Exit(0)
}
