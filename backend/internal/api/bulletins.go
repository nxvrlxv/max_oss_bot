package api

import (
	"errors"
	"log"
	"net/http"

	"oss-max/internal/bot"
	"oss-max/internal/storage"
)

// bulletin — «Получить бюллетень для подписи»: собственнику его бюллетень
// в личку. Кому выдавать, решает bot.Bulletins: нужна подтверждённая заявка.
func (s *Server) bulletin(w http.ResponseWriter, r *http.Request) {
	meeting, ok := s.loadMeeting(w, r)
	if !ok {
		return
	}
	s.deliverBulletins(w, r, meeting, bot.ActionBlank)
}

// blanks — «Бланки для обхода»: инициатору бюллетени тех, чей голос не учтён.
func (s *Server) blanks(w http.ResponseWriter, r *http.Request) {
	s.deliverBulletins(w, r, meetingFrom(r), bot.ActionBlanks)
}

// deliverBulletins присылает PDF через бота. Написать первым бот может,
// только если человек уже нажимал «Начать». Иначе отдаём ссылку на чат
// с ботом: после «Начать» по ней бот пришлёт файл сам.
func (s *Server) deliverBulletins(w http.ResponseWriter, r *http.Request, meeting storage.Meeting, action bot.Action) {
	ctx := r.Context()
	user := currentUser(r)

	bulletins, err := bot.Bulletins(ctx, s.store, meeting, user.ID, action)
	switch {
	case errors.Is(err, bot.ErrBulletinsClosed), errors.Is(err, bot.ErrNotConfirmed), errors.Is(err, bot.ErrNobodyLeft):
		writeError(w, http.StatusConflict, err.Error())
		return
	case err != nil:
		storeError(w, r, err)
		return
	}

	active, err := s.store.DialogActive(ctx, user.ID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if active {
		err := s.notifier.SendBulletins(ctx, user.ID, meeting, action, bulletins)
		if err == nil {
			writeJSON(w, http.StatusOK, map[string]any{"sent": true})
			return
		}
		// Человек мог остановить бота — тогда поможет только «Начать» заново.
		log.Printf("бюллетени собрания %d пользователю %d: %v", meeting.ID, user.ID, err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"sent": false, "link": bot.ChatLink(s.cfg.BotName, action, meeting.ID)})
}
