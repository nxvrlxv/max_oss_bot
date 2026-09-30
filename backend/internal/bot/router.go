package bot

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

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
	if !ok {
		return b.sendMainMenu(ctx, upd.ChatId)
	}

	var meeting Meeting
	var err error
	switch payload.Action {
	case ActionBlank, ActionBlanks: // кнопка бюллетеня в приложении, пока бот не мог написать первым
		return b.deliverBulletins(ctx, upd.ChatId, user.MaxID, payload)
	case ActionJoin: // ссылка-приглашение в формате ?start=join_<токен>
		meeting, err = b.store.MeetingByToken(ctx, payload.Token)
	case ActionOpen:
		meeting, err = b.store.Meeting(ctx, payload.ID)
	default:
		return b.sendMainMenu(ctx, upd.ChatId)
	}
	if errors.Is(err, storage.ErrNotFound) {
		return b.sendMainMenu(ctx, upd.ChatId)
	}
	if err != nil {
		return fmt.Errorf("собрание по ссылке %q: %w", upd.Payload, err)
	}
	// Черновик соседям не показываем: вопрос ещё может поменяться.
	if meeting.Status == storage.MeetingDraft && meeting.InitiatorID != user.MaxID {
		return b.sendMainMenu(ctx, upd.ChatId)
	}

	// Кнопка несёт токен, а не номер: по номеру постороннему собрание не откроется.
	kb := b.api.Messages.NewKeyboardBuilder()
	kb.AddRow().AddOpenApp("Проголосовать", b.app, FormatJoin(meeting.InviteToken), 0)

	text := fmt.Sprintf("Голосование по вопросу:\n\n%s", meeting.Question)

	return b.send(ctx, upd.ChatId, text, kb)
}

