package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"oss-max/internal/storage"
)

// initDataMaxAge — сколько живёт initData. Мини-приложение держат открытым
// долго, но сутки — верхняя граница: дальше строку проще перехватить, чем получить честно.
const initDataMaxAge = 24 * time.Hour

// Notifier — сообщения в MAX от имени бота. Интерфейс, чтобы API
// можно было поднять без токена в тестах.
type Notifier interface {
	AnnounceMeeting(ctx context.Context, meeting storage.Meeting) error
	NotifyClaim(ctx context.Context, meeting storage.Meeting, flatNumber, userName string) error
}

// Config — то, что серверу нужно знать о боте и окружении.
type Config struct {
	BotToken string // проверка подписи initData
	BotName  string // username бота: из него собирается ссылка-приглашение
	DevMaxID int64  // пользователь для разработки в браузере без MAX; 0 — выключено
}

type Server struct {
	store    *storage.Store
	notifier Notifier
	cfg      Config
}

func New(store *storage.Store, notifier Notifier, cfg Config) *Server {
	return &Server{store: store, notifier: notifier, cfg: cfg}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Собственник
	mux.HandleFunc("GET /api/me", s.auth(s.me))
	mux.HandleFunc("GET /api/join/{token}", s.auth(s.join))
	mux.HandleFunc("GET /api/meetings/{id}", s.auth(s.meeting))
	mux.HandleFunc("GET /api/meetings/{id}/flats", s.auth(s.flats))
	mux.HandleFunc("POST /api/meetings/{id}/claims", s.auth(s.claimFlat))
	mux.HandleFunc("DELETE /api/meetings/{id}/claims/{claim}", s.auth(s.cancelClaim))
	mux.HandleFunc("POST /api/meetings/{id}/vote", s.auth(s.vote))

	// Инициатор
	mux.HandleFunc("POST /api/meetings", s.auth(s.createMeeting))
	mux.HandleFunc("PUT /api/meetings/{id}", s.auth(s.initiator(s.updateMeeting)))
	mux.HandleFunc("POST /api/meetings/{id}/registry", s.auth(s.initiator(s.uploadRegistry)))
	mux.HandleFunc("POST /api/meetings/{id}/publish", s.auth(s.initiator(s.publish)))
	mux.HandleFunc("POST /api/meetings/{id}/finish", s.auth(s.initiator(s.finish)))
	mux.HandleFunc("GET /api/meetings/{id}/dashboard", s.auth(s.initiator(s.dashboard)))
	mux.HandleFunc("GET /api/meetings/{id}/claims", s.auth(s.initiator(s.pendingClaims)))
	mux.HandleFunc("GET /api/meetings/{id}/flats/{number}/owners", s.auth(s.initiator(s.flatOwners)))
	mux.HandleFunc("POST /api/meetings/{id}/claims/{claim}/confirm", s.auth(s.initiator(s.confirmClaim)))
	mux.HandleFunc("POST /api/meetings/{id}/claims/{claim}/reject", s.auth(s.initiator(s.rejectClaim)))

	return logRequests(mux)
}

type ctxKey struct{}

// auth пропускает запрос, только если initData подписана токеном нашего бота.
// Пользователь из initData сохраняется: иначе инициатор увидит в заявке пустое имя.
func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw := r.Header.Get("X-Max-Init-Data")

		var user InitUser
		switch {
		case raw != "":
			data, err := ValidateInitData(raw, s.cfg.BotToken, initDataMaxAge, time.Now())
			if err != nil {
				writeError(w, http.StatusUnauthorized, "Откройте приложение заново из MAX")
				return
			}
			user = data.User
		case s.cfg.DevMaxID != 0:
			user = InitUser{ID: s.cfg.DevMaxID, FirstName: "Разработчик"}
		default:
			writeError(w, http.StatusUnauthorized, "Откройте приложение из MAX")
			return
		}

		err := s.store.SaveUser(r.Context(), storage.User{MaxID: user.ID, Name: user.Name(), Username: user.Username})
		if err != nil {
			serverError(w, r, err)
			return
		}

		next(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, user)))
	}
}

