package config

import (
	"errors"
	"os"
	"strconv"
)

// Config — все настройки процесса. Читается один раз при старте,
// os.Getenv по коду больше нигде не вызывается.
type Config struct {
	BotToken string // токен бота, выдан организаторами
	BotMode  string // polling для разработки, webhook для прода
	APIBase  string // базовый адрес Bot API

	WebAppURL string // адрес мини-приложения; в кнопки не уходит, задаётся в настройках бота на платформе MAX

	WebhookURL    string // куда MAX шлёт события в режиме webhook
	WebhookSecret string // приходит в заголовке X-Max-Bot-Api-Secret
	WebhookAddr   string // адрес, который слушает сам бот

	DatabaseURL string

	APIAddr  string // адрес, который слушает API мини-приложения
	DevMaxID int64  // запросы без initData идут от этого пользователя; только для разработки
}

const (
	ModePolling = "polling"
	ModeWebhook = "webhook"

	// Адрес по умолчанию: сертификат platform-api2 выпущен УЦ Минцифры,
	// без его установки в систему соединение обрывается.
	defaultAPIBase = "https://platform-api.max.ru/"
)

func Load() (Config, error) {
	cfg := Config{
		BotToken:      os.Getenv("BOT_TOKEN"),
		BotMode:       env("BOT_MODE", ModePolling),
		APIBase:       env("MAX_API_BASE", defaultAPIBase),
		WebAppURL:     os.Getenv("WEBAPP_URL"),
		WebhookURL:    os.Getenv("WEBHOOK_URL"),
		WebhookSecret: os.Getenv("WEBHOOK_SECRET"),
		WebhookAddr:   env("WEBHOOK_ADDR", ":8081"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		APIAddr:       env("API_ADDR", ":8080"),
	}

	if dev := os.Getenv("DEV_MAX_ID"); dev != "" {
		id, err := strconv.ParseInt(dev, 10, 64)
		if err != nil {
			return cfg, errors.New("DEV_MAX_ID должен быть числом")
		}
		cfg.DevMaxID = id
	}

	if cfg.BotToken == "" {
		return cfg, errors.New("BOT_TOKEN не задан")
	}

	if cfg.BotMode == ModeWebhook {
		if cfg.WebhookURL == "" {
			return cfg, errors.New("BOT_MODE=webhook, но WEBHOOK_URL не задан")
		}
		if cfg.WebhookSecret == "" {
			return cfg, errors.New("BOT_MODE=webhook, но WEBHOOK_SECRET не задан")
		}
	}

	return cfg, nil
}

// DatabaseURL — для процессов, которым нужна только база: миграций и сида.
// Load для них не подходит — требует токен бота.
func DatabaseURL() (string, error) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return "", errors.New("DATABASE_URL не задан")
	}
	return url, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
