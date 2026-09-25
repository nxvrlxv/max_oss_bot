package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"oss-max/internal/storage"
)

type silentNotifier struct{}

func (silentNotifier) AnnounceMeeting(context.Context, storage.Meeting) error { return nil }
func (silentNotifier) NotifyClaim(context.Context, storage.Meeting, string, string) error {
	return nil
}

// testServer поднимает API на отдельной схеме базы: тесты storage в соседнем
// пакете пересоздают public, и при параллельном go test ./... они бы мешали друг другу.
func testServer(t *testing.T) *httptest.Server {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL не задан")
	}
	ctx := context.Background()

	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(ctx, `DROP SCHEMA IF EXISTS api_test CASCADE; CREATE SCHEMA api_test`)
	conn.Close(ctx)
	if err != nil {
		t.Fatal(err)
	}

	separator := "?"
	if strings.Contains(url, "?") {
		separator = "&"
	}
	db, err := storage.Open(ctx, url+separator+"search_path=api_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(New(db, silentNotifier{}, Config{BotToken: testToken, BotName: "oss_bot"}).Handler())
	t.Cleanup(server.Close)
	return server
}

// client — запросы от имени пользователя MAX с настоящей подписью initData.
type client struct {
	t      *testing.T
	base   string
	userID int64
	invite string
}

