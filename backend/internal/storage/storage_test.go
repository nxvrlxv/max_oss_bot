package storage

import (
	"context"
	"errors"
	"os"
	"testing"

	"oss-max/internal/domain"
	"oss-max/internal/registry"
)

// Тесты идут в живую базу и пересоздают в ней схему public.
// Запуск: TEST_DATABASE_URL=postgres://... go test ./internal/storage
func openTest(t *testing.T) *Store {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL не задан")
	}

	ctx := context.Background()
	db, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)

	if _, err := db.pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public`); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	return db
}

func demoFlats(t *testing.T) ([]registry.Flat, registry.Report) {
	t.Helper()

	file, err := os.Open("../../testdata/demo_house.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	rows, err := registry.ParseCSV(file)
	if err != nil {
		t.Fatal(err)
	}
	flats, report, err := registry.Group(rows)
	if err != nil {
		t.Fatal(err)
	}
	return flats, report
}

func newMeeting(t *testing.T, db *Store, initiator int64) Meeting {
	t.Helper()

	meeting, err := db.CreateMeeting(context.Background(), NewMeeting{
		InitiatorMaxID: initiator,
		Address:        "г. Примерск, ул. Тестовая, д. 1",
		Question:       "Установить шлагбаум",
		Rule:           domain.RuleSoft(),
		TotalArea:      1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	return meeting
}

func TestMigrateTwice(t *testing.T) {
	db := openTest(t)
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatalf("повторный накат: %v", err)
	}
}

func TestUsersAndDialogs(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	if err := db.SaveUser(ctx, User{MaxID: 1, Name: "Анна", Username: "anna"}); err != nil {
		t.Fatal(err)
	}
	// Событие без имени не должно затирать известное.
	if err := db.SaveUser(ctx, User{MaxID: 1}); err != nil {
		t.Fatal(err)
	}
	var name string
	if err := db.pool.QueryRow(ctx, `SELECT full_name FROM users WHERE max_id = 1`).Scan(&name); err != nil || name != "Анна" {
		t.Errorf("имя %q, %v — хотели «Анна»", name, err)
	}

	if active, _ := db.DialogActive(ctx, 1); active {
		t.Error("диалога ещё не было")
	}
	for range 2 { // повторная доставка события
		if err := db.SaveDialog(ctx, 1); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.CloseDialog(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if active, _ := db.DialogActive(ctx, 1); active {
		t.Error("диалог закрыт, а считается активным")
	}
}

func TestBindChat(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	first := newMeeting(t, db, 10)
	second := newMeeting(t, db, 10)
	const chat = 555

	if err := db.BindChat(ctx, first.ID, chat, 99); !errors.Is(err, ErrNotFound) {
		t.Fatalf("чужой пользователь привязал чат: %v", err)
	}
	for range 2 {
		if err := db.BindChat(ctx, first.ID, chat, 10); err != nil {
			t.Fatal(err)
		}
	}

	// Второе собрание в том же чате — тот же дом.
	if err := db.BindChat(ctx, second.ID, chat, 10); err != nil {
		t.Fatal(err)
	}
	a, _ := db.Meeting(ctx, first.ID)
	b, _ := db.Meeting(ctx, second.ID)
	if a.HouseID != b.HouseID || b.ChatID != chat {
		t.Errorf("собрания в разных домах: %+v / %+v", a, b)
	}
	var houses int
	_ = db.pool.QueryRow(ctx, `SELECT count(*) FROM houses`).Scan(&houses)
	if houses != 1 {
		t.Errorf("домов %d, опустевший дом должен удалиться", houses)
	}

	found, err := db.MeetingsByChat(ctx, chat)
	if err != nil || len(found) != 2 {
		t.Errorf("собраний в чате %d, %v — хотели 2", len(found), err)
	}

	if err := db.UnbindChat(ctx, chat); err != nil {
		t.Fatal(err)
	}
	if m, _ := db.Meeting(ctx, first.ID); m.ChatID != 0 {
		t.Errorf("чат не отвязан: %+v", m)
	}

	if _, err := db.Meeting(ctx, 12345); !errors.Is(err, ErrNotFound) {
		t.Errorf("несуществующее собрание: %v", err)
	}

	byToken, err := db.MeetingByToken(ctx, first.InviteToken)
	if err != nil || byToken.ID != first.ID || len(first.InviteToken) != 32 {
		t.Errorf("поиск по токену %q: %+v, %v", first.InviteToken, byToken, err)
	}
	if _, err := db.MeetingByToken(ctx, "0123456789abcdef0123456789abcdef"); !errors.Is(err, ErrNotFound) {
		t.Errorf("чужой токен: %v", err)
	}
}

func TestVotingFlow(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	flats, report := demoFlats(t)
	meeting := newMeeting(t, db, 10)

	// Повторная загрузка заменяет реестр, а не дописывает.
	for range 2 {
		if err := db.ImportRegistry(ctx, meeting.ID, flats); err != nil {
			t.Fatal(err)
		}
	}
	units, owners, err := db.Registry(ctx, meeting.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != report.Flats || len(owners) != report.Owners {
		t.Fatalf("в базе %d помещений и %d собственников, в реестре %d и %d",
			len(units), len(owners), report.Flats, report.Owners)
	}

	flatNumber := func(unitID int) string {
		for _, unit := range units {
			if unit.UnitID == unitID {
				return unit.Number
			}
		}
		t.Fatalf("нет помещения %d", unitID)
		return ""
	}

	owner := owners[0]
	voter := User{MaxID: 20, Name: "Иван Петров"}

	if _, _, err := db.ClaimFlat(ctx, meeting.ID, flatNumber(owner.UnitID), voter); !errors.Is(err, ErrCannotVote) {
		t.Fatalf("заявка до публикации: %v", err)
	}
	if err := db.Publish(ctx, meeting.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.Publish(ctx, meeting.ID); !errors.Is(err, ErrNotDraft) {
		t.Errorf("повторная публикация: %v", err)
	}
	if err := db.ImportRegistry(ctx, meeting.ID, flats); !errors.Is(err, ErrNotDraft) {
		t.Errorf("реестр заменён после публикации: %v", err)
	}

	if _, _, err := db.ClaimFlat(ctx, meeting.ID, "нет такой", voter); !errors.Is(err, ErrNotFound) {
		t.Errorf("заявка на несуществующую квартиру: %v", err)
	}
	if err := db.Vote(ctx, meeting.ID, voter.MaxID, domain.ChoiceFor); !errors.Is(err, ErrCannotVote) {
		t.Errorf("голос без заявки: %v", err)
	}

	if has, _ := db.HasClaim(ctx, meeting.ID, voter.MaxID); has {
		t.Error("заявки ещё нет, а HasClaim говорит, что есть")
	}
	claim, fresh, err := db.ClaimFlat(ctx, meeting.ID, flatNumber(owner.UnitID), voter)
	if err != nil || !fresh {
		t.Fatal(fresh, err)
	}
	if has, _ := db.HasClaim(ctx, meeting.ID, voter.MaxID); !has {
		t.Error("заявка подана, а HasClaim её не видит")
	}
	again, fresh, err := db.ClaimFlat(ctx, meeting.ID, flatNumber(owner.UnitID), voter)
	if err != nil || again.ID != claim.ID || fresh {
		t.Errorf("повторная заявка создала новую: %+v, %v", again, err)
	}

	// Голос до подтверждения сохраняется, но не считается.
	if err := db.Vote(ctx, meeting.ID, voter.MaxID, domain.ChoiceFor); err != nil {
		t.Fatal(err)
	}
	result, _ := db.Result(ctx, meeting.ID)
	if result.Tally.Total != 0 {
		t.Errorf("неподтверждённый голос попал в подсчёт: %+v", result.Tally)
	}

	pending, err := db.PendingClaims(ctx, meeting.ID)
	if err != nil || len(pending) != 1 || pending[0].UserName != "Иван Петров" || len(pending[0].Owners) == 0 {
		t.Fatalf("очередь заявок: %+v, %v", pending, err)
	}

	if err := db.ConfirmClaim(ctx, meeting.ID, claim.ID, owners[len(owners)-1].OwnerID); !errors.Is(err, ErrNotFound) {
		t.Errorf("подтверждение собственником из другой квартиры: %v", err)
	}
	if err := db.ConfirmClaim(ctx, meeting.ID, claim.ID, owner.OwnerID); err != nil {
		t.Fatal(err)
	}

	// Голос из заявки переехал в подсчёт с весом собственника.
	votes, _ := db.Votes(ctx, meeting.ID)
	if len(votes) != 1 || votes[0].Choice != domain.ChoiceFor || votes[0].Status != domain.StatusConfirmed {
		t.Fatalf("голоса после подтверждения: %+v", votes)
	}
	weight := votes[0].Area

	// Переголосование меняет вариант, вес остаётся.
	if err := db.Vote(ctx, meeting.ID, voter.MaxID, domain.ChoiceAgainst); err != nil {
		t.Fatal(err)
	}
	result, _ = db.Result(ctx, meeting.ID)
	if result.Tally.Against != weight || result.Tally.For != 0 {
		t.Errorf("переголосование: %+v, вес %v", result.Tally, weight)
	}

	mine, _ := db.UserClaims(ctx, meeting.ID, voter.MaxID)
	if len(mine) != 1 || mine[0].Status != ClaimConfirmed || mine[0].Weight != weight || mine[0].Choice != domain.ChoiceAgainst {
		t.Errorf("заявки пользователя: %+v", mine)
	}

	// Второй человек на того же собственника: подтвердить нельзя.
	stranger := User{MaxID: 21}
	other, _, err := db.ClaimFlat(ctx, meeting.ID, flatNumber(owner.UnitID), stranger)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.ConfirmClaim(ctx, meeting.ID, other.ID, owner.OwnerID); !errors.Is(err, ErrOwnerTaken) {
		t.Errorf("собственник подтверждён дважды: %v", err)
	}
	if err := db.Vote(ctx, meeting.ID, stranger.MaxID, domain.ChoiceFor); err != nil {
		t.Fatal(err)
	}
	if err := db.RejectClaim(ctx, meeting.ID, other.ID); err != nil {
		t.Fatal(err)
	}
	if result, _ := db.Result(ctx, meeting.ID); result.Tally.For != 0 {
		t.Errorf("голос отклонённой заявки учтён: %+v", result.Tally)
	}
	if err := db.Vote(ctx, meeting.ID, stranger.MaxID, domain.ChoiceFor); !errors.Is(err, ErrCannotVote) {
		t.Errorf("голос после отклонения: %v", err)
	}

	// «Это не моя квартира» — отозвать можно только свою неразобранную заявку.
	third, _, _ := db.ClaimFlat(ctx, meeting.ID, flatNumber(owners[len(owners)-1].UnitID), stranger)
	if err := db.CancelClaim(ctx, meeting.ID, third.ID, voter.MaxID); !errors.Is(err, ErrNotFound) {
		t.Errorf("отозвал чужую заявку: %v", err)
	}
	if err := db.CancelClaim(ctx, meeting.ID, third.ID, stranger.MaxID); err != nil {
		t.Errorf("не отозвал свою: %v", err)
	}

	coverage, err := db.Coverage(ctx, meeting.ID)
	if err != nil || coverage.FlatsTotal != report.Flats || coverage.FlatsVoted != 1 {
		t.Errorf("охват: %+v, %v", coverage, err)
	}
	for i := 1; i < len(coverage.NotVoted); i++ {
		if coverage.NotVoted[i].Area > coverage.NotVoted[i-1].Area {
			t.Fatalf("неголосовавшие не отсортированы по площади: %+v", coverage.NotVoted[i-1:i+1])
		}
	}

	found, _ := db.MeetingsForUser(ctx, voter.MaxID)
	if len(found) != 1 || found[0].ID != meeting.ID {
		t.Errorf("собрания собственника: %+v", found)
	}

	if err := db.Finish(ctx, meeting.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.Vote(ctx, meeting.ID, voter.MaxID, domain.ChoiceFor); !errors.Is(err, ErrCannotVote) {
		t.Errorf("голос после завершения: %v", err)
	}
}
