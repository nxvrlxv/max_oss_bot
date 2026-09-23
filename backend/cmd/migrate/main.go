package main

import (
	"context"
	"log"

	"oss-max/internal/config"
	"oss-max/internal/storage"
)

func main() {
	url, err := config.DatabaseURL()
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	db, err := storage.Open(ctx, url)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Migrate(ctx); err != nil {
		log.Fatal(err)
	}
	log.Print("схема актуальна")
}
