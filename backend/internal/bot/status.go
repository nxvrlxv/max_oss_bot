package bot

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"oss-max/internal/domain"
	"oss-max/internal/storage"
)

// В статусе больше пяти собраний не читается, а сообщение MAX ограничено 4000 символов.
const statusLimit = 5

// msk — даты в сообщениях показываем по Москве: так их пишут в уведомлениях о собрании.
var msk = time.FixedZone("MSK", 3*60*60)

// sendStatus — в личке показывает собрания инициатора,
// в домовом чате — собрания, привязанные к этому чату.
func (b *Bot) sendStatus(ctx context.Context, upd *schemes.MessageCreatedUpdate) error {
	chatID := upd.GetChatID()

	var (
		meetings []Meeting
		err      error
	)
	if upd.Message.Recipient.ChatType == schemes.DIALOG {
		meetings, err = b.store.MeetingsByInitiator(ctx, upd.Message.Sender.UserId)
	} else {
		meetings, err = b.store.MeetingsByChat(ctx, chatID)
	}
	if err != nil {
		return fmt.Errorf("собрания для статуса: %w", err)
	}

	if len(meetings) == 0 {
		return b.send(ctx, chatID, "Собраний пока нет.", nil)
	}
	if len(meetings) > statusLimit {
		meetings = meetings[:statusLimit]
	}

	parts := make([]string, 0, len(meetings))
	for _, meeting := range meetings {
		result, err := b.store.Result(ctx, meeting.ID)
		if err != nil {
			return fmt.Errorf("итог собрания %d: %w", meeting.ID, err)
		}
		parts = append(parts, statusText(meeting, result))
	}

	return b.send(ctx, chatID, strings.Join(parts, "\n\n"), nil)
}

// statusText — сводка по одному собранию: явка, кворум, итог.
func statusText(meeting Meeting, res domain.Result) string {
	var lines []string

	lines = append(lines, "«"+strings.TrimSpace(meeting.Question)+"»")
	lines = append(lines, statusLabel(meeting))

	if meeting.Status == storage.MeetingDraft {
		return strings.Join(lines, "\n")
	}

	turnout := 0.0
	if res.TotalArea > 0 {
		turnout = res.Tally.Total / res.TotalArea * 100
	}
	lines = append(lines, fmt.Sprintf("Проголосовали %s из %s м² (%s%%)",
		area(res.Tally.Total), area(res.TotalArea), strings.Replace(strconv.FormatFloat(turnout, 'f', 1, 64), ".", ",", 1)))
	lines = append(lines, fmt.Sprintf("За %s · Против %s · Воздержались %s м²",
		area(res.Tally.For), area(res.Tally.Against), area(res.Tally.Abstain)))

	switch {
	case !res.Quorum:
		lines = append(lines, "Кворума нет: "+shortfall(res.Gap))
	case res.Accepted:
		lines = append(lines, "Кворум есть. Решение принято")
	default:
		lines = append(lines, "Кворум есть. Решение не принято: «за» "+shortfall(res.Gap))
	}

	return strings.Join(lines, "\n")
}

func statusLabel(meeting Meeting) string {
	switch meeting.Status {
	case storage.MeetingDraft:
		return "Черновик: вопрос ещё не опубликован"
	case storage.MeetingFinished:
		return "Голосование завершено"
	}
	if meeting.EndsAt != nil {
		return "Идёт голосование до " + meeting.EndsAt.In(msk).Format("02.01.2006 15:04")
	}
	return "Идёт голосование"
}

// shortfall — сколько не хватает. При строгом «более чем» разрыв бывает
// нулевым, а порог не взят: ровно половины мало.
func shortfall(gap float64) string {
	if gap <= 0 {
		return "не хватает ещё хотя бы одного голоса"
	}
	return "не хватает " + area(gap) + " м²"
}

// area печатает метры по-русски: «1 234,5».
func area(value float64) string {
	value = math.Round(value*100) / 100
	text := strconv.FormatFloat(value, 'f', -1, 64)

	integer, fraction, hasFraction := strings.Cut(text, ".")

	var grouped strings.Builder
	for i, digit := range integer {
		if i > 0 && (len(integer)-i)%3 == 0 {
			grouped.WriteRune(' ')
		}
		grouped.WriteRune(digit)
	}

	if hasFraction {
		return grouped.String() + "," + fraction
	}
	return grouped.String()
}