func currentUser(r *http.Request) InitUser {
	user, _ := r.Context().Value(ctxKey{}).(InitUser)
	return user
}

type meetingKey struct{}

// initiator загружает собрание из пути и пускает только его инициатора.
// Чужому отвечаем 404, а не 403: номер собрания подбирается перебором.
func (s *Server) initiator(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		meeting, ok := s.loadMeeting(w, r)
		if !ok {
			return
		}
		if meeting.InitiatorID != currentUser(r).ID {
			writeError(w, http.StatusNotFound, "Собрание не найдено")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), meetingKey{}, meeting)))
	}
}

func meetingFrom(r *http.Request) storage.Meeting {
	meeting, _ := r.Context().Value(meetingKey{}).(storage.Meeting)
	return meeting
}

func (s *Server) loadMeeting(w http.ResponseWriter, r *http.Request) (storage.Meeting, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "Собрание не найдено")
		return storage.Meeting{}, false
	}
	meeting, err := s.store.Meeting(r.Context(), id)
	if err != nil {
		storeError(w, r, err)
		return storage.Meeting{}, false
	}
	return meeting, true
}

// visibleMeeting загружает собрание и проверяет, что человеку можно его видеть.
// Номера собраний идут подряд, поэтому одного номера мало: посторонний
// должен прийти по приглашению — с токеном из ссылки, QR или кнопки в чате.
// Инициатору и тем, кто уже подал заявку, токен не нужен.
func (s *Server) visibleMeeting(w http.ResponseWriter, r *http.Request) (storage.Meeting, bool) {
	meeting, ok := s.loadMeeting(w, r)
	if !ok {
		return meeting, false
	}

	user := currentUser(r)
	if meeting.InitiatorID == user.ID {
		return meeting, true
	}
	// Черновик видит только инициатор: вопрос ещё может поменяться.
	if meeting.Status == storage.MeetingDraft {
		writeError(w, http.StatusNotFound, "Собрание ещё не опубликовано")
		return meeting, false
	}

	token := r.Header.Get("X-Invite-Token")
	if token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(meeting.InviteToken)) == 1 {
		return meeting, true
	}

	has, err := s.store.HasClaim(r.Context(), meeting.ID, user.ID)
	if err != nil {
		serverError(w, r, err)
		return meeting, false
	}
	if !has {
		writeError(w, http.StatusNotFound, "Собрание не найдено. Откройте его по ссылке или кнопке из чата дома")
		return meeting, false
	}
	return meeting, true
}

func pathInt(r *http.Request, name string) (int, bool) {
	value, err := strconv.Atoi(r.PathValue(name))
	return value, err == nil
}

// readJSON разбирает тело запроса. Лимит — от случайной отправки файла в JSON-ручку.
func readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "Некорректный запрос")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// writeError — текст ошибки показывается пользователю как есть, поэтому по-русски.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func serverError(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
	writeError(w, http.StatusInternalServerError, "Что-то пошло не так, попробуйте ещё раз")
}

// storeError переводит ошибки хранилища в ответы для пользователя.
func storeError(w http.ResponseWriter, r *http.Request, err error) {
	var invalid storage.ErrInvalid
	switch {
	case errors.As(err, &invalid):
		writeError(w, http.StatusBadRequest, invalid.Reason)
	case errors.Is(err, storage.ErrNotFound):
		writeError(w, http.StatusNotFound, "Не найдено")
	case errors.Is(err, storage.ErrNotDraft):
		writeError(w, http.StatusConflict, "Собрание уже опубликовано, менять его нельзя")
	case errors.Is(err, storage.ErrCannotVote):
		writeError(w, http.StatusConflict, "Голосование сейчас не идёт")
	case errors.Is(err, storage.ErrTooManyClaims):
		writeError(w, http.StatusTooManyRequests, "У вас уже пять заявок без ответа — дождитесь решения инициатора")
	case errors.Is(err, storage.ErrOwnerTaken):
		writeError(w, http.StatusConflict, "Этот собственник уже подтверждён за другим человеком")
	default:
		serverError(w, r, err)
	}
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}
