package bot

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Action — что делает кнопка. Payload ходит в трёх местах: в callback-кнопках,
// в кнопке запуска мини-приложения и в deep-link, формат везде один.
type Action string

const (
	ActionBind   Action = "bind"   // привязать чат к собранию
	ActionOpen   Action = "open"   // открыть своё собрание: инициатору или тому, кто уже подал заявку
	ActionJoin   Action = "join"   // войти в собрание по приглашению: чат дома, ссылка, QR
	ActionList   Action = "list"   // список собраний инициатора
	ActionClaims Action = "claims" // очередь заявок на привязку
	ActionNew    Action = "new"    // создать собрание
)

// Payload — разобранная нагрузка кнопки: действие и номер собрания
// или токен приглашения.
type Payload struct {
	Action Action
	ID     int
	Token  string
}

// Токен приглашения — votings.invite_token, 32 шестнадцатеричных символа.
// Длину не фиксируем жёстко, но мусор в кнопку не пропускаем.
var tokenPattern = regexp.MustCompile(`^[0-9a-f]{16,64}$`)

// Format собирает строку для кнопки: "open_42" или "new".
// Разделитель — подчёркивание: payload кнопки мини-приложения MAX
// принимает только буквы, цифры, «_» и «-», двоеточие отклоняет.
func Format(action Action, id int) string {
	if id == 0 {
		return string(action)
	}
	return string(action) + "_" + strconv.Itoa(id)
}

// FormatJoin — нагрузка приглашения: "join_<токен>". Номер собрания
// наружу не отдаём: номера идут подряд и перебираются.
func FormatJoin(token string) string {
	return string(ActionJoin) + "_" + token
}

// InviteLink — ссылка, которая сразу открывает мини-приложение на собрании.
// Её пересылают соседям и печатают QR-кодом в объявлении.
// Формат из документации MAX: https://max.ru/<бот>?startapp=<payload>.
func InviteLink(botName, token string) string {
	return "https://max.ru/" + url.PathEscape(botName) + "?startapp=" + FormatJoin(token)
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

	switch payload.Action {
	case ActionJoin:
		if !tokenPattern.MatchString(rest) {
			return Payload{}, false
		}
		payload.Token = rest
		return payload, true

	case ActionBind, ActionOpen, ActionList, ActionClaims, ActionNew:
		if found {
			id, err := strconv.Atoi(rest)
			if err != nil {
				return Payload{}, false
			}
			payload.ID = id
		}
		return payload, true

	default:
		return Payload{}, false
	}
}
