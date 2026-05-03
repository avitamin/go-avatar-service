// Package main provides the legacy server-only compatibility entrypoint.
package main

import (
	"log"

	"go-avatar-service/internal/app"
)

func main() {
	if err := app.Run([]string{"avatars-service", "server"}, nil); err != nil {
		log.Fatal(err)
	}
}
