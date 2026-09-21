package bot

import (
	"context"
	"log"
	"net/http"
	"time"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"oss-max/internal/config"
)

type Bot struct {
	api   *maxbot.Api
	cfg   config.Config
	store Store
}

func New(api *maxbot.Api, cfg config.Config, store Store) *Bot {
	return &Bot{api: api, cfg: cfg, store: store}
}

// Run получает события и передаёт их роутеру. Транспорт выбирается
// по BOT_MODE, но дальше по коду разницы нет: и там и там канал апдейтов.
func (b *Bot) Run(ctx context.Context) error {
	updates, err := b.updates(ctx)
	if err != nil {
		return err
	}

	for update := range updates {
		// Обработчик не должен задерживать приём: MAX повторит
		// недоставленное событие, если ответа долго нет.
		go func(update schemes.UpdateInterface) {
			if err := b.Handle(ctx, update); err != nil {
				log.Printf("обработка %T: %v", update, err)
			}
		}(update)
	}

	return ctx.Err()
}

func (b *Bot) updates(ctx context.Context) (<-chan schemes.UpdateInterface, error) {
	if b.cfg.BotMode != config.ModeWebhook {
		log.Print("режим long polling")
		return b.api.GetUpdates(ctx), nil
	}

	if err := b.subscribe(ctx); err != nil {
		return nil, err
	}

	updates := make(chan schemes.UpdateInterface, 100)

	mux := http.NewServeMux()
	// Хендлер библиотеки сам сверяет X-Max-Bot-Api-Secret,
	// разбирает тело и отвечает 200, не дожидаясь обработки.
	mux.HandleFunc("/", b.api.GetUpdateHandler(updates, b.cfg.WebhookSecret))

	server := &http.Server{
		Addr:              b.cfg.WebhookAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("вебхук слушает %s", b.cfg.WebhookAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("вебхук: %v", err)
		}
		close(updates)
	}()

	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()

	return updates, nil
}

// subscribe перерегистрирует адрес вебхука при каждом старте:
// после передеплоя на другой домен бот иначе молча перестаёт получать события.
func (b *Bot) subscribe(ctx context.Context) error {
	current, err := b.api.Subscriptions.GetSubscriptions(ctx)
	if err != nil {
		return err
	}

	for _, sub := range current.Subscriptions {
		if sub.Url != b.cfg.WebhookURL {
			log.Printf("отцепляю старую подписку %s", sub.Url)
			if _, err := b.api.Subscriptions.Unsubscribe(ctx, sub.Url); err != nil {
				return err
			}
		}
	}

	_, err = b.api.Subscriptions.Subscribe(ctx, b.cfg.WebhookURL, nil, b.cfg.WebhookSecret)
	return err
}

// SetCommands показывает подсказки по слешу в клиенте MAX.
func (b *Bot) SetCommands(ctx context.Context) error {
	_, err := b.api.Bots.PatchBot(ctx, &schemes.BotPatch{
		Commands: []schemes.BotCommand{
			{Name: "start", Description: "Начать работу"},
			{Name: "init_sobr", Description: "Создать собрание"},
			{Name: "status", Description: "Статус собрания"},
		},
	})
	return err
}
