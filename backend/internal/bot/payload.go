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

// Format собирает строку для кнопки: "open_42" или "new".
// Разделитель — подчёркивание: payload кнопки мини-приложения MAX
// принимает только буквы, цифры, «_» и «-», двоеточие отклоняет.
func Format(action Action, id int) string {
	if id == 0 {
		return string(action)
	}
	return string(action) + "_" + strconv.Itoa(id)
}

// Parse разбирает нагрузку. Второе значение — false, если строка пустая
// или испорчена: такие нажатия игнорируются молча.
func Parse(raw string) (Payload, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Payload{}, false
	}

	// Двоеточие — старый формат, в уже отправленных кнопках он остался.
	action, rest, found := strings.Cut(strings.Replace(raw, ":", "_", 1), "_")
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
