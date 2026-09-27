package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"
	"oss-max/internal/config"
	"oss-max/internal/storage"
)

func TestBindCommand(t *testing.T) {
	for _, tc := range []struct {
		name, command, chatType, status string
		user, existing                  int64
		expired, bound, announce        bool
	}{
		{"draft", "/bind 2", "chat", storage.MeetingDraft, 10, 0, false, true, false},
		{"active", "/bind@oss_bot 2", "chat", storage.MeetingActive, 10, 0, false, true, true},
		{"retry", "/bind 2", "chat", storage.MeetingActive, 10, 555, false, true, true},
		{"other user", "/bind 2", "chat", storage.MeetingActive, 99, 0, false, false, false},
		{"private", "/bind 2", "dialog", storage.MeetingActive, 10, 0, false, false, false},
		{"finished", "/bind 2", "chat", storage.MeetingFinished, 10, 0, false, false, false},
		{"expired", "/bind 2", "chat", storage.MeetingActive, 10, 0, true, false, false},
		{"other chat", "/bind 2", "chat", storage.MeetingActive, 10, 777, false, false, false},
		{"missing id", "/bind", "chat", storage.MeetingActive, 10, 0, false, false, false},
		{"unknown", "/bind 999", "chat", storage.MeetingActive, 10, 0, false, false, false},
		{"invalid", "/bind abc", "chat", storage.MeetingActive, 10, 0, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var messages []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path != "/messages" {
					// QR upload unavailable: the announcement must still contain the link.
					w.WriteHeader(http.StatusBadRequest)
					fmt.Fprint(w, `{"code":"upload.failed","message":"test"}`)
					return
				}
				if r.URL.Query().Get("chat_id") != "555" {
					t.Error("wrong destination")
				}
				var body struct {
					Text string `json:"text"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				messages = append(messages, body.Text)
				fmt.Fprint(w, `{"message":{}}`)
			}))
			defer server.Close()
			api, err := maxbot.New("test-token", maxbot.WithBaseURL(server.URL))
			if err != nil {
				t.Fatal(err)
			}
			store := NewMemoryStore()
			end := time.Now().Add(time.Hour)
			if tc.expired {
				end = time.Now().Add(-time.Hour)
			}
			store.meetings[1] = Meeting{ID: 1, InitiatorID: 10, Question: "Другой дом"}
			store.meetings[2] = Meeting{ID: 2, InitiatorID: 10, Status: tc.status, ChatID: tc.existing,
				Question: "Нужный вопрос", Address: "Нужный дом", InviteToken: "abcdef0123456789", EndsAt: &end}
			var update schemes.MessageCreatedUpdate
			raw := fmt.Sprintf(`{"message":{"sender":{"user_id":%d},"recipient":{"chat_id":555,"chat_type":%q},"body":{"text":%q}}}`, tc.user, tc.chatType, tc.command)
			if err := json.Unmarshal([]byte(raw), &update); err != nil {
				t.Fatal(err)
			}
			b := New(api, config.Config{}, store, "oss_bot")
			if err := b.onMessage(context.Background(), &update); err != nil {
				t.Fatal(err)
			}
			wantChat := tc.existing
			if tc.bound {
				wantChat = 555
			}
			if store.meetings[2].ChatID != wantChat || store.meetings[1].ChatID != 0 {
				t.Fatal("incorrect binding")
			}
			if len(messages) != 1 {
				t.Fatalf("messages: %v", messages)
			}
			if strings.Contains(messages[0], InviteLink("oss_bot", "abcdef0123456789")) != tc.announce {
				t.Fatalf("announcement: %s", messages[0])
			}
			if strings.Contains(messages[0], "Другой дом") {
				t.Fatal("unrelated meeting leaked")
			}
		})
	}
}
