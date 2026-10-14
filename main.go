package main

import (
	"log"
	"oshell-desktop/internal/desktop"
)

func main() {
	if err := forwardExternalLaunchIfRequested(); err != nil {
		showStartupError(err)
		log.Fatal(err)
	}
	if err := desktop.Run(); err != nil {
		showStartupError(err)
		log.Fatal(err)
	}
}
