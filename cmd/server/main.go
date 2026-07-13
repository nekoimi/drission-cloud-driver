package main

import (
	"flag"
	"log"

	"github.com/nekoimi/drission-cloud-driver/internal/app"
)

func main() {
	configPath := flag.String("config", "config/config.dev.yaml", "path to config file")
	flag.Parse()

	if err := app.Run(*configPath); err != nil {
		log.Fatalf("app stopped with error: %v", err)
	}
}
