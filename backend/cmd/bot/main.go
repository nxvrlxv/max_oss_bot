package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	maxbot "github.com/max-messenger/max-bot-api-client-go"

	"oss-max/internal/bot"
	"oss-max/internal/config"
	"oss-max/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	api, err := maxbot.New(cfg.BotToken, maxbot.WithBaseURL(cfg.APIBase))
	if err != nil {
		log.Fatalf("клиент MAX: %v", err)
	}

	// Ошибки поллинга библиотека складывает в канал; без читателя они теряются.
	go func() {
		for err := range api.GetErrors() {
			log.Printf("MAX API: %v", err)
		}
	}()

	info, err := api.Bots.GetBot(ctx)
	if err != nil {
		log.Fatalf("проверка токена: %v", err)
	}
	log.Printf("бот @%s (%d)", info.Username, info.UserId)

	var store bot.Store
	if cfg.DatabaseURL == "" {
		log.Print("DATABASE_URL не задан — данные в памяти, собраний не будет")
		store = bot.NewMemoryStore()
	} else {
		db, err := storage.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()
		store = db
	}

	b := bot.New(api, cfg, store, info.Username)
	if err := b.SetCommands(ctx); err != nil {
		log.Printf("список команд: %v", err)
	}

	if err := b.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}
