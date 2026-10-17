package main

import (
	"log"
	"oshell-desktop/internal/bundled"
	"oshell-desktop/internal/desktop"
	"oshell-desktop/internal/terminal"
)

func main() {
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
