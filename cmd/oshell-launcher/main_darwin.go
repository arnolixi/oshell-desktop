//go:build darwin

package main

import (
	"context"
	"fmt"
	"os"
	"oshell-desktop/internal/core"
	"time"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	request, e := core.ParseExternalLauncher(os.Args[0], os.Args[1:])
	if e != nil {
		return e
	}
	helper, e := os.Executable()
	if e != nil {
		return e
	}
	executable, e := core.LaunchApplicationExecutable(helper)
	if e != nil {
		return e
	}
	configuration, e := core.LauncherConfigurationDirectory()
	if e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	return core.LaunchApplicationAndSend(ctx, configuration, executable, request)
}
