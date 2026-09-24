package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	maxbot "github.com/max-messenger/max-bot-api-client-go"

	"oss-max/internal/api"
	"oss-max/internal/bot"
	"oss-max/internal/config"
	"oss-max/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL не задан: API без базы не работает")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := storage.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	client, err := maxbot.New(cfg.BotToken, maxbot.WithBaseURL(cfg.APIBase))
	if err != nil {
		log.Fatalf("клиент MAX: %v", err)
	}
	info, err := client.Bots.GetBot(ctx)
	if err != nil {
		log.Fatalf("проверка токена: %v", err)
	}

	if cfg.DevMaxID != 0 {
		log.Printf("ВНИМАНИЕ: DEV_MAX_ID=%d — запросы без initData идут от этого пользователя. Только для разработки!", cfg.DevMaxID)
	}

	server := &http.Server{
		Addr: cfg.APIAddr,
		Handler: api.New(db, bot.NewNotifier(client, info.Username), api.Config{
			BotToken: cfg.BotToken,
			BotName:  info.Username,
			DevMaxID: cfg.DevMaxID,
		}).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()

	log.Printf("API слушает %s", cfg.APIAddr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