// deliverBulletins присылает бюллетень или бланки для обхода тому, кто нажал
// «Начать» по ссылке из приложения. Если выдать нельзя — объясняет почему.
func (b *Bot) deliverBulletins(ctx context.Context, chatID, maxID int64, payload Payload) error {
	meeting, err := b.store.Meeting(ctx, payload.ID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return b.sendMainMenu(ctx, chatID)
		}
		return fmt.Errorf("собрание %d: %w", payload.ID, err)
	}

	bulletins, err := Bulletins(ctx, b.store, meeting, maxID, payload.Action)
	switch {
	case errors.Is(err, storage.ErrNotFound):
		return b.sendMainMenu(ctx, chatID)
	case errors.Is(err, ErrBulletinsClosed), errors.Is(err, ErrNotConfirmed), errors.Is(err, ErrNobodyLeft):
		return b.send(ctx, chatID, err.Error(), nil)
	case err != nil:
		return fmt.Errorf("бюллетени собрания %d: %w", meeting.ID, err)
	}
	return NewNotifier(b.api, b.app).SendBulletins(ctx, maxID, meeting, payload.Action, bulletins)
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
	// MAX может прислать текст с явным упоминанием перед командой.
	if parts := strings.Fields(content); len(parts) > 1 && b.app != "" && strings.EqualFold(parts[0], "@"+b.app) {
		content = strings.Join(parts[1:], " ")
	}

	if !strings.HasPrefix(content, "/") {
		return nil
	}

	// В групповом чате команда может прийти как «/status@имя_бота» или с аргументами.
	command := strings.Fields(content)[0]
	command, target, targeted := strings.Cut(command, "@")
	if targeted && !strings.EqualFold(target, b.app) {
		return nil
	}

	// По логу инициатор узнаёт свой max_id — он нужен для seed.
	log.Printf("%s от пользователя %d в чате %d", command, upd.Message.Sender.UserId, upd.GetChatID())

	switch command {
	case "/bind":
		if string(upd.Message.Recipient.ChatType) != "chat" {
			return b.send(ctx, upd.GetChatID(), "Отправьте команду привязки в групповой чат дома, куда добавлен бот.", nil)
		}
		parts := strings.Fields(content)
		if len(parts) != 2 {
			return b.send(ctx, upd.GetChatID(), "Скопируйте команду /bind с номером из раздела «Домовой чат» нужного собрания.", nil)
		}
		id, err := strconv.Atoi(parts[1])
		if err != nil || id <= 0 {
			return b.send(ctx, upd.GetChatID(), "Некорректный номер собрания.", nil)
		}
		if err := b.bindMeeting(ctx, id, upd.GetChatID(), upd.Message.Sender.UserId); err != nil {
			log.Printf("привязка собрания %d в чат %d: %v", id, upd.GetChatID(), err)
			return b.send(ctx, upd.GetChatID(), "Не удалось привязать собрание из-за ошибки сервиса. Попробуйте ещё раз. Если ошибка повторяется, передайте администратору номер собрания.", nil)
		}
		return nil
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

// onBotAdded объясняет адресную привязку, не раскрывая в группе список собраний.
func (b *Bot) onBotAdded(ctx context.Context, upd *schemes.BotAddedToChatUpdate) error {
	kb := b.api.Messages.NewKeyboardBuilder()
	kb.AddRow().AddOpenApp("Мои собрания", b.app, Format(ActionList, 0), 0)
	return b.send(ctx, upd.ChatId, "Готов помочь с собранием собственников. Назначьте меня администратором с правом чтения сообщений, чтобы я получал команды из этого чата. Затем откройте нужное собрание, скопируйте команду из раздела «Домовой чат» и отправьте её сюда. Для каждого нового собрания используйте его команду — повторно добавлять бота не нужно.", kb)
}

func (b *Bot) bindMeeting(ctx context.Context, id int, chatID, userID int64) error {
	meeting, err := b.store.Meeting(ctx, id)
	if errors.Is(err, storage.ErrNotFound) || (err == nil && meeting.InitiatorID != userID) {
		return b.send(ctx, chatID, "Собрание не найдено или вы не его инициатор.", nil)
	}
	if err != nil {
		return err
	}
	if meeting.Closed(time.Now()) {
		return b.send(ctx, chatID, "Голосование уже завершено.", nil)
	}
	if meeting.ChatID != 0 && meeting.ChatID != chatID {
		return b.send(ctx, chatID, "Собрание уже привязано к другому чату.", nil)
	}
	if err := b.store.BindChat(ctx, id, chatID, userID); err != nil {
		var invalid storage.ErrInvalid
		if errors.As(err, &invalid) {
			return b.send(ctx, chatID, invalid.Reason, nil)
		}
		return err
	}
	meeting, err = b.store.Meeting(ctx, id)
	if err != nil {
		return err
	}
	if meeting.Inviting(time.Now()) {
		if err := NewNotifier(b.api, b.app).AnnounceMeeting(ctx, meeting); err != nil {
			log.Printf("объявление собрания %d в чат %d: %v", id, chatID, err)
			return b.send(ctx, chatID, "Чат привязан, но объявление отправить не удалось. Повторите команду /bind с номером собрания.", nil)
		}
		return nil
	}
	return b.send(ctx, chatID, fmt.Sprintf("Чат привязан: %s\n%s\n\nПосле публикации бот отправит сюда вопрос, ссылку и QR-код.", meeting.Address, meeting.Question), nil)
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
		if upd.Message == nil || string(upd.Message.Recipient.ChatType) != "chat" {
			return nil
		}
		return b.bindMeeting(ctx, payload.ID, upd.Message.Recipient.ChatId, upd.Callback.User.UserId)

	case ActionList:
		if upd.Message != nil && upd.Message.Recipient.ChatType != schemes.DIALOG {
			kb := b.api.Messages.NewKeyboardBuilder()
			kb.AddRow().AddOpenApp("Мои собрания", b.app, Format(ActionList, 0), 0)
			return b.send(ctx, upd.Message.Recipient.ChatId, "Выберите нужное собрание в приложении.", kb)
		}
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
