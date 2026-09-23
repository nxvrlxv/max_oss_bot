// Package api — HTTP для мини-приложения. Пользователь определяется
// по initData, которую MAX подписывает токеном бота.
package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// InitData — проверенные данные запуска мини-приложения.
type InitData struct {
	User       InitUser
	StartParam string
	AuthDate   time.Time
}

type InitUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

// Name — имя для инициатора в очереди заявок.
func (u InitUser) Name() string {
	return strings.TrimSpace(u.FirstName + " " + u.LastName)
}

var (
	errNoHash    = errors.New("initData без подписи")
	errBadHash   = errors.New("подпись initData не сходится")
	errExpired   = errors.New("initData устарела")
	errNoUser    = errors.New("в initData нет пользователя")
	errMalformed = errors.New("initData повреждена")
)

// ValidateInitData проверяет подпись по алгоритму из документации MAX:
// secret = HMAC-SHA256(key="WebAppData", msg=BOT_TOKEN), подпись —
// HMAC-SHA256(secret, отсортированные "key=value" через \n без hash).
//
// maxAge ограничивает возраст данных: украденная строка initData
// иначе годилась бы вечно.
func ValidateInitData(raw, botToken string, maxAge time.Duration, now time.Time) (InitData, error) {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return InitData{}, errMalformed
	}

	hashes := values["hash"]
	if len(hashes) != 1 || hashes[0] == "" {
		return InitData{}, errNoHash
	}
	got, err := hex.DecodeString(hashes[0])
	if err != nil {
		return InitData{}, errBadHash
	}

	keys := make([]string, 0, len(values))
	for key, list := range values {
		if key == "hash" {
			continue
		}
		if len(list) != 1 {
			return InitData{}, errMalformed
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	lines := make([]string, len(keys))
	for i, key := range keys {
		lines[i] = key + "=" + values.Get(key)
	}

	secret := sign([]byte("WebAppData"), []byte(botToken))
	want := sign(secret, []byte(strings.Join(lines, "\n")))
	if !hmac.Equal(got, want) {
		return InitData{}, errBadHash
	}

	seconds, err := strconv.ParseInt(values.Get("auth_date"), 10, 64)
	if err != nil {
		return InitData{}, errMalformed
	}
	authDate := time.Unix(seconds, 0)
	if now.Sub(authDate) > maxAge {
		return InitData{}, errExpired
	}

	var user InitUser
	if err := json.Unmarshal([]byte(values.Get("user")), &user); err != nil || user.ID == 0 {
		return InitData{}, errNoUser
	}

	return InitData{User: user, StartParam: values.Get("start_param"), AuthDate: authDate}, nil
}

func sign(key, message []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(message)
	return mac.Sum(nil)
}
