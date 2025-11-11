package main

import (
	"log"

	"node-agent/app"
)

func main() {
	if err := app.Bootstrap(); err != nil {
		log.Fatalf("failed to start agent: %v", err)
	}
}
