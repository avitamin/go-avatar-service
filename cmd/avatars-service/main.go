package main

import (
	"log"
	"os"

	"go-avatar-service/internal/app"
)

func main() {
	if err := app.Run(os.Args, os.Stdout); err != nil {
		log.Fatal(err)
	}
}
