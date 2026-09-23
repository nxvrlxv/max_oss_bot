package bot

import (
	"context"
	"fmt"
	"strings"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
)

// Notifier — сообщения, которые отправляет не обработчик апдейта, а API:
// публикация собрания в домовой чат и уведомления инициатору.
// Тексты и кнопки бота живут в одном пакете.
type Notifier struct {
	api *maxbot.Api
	app string
}

func NewNotifier(api *maxbot.Api, app string) *Notifier {
	return &Notifier{api: api, app: app}
}

// AnnounceMeeting публикует вопрос в домовой чат. Без привязанного чата
// ничего не делает: инициатор разошлёт ссылку сам.
func (n *Notifier) AnnounceMeeting(ctx context.Context, meeting Meeting) error {
	if meeting.ChatID == 0 {
		return nil
	}

	var text strings.Builder
	fmt.Fprintf(&text, "Общее собрание собственников · %s\n\n", meeting.Address)
	fmt.Fprintf(&text, "Вопрос: «%s»\n", strings.TrimSpace(meeting.Question))
	if meeting.EndsAt != nil {
		fmt.Fprintf(&text, "Голосование до %s (МСК).\n", meeting.EndsAt.In(msk).Format("02.01.2006 15:04"))
	}
	text.WriteString("\nНажмите «Проголосовать» и выберите свою квартиру. " +
		"Инициатор сверит заявку с реестром собственников — после этого голос будет учтён.")

	kb := n.api.Messages.NewKeyboardBuilder()
	kb.AddRow().AddOpenApp("Проголосовать", n.app, Format(ActionOpen, meeting.ID), 0)

	return n.api.Messages.Send(ctx, maxbot.NewMessage().SetChat(meeting.ChatID).SetText(text.String()).AddKeyboard(kb))
}

// NotifyClaim сообщает инициатору о новой заявке. Бот может написать ему,
// только если инициатор хоть раз запускал бота, — ошибку вызывающий логирует и идёт дальше.
func (n *Notifier) NotifyClaim(ctx context.Context, meeting Meeting, flatNumber, userName string) error {
	text := fmt.Sprintf("Новая заявка: кв. %s — %s.\nСверьте её с реестром собственников.", flatNumber, userName)

	kb := n.api.Messages.NewKeyboardBuilder()
	kb.AddRow().AddOpenApp("Разобрать заявки", n.app, Format(ActionClaims, meeting.ID), 0)

	return n.api.Messages.Send(ctx, maxbot.NewMessage().SetUser(meeting.InitiatorID).SetText(text).AddKeyboard(kb))
}
