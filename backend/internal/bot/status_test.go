package bot

import (
	"strings"
	"testing"

	"oss-max/internal/domain"
	"oss-max/internal/storage"
)

func TestArea(t *testing.T) {
	tests := map[float64]string{
		0:          "0",
		45.5:       "45,5",
		1234.567:   "1\u00a0234,57",
		6000:       "6\u00a0000",
		1234567.01: "1\u00a0234\u00a0567,01",
	}
	for value, want := range tests {
		if got := area(value); got != want {
			t.Errorf("area(%v) = %q, хотели %q", value, got, want)
		}
	}
}

func TestStatusText(t *testing.T) {
	meeting := Meeting{Question: "Установить шлагбаум", Status: storage.MeetingActive}

	votes := []domain.Vote{
		{OwnerID: 1, Choice: domain.ChoiceFor, Area: 2500, Status: domain.StatusConfirmed},
		{OwnerID: 2, Choice: domain.ChoiceAgainst, Area: 200, Status: domain.StatusConfirmed},
	}
	text := statusText(meeting, domain.Evaluate(6000, votes, domain.RuleSoft()))

	for _, want := range []string{"«Установить шлагбаум»", "2\u00a0700 из 6\u00a0000 м² (45,0%)", "Кворума нет: не хватает 300 м²"} {
		if !strings.Contains(text, want) {
			t.Errorf("в статусе нет %q:\n%s", want, text)
		}
	}

	// Ровно половина — кворума нет, хотя разрыв нулевой.
	votes = []domain.Vote{{OwnerID: 1, Choice: domain.ChoiceFor, Area: 3000, Status: domain.StatusConfirmed}}
	text = statusText(meeting, domain.Evaluate(6000, votes, domain.RuleSoft()))
	if !strings.Contains(text, "хотя бы одного голоса") {
		t.Errorf("ровно 50%% должно требовать ещё голос:\n%s", text)
	}

	draft := statusText(Meeting{Question: "Вопрос", Status: storage.MeetingDraft}, domain.Result{})
	if strings.Contains(draft, "Проголосовали") {
		t.Errorf("у черновика не должно быть явки:\n%s", draft)
	}
}
