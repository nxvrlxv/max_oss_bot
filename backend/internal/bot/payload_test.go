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
		"open_42":  {Action: ActionOpen, ID: 42},
		"open:42":  {Action: ActionOpen, ID: 42}, // старые кнопки
		"claims_7": {Action: ActionClaims, ID: 7},
		" new ":    {Action: ActionNew},
	} {
		got, ok := Parse(raw)
		if !ok || got != want {
			t.Errorf("Parse(%q) = %+v, %v — хотели %+v", raw, got, ok, want)
		}
	}

	for _, raw := range []string{"", "open_", "open_x", "hack_1", "open_1_2"} {
		if _, ok := Parse(raw); ok {
			t.Errorf("Parse(%q) принял мусор", raw)
		}
	}
}
