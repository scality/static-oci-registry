package main

import (
	"log"
	"platform-static-registry/pkg/infrastructure/di"
)

func main() {
	container := di.NewContainer()

	httpServer := container.GetHTTPServer()

	err := httpServer.ListenAndServe()
	if err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
