package api

import (
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"
)

const testToken = "test-bot-token"

// signed собирает initData так же, как MAX: подписывает все поля, кроме hash.
func signed(fields map[string]string, token string) string {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	lines := make([]string, len(keys))
	values := url.Values{}
	for i, key := range keys {
		lines[i] = key + "=" + fields[key]
		values.Set(key, fields[key])
	}

	secret := sign([]byte("WebAppData"), []byte(token))
	values.Set("hash", hex.EncodeToString(sign(secret, []byte(strings.Join(lines, "\n")))))
	return values.Encode()
}

func TestValidateInitData(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	fields := map[string]string{
		"auth_date":   "1799999000",
		"query_id":    "4c0ab423-342b-4e45-aea4-2747dbc500cd",
		"user":        `{"id":67890,"first_name":"Иван","last_name":"Петров","username":"ivan"}`,
		"chat":        `{"id":12345,"type":"DIALOG"}`,
		"start_param": "open:42",
	}

	data, err := ValidateInitData(signed(fields, testToken), testToken, time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	if data.User.ID != 67890 || data.User.Name() != "Иван Петров" || data.StartParam != "open:42" {
		t.Errorf("разобрано неверно: %+v", data)
	}

	cases := map[string]string{
		"чужой токен":  signed(fields, "other-token"),
		"без подписи":  strings.Split(signed(fields, testToken), "&hash=")[0],
		"подменён id":  strings.Replace(signed(fields, testToken), "67890", "67891", 1),
		"мусор":        "%%%",
		"двойной hash": signed(fields, testToken) + "&hash=00",
	}
	for name, raw := range cases {
		if _, err := ValidateInitData(raw, testToken, time.Hour, now); err == nil {
			t.Errorf("%s: принято", name)
		}
	}

	if _, err := ValidateInitData(signed(fields, testToken), testToken, time.Minute, now); err != errExpired {
		t.Errorf("старые данные: %v", err)
	}

	noUser := map[string]string{"auth_date": "1799999000"}
	if _, err := ValidateInitData(signed(noUser, testToken), testToken, time.Hour, now); err != errNoUser {
		t.Errorf("без пользователя: %v", err)
	}
}
