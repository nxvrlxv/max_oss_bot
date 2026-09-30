package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// Бюллетени выдаются, пока идёт голосование: свой — после сверки с реестром,
// бланки для обхода — только инициатору. Бот присылает файл сам, если
// диалог с ним открыт; иначе API отдаёт ссылку на «Начать» в чате с ботом.
func TestBulletinDelivery(t *testing.T) {
	server, db := testServerStore(t)
	ctx := context.Background()
	initiator := client{t: t, base: server.URL, userID: 10}
	stranger := client{t: t, base: server.URL, userID: 20}

	status, created := initiator.do("POST", "/api/meetings", map[string]any{
		"address": "ул. Садовая, д. 12", "question": "Шлагбаум", "rule": "soft",
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

	if status, _ := initiator.do("POST", meetingPath+"/blanks", nil); status != http.StatusConflict {
		t.Errorf("бланки черновика: %d, хотели 409", status)
	}
	if status, body := initiator.do("POST", meetingPath+"/publish", nil); status != http.StatusOK {
		t.Fatalf("публикация: %d %v", status, body)
	}

	if status, _ := stranger.do("POST", meetingPath+"/blanks", nil); status != http.StatusNotFound {
		t.Errorf("бланки постороннему: %d, хотели 404", status)
	}
	status, body := initiator.do("POST", meetingPath+"/blanks", nil)
	if link := fmt.Sprintf("https://max.ru/oss_bot?start=blanks_%d", id); status != http.StatusOK ||
		body["sent"] != false || body["link"] != link {
		t.Errorf("бланки без диалога с ботом: %d %v", status, body)
	}

	if status, _ := initiator.do("POST", meetingPath+"/bulletin", nil); status != http.StatusConflict {
		t.Errorf("бюллетень без подтверждённой доли: %d, хотели 409", status)
	}
	owners, err := db.FlatOwners(ctx, id, "1")
	if err != nil || len(owners) == 0 {
		t.Fatalf("собственники квартиры 1: %v %v", owners, err)
	}
	// Своя квартира инициатора подтверждается сразу.
	vote := map[string]any{"choice": "for", "flat_number": "1", "owner_id": owners[0].ID}
	if status, body := initiator.do("POST", meetingPath+"/vote", vote); status != http.StatusOK {
		t.Fatalf("голос инициатора: %d %v", status, body)
	}

	if err := db.SaveDialog(ctx, 10); err != nil {
		t.Fatal(err)
	}
	if status, body := initiator.do("POST", meetingPath+"/bulletin", nil); status != http.StatusOK || body["sent"] != true {
		t.Errorf("бюллетень при открытом диалоге: %d %v", status, body)
	}
}
