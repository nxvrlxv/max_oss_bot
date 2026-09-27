package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"oss-max/internal/storage"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestNeighbourSelectionAndRevoke(t *testing.T) {
	server := testServer(t)
	initiator := client{t: t, base: server.URL, userID: 10}
	voter := client{t: t, base: server.URL, userID: 20}
	_, created := initiator.do("POST", "/api/meetings", map[string]any{"address": "Дом", "question": "Вопрос", "rule": "soft", "ends_at": time.Now().Add(time.Hour).Format(time.RFC3339)})
	path := fmt.Sprintf("/api/meetings/%d", int(created["id"].(float64)))
	csv := "№ помещения;Адрес объекта;Правообладатель (правообладатели);Долевая площадь, м²;Общая площадь, м²\n1;Дом, кв. 1;Иванов Иван;30;80\n1;Дом, кв. 1;Петров Пётр;50;80\n"
	if status, body := initiator.do("POST", path+"/registry", csv); status != 200 {
		t.Fatalf("registry: %d %v", status, body)
	}
	_, published := initiator.do("POST", path+"/publish", nil)
	voter.invite = strings.TrimPrefix(published["invite_link"].(string), "https://max.ru/oss_bot?startapp=join_")
	req, _ := http.NewRequest("GET", server.URL+path+"/flats/1/owners", nil)
	req.Header.Set("X-Max-Init-Data", signed(map[string]string{"auth_date": strconv.FormatInt(time.Now().Unix(), 10), "user": `{"id":20}`}, testToken))
	req.Header.Set("X-Invite-Token", voter.invite)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var owners []storage.RegistryOwner
	err = json.NewDecoder(resp.Body).Decode(&owners)
	resp.Body.Close()
	if resp.StatusCode != 200 || err != nil || len(owners) != 2 {
		t.Fatalf("owners: %d %+v %v", resp.StatusCode, owners, err)
	}
	_, before := voter.do("GET", path, nil)
	if len(before["claims"].([]any)) != 0 {
		t.Fatal("selection created claim")
	}
	if status, _ := voter.do("POST", path+"/vote", map[string]any{"choice": "for", "flat_number": "1"}); status != 400 {
		t.Fatalf("missing owner: %d", status)
	}
	status, after := voter.do("POST", path+"/vote", map[string]any{"choice": "for", "flat_number": "1", "owner_id": owners[0].ID})
	if status != 200 {
		t.Fatalf("vote: %d %v", status, after)
	}
	claims := after["claims"].([]any)
	claim := claims[0].(map[string]any)
	if claim["status"] != "pending" || claim["weight"] != float64(30) {
		t.Fatalf("claim: %v", claim)
	}
	claimPath := fmt.Sprintf("%s/claims/%d", path, int(claim["id"].(float64)))
	if status, _ := voter.do("POST", claimPath+"/confirm", map[string]int{"owner_id": owners[0].ID}); status != 404 {
		t.Fatal("voter confirmed self")
	}
	// Старый клиент не может подменить выбор участника своим owner_id.
	if status, body := initiator.do("POST", claimPath+"/confirm", map[string]int{"owner_id": owners[1].ID}); status != 204 {
		t.Fatalf("confirm: %d %v", status, body)
	}
	_, board := initiator.do("GET", path+"/dashboard", nil)
	if board["tally"].(map[string]any)["total"] != float64(30) {
		t.Fatalf("share tally: %v", board)
	}
	if status, _ := voter.do("GET", path+"/claims?status=confirmed", nil); status != 404 {
		t.Fatal("voter read review list")
	}
	if status, _ := voter.do("POST", claimPath+"/revoke", nil); status != 404 {
		t.Fatal("voter revoked confirmation")
	}
	if status, body := initiator.do("POST", claimPath+"/revoke", nil); status != 204 {
		t.Fatalf("revoke: %d %v", status, body)
	}
	_, board = initiator.do("GET", path+"/dashboard", nil)
	if board["tally"].(map[string]any)["total"] != float64(0) || board["pending_claims"] != float64(1) {
		t.Fatalf("revoke tally: %v", board)
	}
	if status, body := initiator.do("POST", claimPath+"/confirm", nil); status != 204 {
		t.Fatalf("confirm without owner input: %d %v", status, body)
	}
	_, board = initiator.do("GET", path+"/dashboard", nil)
	if board["tally"].(map[string]any)["total"] != float64(30) {
		t.Fatalf("owner changed on reconfirm: %v", board)
	}
}
