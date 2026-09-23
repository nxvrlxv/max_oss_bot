package bot

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"oss-max/internal/storage"
)

// Handle разбирает событие по типу и зовёт нужный обработчик.
func (b *Bot) Handle(ctx context.Context, update schemes.UpdateInterface) error {
	switch upd := update.(type) {
	case *schemes.BotStartedUpdate:
		return b.onBotStarted(ctx, upd)
	case *schemes.MessageCreatedUpdate:
		return b.onMessage(ctx, upd)
	case *schemes.BotAddedToChatUpdate:
		return b.onBotAdded(ctx, upd)
	case *schemes.MessageCallbackUpdate:
		return b.onCallback(ctx, upd)
	case *schemes.BotRemovedFromChatUpdate:
		return b.onBotRemoved(ctx, upd)
	case *schemes.BotStopedFromChatUpdate:
		return b.onBotStopped(ctx, upd)
	default:
		return nil // неизвестные типы игнорируем молча: API в бете
	}
}

// onBotStarted — человек нажал «Начать». Запоминаем его и открытый диалог,
// затем смотрим, с чем он пришёл: по ссылке на голосование или просто так.
func (b *Bot) onBotStarted(ctx context.Context, upd *schemes.BotStartedUpdate) error {
	user := User{
		MaxID:    upd.User.UserId,
		Name:     upd.User.Name,
		Username: upd.User.Username,
	}

	if err := b.store.SaveUser(ctx, user); err != nil {
		return fmt.Errorf("сохранение пользователя: %w", err)
	}
	// Без этой записи бот не сможет написать человеку первым,
	// а значит не отправит персональный бланк.
	if err := b.store.SaveDialog(ctx, user.MaxID); err != nil {
		return fmt.Errorf("сохранение диалога: %w", err)
	}

	payload, ok := Parse(upd.Payload)
	if !ok || payload.Action != ActionOpen {
		return b.sendMainMenu(ctx, upd.ChatId)
	}

	meeting, err := b.store.Meeting(ctx, payload.ID)
	if errors.Is(err, storage.ErrNotFound) {
		return b.sendMainMenu(ctx, upd.ChatId)
	}
	if err != nil {
		return fmt.Errorf("собрание %d: %w", payload.ID, err)
	}

	kb := b.api.Messages.NewKeyboardBuilder()
	kb.AddRow().AddOpenApp("Проголосовать", b.app, Format(ActionOpen, meeting.ID), 0)

	text := fmt.Sprintf("Голосование по вопросу:\n\n%s", meeting.Question)

	return b.send(ctx, upd.ChatId, text, kb)
}

// sendMainMenu — меню инициатора: с него начинается сценарий создания собрания.
func (b *Bot) sendMainMenu(ctx context.Context, chatID int64) error {
	kb := b.api.Messages.NewKeyboardBuilder()
	kb.AddRow().AddOpenApp("Создать собрание", b.app, Format(ActionNew, 0), 0)
	kb.AddRow().AddCallback("Мои собрания", schemes.DEFAULT, Format(ActionList, 0))

	text := "Помогу подготовить общее собрание собственников:\n" +
		"соберу позиции с учётом площадей, покажу, сколько метров не хватает до кворума,\n" +
		"и подготовлю бланки с протоколом."

	return b.send(ctx, chatID, text, kb)
}

// send — единственное место, где собирается сообщение.
func (b *Bot) send(ctx context.Context, chatID int64, text string, kb *maxbot.Keyboard) error {
	msg := maxbot.NewMessage().SetChat(chatID).SetText(text)
	if kb != nil {
		msg = msg.AddKeyboard(kb)
	}
	return b.api.Messages.Send(ctx, msg)
}

func (b *Bot) onMessage(ctx context.Context, upd *schemes.MessageCreatedUpdate) error {
	content := strings.TrimSpace(upd.Message.Body.Text)

	if !strings.HasPrefix(content, "/") {
		return nil
	}

	// В групповом чате команда может прийти как «/status@имя_бота» или с аргументами.
	command, _, _ := strings.Cut(content, " ")
	command, _, _ = strings.Cut(command, "@")

	// По логу инициатор узнаёт свой max_id — он нужен для seed.
	log.Printf("%s от пользователя %d в чате %d", command, upd.Message.Sender.UserId, upd.GetChatID())

	switch command {
	case "/start":
		return b.sendMainMenu(ctx, upd.GetChatID())
	case "/init_sobr":
		kb := b.api.Messages.NewKeyboardBuilder()
		kb.AddRow().AddOpenApp("Запустить приложение", b.app, Format(ActionNew, 0), 0)
		return b.send(ctx, upd.GetChatID(), "Создадим собрание", kb)
	case "/status":
		return b.sendStatus(ctx, upd)
	default:
		return nil
	}
}

