package bot

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"oss-max/internal/docs"
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
	// Ссылку текстом можно скопировать и переслать тем, кого нет в чате.
	link := InviteLink(n.app, meeting.InviteToken)
	fmt.Fprintf(&text, "\n\nСсылка для соседей: %s", link)

	message := maxbot.NewMessage().SetChat(meeting.ChatID).SetText(text.String())
	// QR с той же ссылкой — распечатать для подъезда. Без картинки публикация
	// всё равно уходит: кнопка и ссылка важнее.
	if photo, err := n.qrPhoto(ctx, link); err != nil {
		log.Printf("QR-код собрания %d: %v", meeting.ID, err)
	} else {
		message.AddPhoto(photo)
	}

	kb := n.api.Messages.NewKeyboardBuilder()
	kb.AddRow().AddOpenApp("Проголосовать", n.app, FormatJoin(meeting.InviteToken), 0)

	return n.api.Messages.Send(ctx, message.AddKeyboard(kb))
}

// qrPhoto рисует QR-код ссылки и загружает его в MAX как фото.
func (n *Notifier) qrPhoto(ctx context.Context, link string) (*schemes.PhotoTokens, error) {
	png, err := docs.QR(link)
	if err != nil {
		return nil, err
	}
	return n.api.Uploads.UploadPhotoFromReaderWithName(ctx, bytes.NewReader(png), "qr.png")
}

// AnnounceCancelled сообщает в домовой чат, что голосование удалено:
// иначе там останется кнопка «Проголосовать», которая ведёт в никуда.
func (n *Notifier) AnnounceCancelled(ctx context.Context, meeting Meeting) error {
	if meeting.ChatID == 0 {
		return nil
	}
	text := fmt.Sprintf("Голосование «%s» отменено инициатором. Голоса, отданные по нему, не учитываются.",
		strings.TrimSpace(meeting.Question))
	return n.api.Messages.Send(ctx, maxbot.NewMessage().SetChat(meeting.ChatID).SetText(text))
}

// NotifyClaim сообщает инициатору о новой заявке. Бот может написать ему,
// только если инициатор хоть раз запускал бота, — ошибку вызывающий логирует и идёт дальше.
func (n *Notifier) NotifyClaim(ctx context.Context, meeting Meeting, flatNumber, userName string) error {
	text := fmt.Sprintf("Новая заявка: кв. %s — %s.\nСверьте её с реестром собственников.", flatNumber, userName)

	kb := n.api.Messages.NewKeyboardBuilder()
	kb.AddRow().AddOpenApp("Разобрать заявки", n.app, Format(ActionClaims, meeting.ID), 0)

	return n.api.Messages.Send(ctx, maxbot.NewMessage().SetUser(meeting.InitiatorID).SetText(text).AddKeyboard(kb))
}
