package bot

import (
	"strconv"
	"strings"
)

// Action — что делает кнопка. Payload ходит в трёх местах: в callback-кнопках,
// в кнопке запуска мини-приложения и в deep-link, формат везде один.
type Action string

const (
	ActionBind   Action = "bind"   // привязать чат к собранию
	ActionOpen   Action = "open"   // открыть собрание в мини-приложении
	ActionList   Action = "list"   // список собраний инициатора
	ActionClaims Action = "claims" // очередь заявок на привязку
	ActionNew    Action = "new"    // создать собрание
)

// Payload — разобранная нагрузка кнопки: действие и необязательный номер.
type Payload struct {
	Action Action
	ID     int
}

// Format собирает строку для кнопки: "open:42" или "new".
func Format(action Action, id int) string {
	if id == 0 {
		return string(action)
	}
	return string(action) + ":" + strconv.Itoa(id)
}

// Parse разбирает нагрузку. Второе значение — false, если строка пустая
// или испорчена: такие нажатия игнорируются молча.
func Parse(raw string) (Payload, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Payload{}, false
	}

	action, rest, found := strings.Cut(raw, ":")
	payload := Payload{Action: Action(action)}

	if found {
		id, err := strconv.Atoi(rest)
		if err != nil {
			return Payload{}, false
		}
		payload.ID = id
	}

	switch payload.Action {
	case ActionBind, ActionOpen, ActionList, ActionClaims, ActionNew:
		return payload, true
	default:
		return Payload{}, false
	}
}