// onBotAdded — бота добавили в домовой чат. Предлагаем тому, кто добавил,
// привязать чат к одному из его собраний: без привязки боту некуда публиковать.
func (b *Bot) onBotAdded(ctx context.Context, upd *schemes.BotAddedToChatUpdate) error {
	meetings, err := b.store.MeetingsByInitiator(ctx, upd.User.UserId)
	if err != nil {
		return fmt.Errorf("собрания пользователя %d: %w", upd.User.UserId, err)
	}

	if len(meetings) == 0 {
		kb := b.api.Messages.NewKeyboardBuilder()
		kb.AddRow().AddOpenApp("Создать собрание", b.app, Format(ActionNew, 0), 0)

		return b.send(ctx, upd.ChatId,
			"Готов помочь с собранием собственников. Сначала создайте собрание, "+
				"потом вернитесь сюда и привяжите к нему этот чат.", kb)
	}

	kb := b.api.Messages.NewKeyboardBuilder()
	for _, meeting := range meetings {
		kb.AddRow().AddCallback(label(meeting), schemes.DEFAULT, Format(ActionBind, meeting.ID))
	}

	return b.send(ctx, upd.ChatId, "К какому собранию привязать этот чат?", kb)
}

// label — подпись кнопки: вопрос собрания, обрезанный до читаемой длины.
func label(meeting Meeting) string {
	const limit = 40

	text := strings.TrimSpace(meeting.Question)
	if text == "" {
		return fmt.Sprintf("Собрание №%d", meeting.ID)
	}

	runes := []rune(text)
	if len(runes) > limit {
		return string(runes[:limit]) + "…"
	}
	return text
}

func (b *Bot) onCallback(ctx context.Context, upd *schemes.MessageCallbackUpdate) error {
	b.api.Messages.AnswerOnCallback(ctx, upd.Callback.CallbackID,
		&schemes.CallbackAnswer{Notification: "Готово"})

	payload, found := Parse(upd.Callback.Payload)

	if !found {
		return nil
	}

	switch payload.Action {
	case ActionBind:
		if upd.Message == nil {
			return nil
		}
		chatID := upd.Message.Recipient.ChatId

		err := b.store.BindChat(ctx, payload.ID, chatID, upd.Callback.User.UserId)
		if errors.Is(err, storage.ErrNotFound) {
			return b.send(ctx, chatID, "Привязать чат может только инициатор собрания.", nil)
		}
		if err != nil {
			return fmt.Errorf("привязка чата %d к собранию %d: %w", chatID, payload.ID, err)
		}

		meeting, err := b.store.Meeting(ctx, payload.ID)
		if err != nil {
			return fmt.Errorf("собрание %d: %w", payload.ID, err)
		}

		// Отправляем без клавиатуры: привязка уже сделана,
		// повторное нажатие той же кнопки ничего не даст.
		return b.send(ctx, chatID,
			fmt.Sprintf("Чат привязан к собранию: %s\n\nКогда опубликуете вопрос, "+
				"уведомление придёт сюда.", label(meeting)), nil)

	case ActionList:
		meetings, err := b.store.MeetingsByInitiator(ctx, upd.Callback.User.UserId)
		if err != nil {
			return fmt.Errorf("собрания пользователя %d: %w", upd.Callback.User.UserId, err)
		}

		chatID := upd.Callback.User.UserId // список показываем в личке
		if upd.Message != nil {
			chatID = upd.Message.Recipient.ChatId
		}

		if len(meetings) == 0 {
			kb := b.api.Messages.NewKeyboardBuilder()
			kb.AddRow().AddOpenApp("Создать собрание", b.app, Format(ActionNew, 0), 0)

			return b.send(ctx, chatID, "Пока собраний нет.", kb)
		}

		kb := b.api.Messages.NewKeyboardBuilder()
		for _, meeting := range meetings {
			kb.AddRow().AddOpenApp(label(meeting), b.app, Format(ActionOpen, meeting.ID), 0)
		}

		return b.send(ctx, chatID, "Ваши собрания:", kb)

	default:
		return nil
	}
}

// onBotRemoved — бота выкинули из чата: снимаем привязку,
// иначе публикация уйдёт туда, где бота уже нет.
func (b *Bot) onBotRemoved(ctx context.Context, upd *schemes.BotRemovedFromChatUpdate) error {
	return b.store.UnbindChat(ctx, upd.ChatId)
}

// onBotStopped — человек остановил бота в личке: больше ему не пишем.
func (b *Bot) onBotStopped(ctx context.Context, upd *schemes.BotStopedFromChatUpdate) error {
	return b.store.CloseDialog(ctx, upd.User.UserId)
}