func (c client) do(method, path string, body any) (int, map[string]any) {
	c.t.Helper()

	var reader io.Reader
	contentType := "application/json"
	switch b := body.(type) {
	case nil:
	case string:
		reader, contentType = strings.NewReader(b), "text/csv"
	default:
		raw, _ := json.Marshal(b)
		reader = strings.NewReader(string(raw))
	}

	req, _ := http.NewRequest(method, c.base+path, reader)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Max-Init-Data", signed(map[string]string{
		"auth_date": strconv.FormatInt(time.Now().Unix(), 10),
		"user":      fmt.Sprintf(`{"id":%d,"first_name":"Тест"}`, c.userID),
	}, testToken))
	if c.invite != "" {
		req.Header.Set("X-Invite-Token", c.invite)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()

	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

const testRegistry = "№ помещения;Адрес объекта;Правообладатель (правообладатели);Долевая площадь, м²;Общая площадь, м²\n" +
	"1;ул. Садовая, д. 12, кв. 1;Иванов Иван;60;60\n" +
	"2;ул. Садовая, д. 12, кв. 2;Петрова Анна;40;40\n"

func TestInviteAccess(t *testing.T) {
	server := testServer(t)
	initiator := client{t: t, base: server.URL, userID: 10}
	stranger := client{t: t, base: server.URL, userID: 20}

	status, created := initiator.do("POST", "/api/meetings", map[string]any{
		"address": "ул. Садовая, д. 12", "question": "Шлагбаум", "rule": "soft",
		"total_area": 100, "entrances_count": 1,
		"ends_at": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
	})
	if status != http.StatusCreated {
		t.Fatalf("создание: %d %v", status, created)
	}
	id := int(created["id"].(float64))
	meetingPath := fmt.Sprintf("/api/meetings/%d", id)

	if status, body := initiator.do("POST", meetingPath+"/registry", testRegistry); status != http.StatusOK {
		t.Fatalf("реестр: %d %v", status, body)
	}

	// До публикации ссылки нет, и по токену черновик не открывается.
	_, draft := initiator.do("GET", meetingPath, nil)
	if draft["invite_link"] != nil {
		t.Errorf("у черновика есть ссылка: %v", draft["invite_link"])
	}

	status, published := initiator.do("POST", meetingPath+"/publish", nil)
	if status != http.StatusOK {
		t.Fatalf("публикация: %d %v", status, published)
	}
	link, _ := published["invite_link"].(string)
	token, found := strings.CutPrefix(link, "https://max.ru/oss_bot?startapp=join_")
	if !found || len(token) != 32 {
		t.Fatalf("ссылка-приглашение: %q", link)
	}

	// Посторонний по одному номеру собрание не видит.
	for _, path := range []string{meetingPath, meetingPath + "/flats"} {
		if status, _ := stranger.do("GET", path, nil); status != http.StatusNotFound {
			t.Errorf("GET %s без приглашения: %d, хотели 404", path, status)
		}
	}
	if status, _ := stranger.do("POST", meetingPath+"/claims", map[string]string{"flat_number": "1"}); status != http.StatusNotFound {
		t.Errorf("заявка без приглашения: %d, хотели 404", status)
	}
	wrong := stranger
	wrong.invite = strings.Repeat("0", 32)
	if status, _ := wrong.do("GET", meetingPath, nil); status != http.StatusNotFound {
		t.Errorf("чужой токен: %d, хотели 404", status)
	}

	// По приглашению — видит, выбирает квартиру, подаёт заявку.
	status, joined := stranger.do("GET", "/api/join/"+token, nil)
	if status != http.StatusOK || int(joined["id"].(float64)) != id {
		t.Fatalf("join: %d %v", status, joined)
	}
	invited := stranger
	invited.invite = token
	if status, body := invited.do("GET", meetingPath, nil); status != http.StatusOK || body["invite_link"] != link {
		t.Errorf("собрание по приглашению: %d %v", status, body)
	}
	if status, _ := invited.do("GET", meetingPath+"/flats", nil); status != http.StatusOK {
		t.Errorf("квартиры по приглашению: %d", status)
	}
	if status, body := invited.do("POST", meetingPath+"/claims", map[string]string{"flat_number": "1"}); status != http.StatusOK {
		t.Fatalf("заявка по приглашению: %d %v", status, body)
	}

	// С заявкой токен больше не нужен: человек уже участник.
	if status, _ := stranger.do("GET", meetingPath, nil); status != http.StatusOK {
		t.Errorf("участник без токена: %d, хотели 200", status)
	}

	if status, _ := stranger.do("GET", "/api/join/"+strings.Repeat("f", 32), nil); status != http.StatusNotFound {
		t.Errorf("несуществующее приглашение: %d, хотели 404", status)
	}
}

func TestInitiatorVotes(t *testing.T) {
	server := testServer(t)
	initiator := client{t: t, base: server.URL, userID: 10}
	stranger := client{t: t, base: server.URL, userID: 20}

	_, created := initiator.do("POST", "/api/meetings", map[string]any{
		"address": "ул. Садовая, д. 12", "question": "Шлагбаум", "rule": "soft",
		"total_area": 100, "entrances_count": 1,
		"ends_at": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
	})
	meetingPath := fmt.Sprintf("/api/meetings/%d", int(created["id"].(float64)))
	initiator.do("POST", meetingPath+"/registry", testRegistry)
	_, published := initiator.do("POST", meetingPath+"/publish", nil)
	token := strings.TrimPrefix(published["invite_link"].(string), "https://max.ru/oss_bot?startapp=join_")

	// Список собственников квартиры — только инициатору.
	if status, _ := stranger.do("GET", meetingPath+"/flats/1/owners", nil); status != http.StatusNotFound {
		t.Errorf("собственники постороннему: %d, хотели 404", status)
	}
	req, _ := http.NewRequest("GET", server.URL+meetingPath+"/flats/1/owners", nil)
	req.Header.Set("X-Max-Init-Data", signed(map[string]string{
		"auth_date": strconv.FormatInt(time.Now().Unix(), 10), "user": `{"id":10}`,
	}, testToken))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var owners []storage.RegistryOwner
	_ = json.NewDecoder(resp.Body).Decode(&owners)
	resp.Body.Close()
	if len(owners) != 1 || owners[0].Name != "Иванов Иван" {
		t.Fatalf("собственники кв. 1: %+v", owners)
	}

	// Посторонний не может сам себя подтвердить, даже с приглашением.
	invited := stranger
	invited.invite = token
	if status, _ := invited.do("POST", meetingPath+"/claims", map[string]any{"flat_number": "1", "owner_id": owners[0].ID}); status != http.StatusForbidden {
		t.Errorf("самоподтверждение постороннего: %d, хотели 403", status)
	}

	status, claim := initiator.do("POST", meetingPath+"/claims", map[string]any{"flat_number": "1", "owner_id": owners[0].ID})
	if status != http.StatusOK || claim["status"] != "confirmed" {
		t.Fatalf("заявка инициатора: %d %v", status, claim)
	}
	if status, voted := initiator.do("POST", meetingPath+"/vote", map[string]string{"choice": "for"}); status != http.StatusOK || voted["choice"] != "for" {
		t.Fatalf("голос инициатора: %d %v", status, voted)
	}

	_, board := initiator.do("GET", meetingPath+"/dashboard", nil)
	tally := board["tally"].(map[string]any)
	if tally["for"].(float64) != 60 {
		t.Errorf("голос инициатора не в итоге: %v", tally)
	}
	if board["pending_claims"].(float64) != 0 {
		t.Errorf("своя заявка инициатора в очереди: %v", board["pending_claims"])
	}
}

func TestTotalAreaFlow(t *testing.T) {
	server := testServer(t)
	initiator := client{t: t, base: server.URL, userID: 10}
	deadline := time.Now().Add(24 * time.Hour).Format(time.RFC3339)

	// Без площади: её посчитает реестр (в testRegistry помещения на 100 м²).
	status, created := initiator.do("POST", "/api/meetings", map[string]any{
		"address": "ул. Садовая, д. 12", "question": "Шлагбаум", "rule": "soft", "ends_at": deadline,
	})
	if status != http.StatusCreated || created["total_area"].(float64) != 0 || created["total_area_source"] != "registry" {
		t.Fatalf("создание без площади: %d %v", status, created)
	}
	path := fmt.Sprintf("/api/meetings/%d", int(created["id"].(float64)))
	initiator.do("POST", path+"/registry", testRegistry)
	if _, m := initiator.do("GET", path, nil); m["total_area"].(float64) != 100 {
		t.Errorf("площадь из реестра: %v", m["total_area"])
	}

	if status, body := initiator.do("PUT", path+"/total-area", map[string]any{"total_area": 90}); status != http.StatusBadRequest {
		t.Errorf("ручная площадь меньше реестра: %d %v", status, body)
	}
	if status, m := initiator.do("PUT", path+"/total-area", map[string]any{"total_area": 120}); status != http.StatusOK || m["total_area_source"] != "manual" {
		t.Errorf("ручная площадь: %d %v", status, m)
	}
	if status, _ := initiator.do("POST", path+"/publish", nil); status != http.StatusOK {
		t.Errorf("публикация: %d", status)
	}

	// Площадь, заданная до реестра меньше его суммы, не даёт опубликовать.
	_, low := initiator.do("POST", "/api/meetings", map[string]any{
		"address": "ул. Садовая, д. 12", "question": "Калитка", "rule": "soft", "ends_at": deadline, "total_area": 50,
	})
	lowPath := fmt.Sprintf("/api/meetings/%d", int(low["id"].(float64)))
	initiator.do("POST", lowPath+"/registry", testRegistry)
	if status, body := initiator.do("POST", lowPath+"/publish", nil); status != http.StatusBadRequest {
		t.Errorf("публикация с заниженной площадью: %d %v", status, body)
	}
}
