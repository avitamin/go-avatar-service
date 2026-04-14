package main

import (
	"log"

	"go-avatar-service/internal/app"
)

func main() {
	if err := app.Run([]string{"avatars-service", "worker"}, nil); err != nil {
		log.Fatal(err)
	}
}
