package bot

import "testing"

func TestPayload(t *testing.T) {
	if got := Format(ActionOpen, 42); got != "open_42" {
		t.Errorf("Format = %q, хотели open_42", got)
	}
	if got := Format(ActionNew, 0); got != "new" {
		t.Errorf("Format = %q, хотели new", got)
	}

	for raw, want := range map[string]Payload{
		"open_42":                               {Action: ActionOpen, ID: 42},
		"open:42":                               {Action: ActionOpen, ID: 42}, // старые кнопки
		"claims_7":                              {Action: ActionClaims, ID: 7},
		" new ":                                 {Action: ActionNew},
		"join_0123456789abcdef0123456789abcdef": {Action: ActionJoin, Token: "0123456789abcdef0123456789abcdef"},
	} {
		got, ok := Parse(raw)
		if !ok || got != want {
			t.Errorf("Parse(%q) = %+v, %v — хотели %+v", raw, got, ok, want)
		}
	}

	for _, raw := range []string{"", "open_", "open_x", "hack_1", "open_1_2",
		"join", "join_", "join_42", "join_XYZ0123456789abcdef0123456789ab", "join_0123456789abcdef&x=1"} {
		if _, ok := Parse(raw); ok {
			t.Errorf("Parse(%q) принял мусор", raw)
		}
	}
}

func TestInviteLink(t *testing.T) {
	const token = "0123456789abcdef0123456789abcdef"
	want := "https://max.ru/oss_bot?startapp=join_" + token
	if got := InviteLink("oss_bot", token); got != want {
		t.Errorf("InviteLink = %q, хотели %q", got, want)
	}

	// Ссылка должна разбираться обратно тем же Parse: payload приходит во фронт и боту.
	payload, ok := Parse(FormatJoin(token))
	if !ok || payload.Action != ActionJoin || payload.Token != token {
		t.Errorf("Parse(FormatJoin) = %+v, %v", payload, ok)
	}
}
