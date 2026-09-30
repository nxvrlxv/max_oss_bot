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
	"oss-max/internal/storage"
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

// SendBulletins присылает в личку то, что выбрала Bulletins. Только в личку:
// в бюллетене ФИО и реквизиты права, в общий чат им нельзя.
func (n *Notifier) SendBulletins(ctx context.Context, maxID int64, meeting Meeting, action Action, bulletins []storage.Bulletin) error {
	if action == ActionBlanks {
		return n.sendBlanks(ctx, maxID, meeting, bulletins)
	}
	return n.sendBulletin(ctx, maxID, meeting, bulletins)
}

// sendBulletin — собственнику его бюллетень с уже отмеченным ответом.
func (n *Notifier) sendBulletin(ctx context.Context, maxID int64, meeting Meeting, bulletins []storage.Bulletin) error {
	text := fmt.Sprintf("Ваш бюллетень по вопросу «%s».\n\n", strings.TrimSpace(meeting.Question))
	if len(bulletins) > 0 && bulletins[0].Choice != "" {
		text += "Ответ уже отмечен — как вы проголосовали в приложении. "
	}
	text += "Распечатайте, проверьте, подпишите и передайте инициатору собрания: " +
		"голос засчитывается по подписанному бланку."
	return n.sendPDF(ctx, maxID, meeting, bulletins, fmt.Sprintf("bulletin-%d.pdf", meeting.ID), text)
}

// sendBlanks — инициатору бюллетени тех, чей голос ещё не учтён:
// с ними он обходит квартиры. Крупные доли — первыми.
func (n *Notifier) sendBlanks(ctx context.Context, maxID int64, meeting Meeting, bulletins []storage.Bulletin) error {
	text := fmt.Sprintf("Бланки для обхода — собственники, чей голос ещё не учтён: %d.\n\n"+
		"Сверху — самые крупные доли: так разрыв до кворума закрывается быстрее.", len(bulletins))
	return n.sendPDF(ctx, maxID, meeting, bulletins, fmt.Sprintf("blanks-%d.pdf", meeting.ID), text)
}

func (n *Notifier) sendPDF(ctx context.Context, maxID int64, meeting Meeting, bulletins []storage.Bulletin, name, text string) error {
	var pdf bytes.Buffer
	if err := docs.Bulletins(&pdf, meeting, bulletins); err != nil {
		return err
	}
	file, err := n.api.Uploads.UploadMediaFromReaderWithName(ctx, schemes.FILE, &pdf, name)
	if err != nil {
		return fmt.Errorf("загрузка %s: %w", name, err)
	}
	return n.api.Messages.Send(ctx, maxbot.NewMessage().SetUser(maxID).SetText(text).AddFile(file))
}
