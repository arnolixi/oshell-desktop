package main

import (
	"log"
	"oshell-desktop/internal/bundled"
	"oshell-desktop/internal/core"
	"oshell-desktop/internal/desktop"
	"oshell-desktop/internal/terminal"
)

func main() {
	if handled, err := core.HandlePortableUpdate(); err != nil {
		showStartupError(err)
		log.Fatal(err)
	} else if handled {
		return
	}
	if err := forwardExternalLaunchIfRequested(); err != nil {
		showStartupError(err)
		log.Fatal(err)
	}
	if bundled.Enabled() {
		if err := terminal.Load(); err != nil {
			showStartupError(err)
			log.Fatal(err)
		}
	}
	if err := desktop.Run(); err != nil {
		showStartupError(err)
		log.Fatal(err)
	}
}
