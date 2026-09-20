package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"
)

func main() {
	api, err := maxbot.New(os.Getenv("TOKEN"), maxbot.WithBaseURL("https://platform-api.max.ru/"))

	if err != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background()) // создам
	// Some methods demo:
	info, err := api.Bots.GetBot(ctx)
	fmt.Printf("Get me: %#v %#v", info, err)
	go func() {
		exit := make(chan os.Signal)
		signal.Notify(exit, os.Kill, os.Interrupt)
		<-exit
		cancel()
	}()

	for upd := range api.GetUpdates(ctx) { // Чтение из канала с обновлениями
		switch upd := upd.(type) { // Определение типа пришедшего обновления
		case *schemes.MessageCreatedUpdate:
			// Отправка сообщения
			err := api.Messages.Send(ctx, maxbot.NewMessage().SetChat(upd.Message.Recipient.ChatId).SetText("Hello from Bot"))
			if err != nil {
				return
			}
		}
	}
}
