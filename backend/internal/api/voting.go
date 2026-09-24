package api

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"oss-max/internal/domain"
	"oss-max/internal/storage"
)

func (s *Server) claimFlat(w http.ResponseWriter, r *http.Request) {
	meeting, ok := s.visibleMeeting(w, r)
	if !ok {
		return
	}

	var in struct {
		FlatNumber string `json:"flat_number"`
		OwnerID    int    `json:"owner_id"` // только инициатор: сразу указывает себя по реестру
	}
	if !readJSON(w, r, &in) {
		return
	}
	number := strings.TrimSpace(in.FlatNumber)
	if number == "" {
		writeError(w, http.StatusBadRequest, "Выберите квартиру")
		return
	}

	user := currentUser(r)
	owner := storage.User{MaxID: user.ID, Name: user.Name(), Username: user.Username}
	isInitiator := meeting.InitiatorID == user.ID

	// Инициатор голосует тем же путём, что все, но свою заявку
	// подтверждает сразу: реестр у него перед глазами.
	if in.OwnerID != 0 {
		if !isInitiator {
			writeError(w, http.StatusForbidden, "Собственника по реестру указывает только инициатор")
			return
		}
		claim, err := s.store.ClaimOwnFlat(r.Context(), meeting.ID, number, owner, in.OwnerID)
		if err != nil {
			storeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, claim)
		return
	}

	claim, fresh, err := s.store.ClaimFlat(r.Context(), meeting.ID, number, owner)
	if err != nil {
		storeError(w, r, err)
		return
	}

	// Уведомление — фоном и только о новой заявке: повторное нажатие
	// не должно слать инициатору второе сообщение. Себе инициатор не пишет.
	if fresh && !isInitiator {
		name := user.Name()
		if name == "" {
			name = "собственник без имени в профиле"
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := s.notifier.NotifyClaim(ctx, meeting, claim.FlatNumber, name); err != nil {
				log.Printf("уведомление о заявке %d: %v", claim.ID, err)
			}
		}()
	}

	writeJSON(w, http.StatusOK, claim)
}

func (s *Server) cancelClaim(w http.ResponseWriter, r *http.Request) {
	meetingID, ok1 := pathInt(r, "id")
	claimID, ok2 := pathInt(r, "claim")
	if !ok1 || !ok2 {
		writeError(w, http.StatusNotFound, "Заявка не найдена")
		return
	}
	if err := s.store.CancelClaim(r.Context(), meetingID, claimID, currentUser(r).ID); err != nil {
		storeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) vote(w http.ResponseWriter, r *http.Request) {
	meeting, ok := s.loadMeeting(w, r)
	if !ok {
		return
	}

	var in struct {
		Choice domain.Choice `json:"choice"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	switch in.Choice {
	case domain.ChoiceFor, domain.ChoiceAgainst, domain.ChoiceAbstain:
	default:
		writeError(w, http.StatusBadRequest, "Выберите вариант ответа")
		return
	}

	if err := s.store.Vote(r.Context(), meeting.ID, currentUser(r).ID, in.Choice); err != nil {
		storeError(w, r, err)
		return
	}
	s.respondMeeting(w, r, meeting.ID, http.StatusOK)
}

type thresholdView struct {
	Label     string  `json:"label"`
	Threshold float64 `json:"threshold"` // доля от базы: 0.5, 0.667, 1
	Strict    bool    `json:"strict"`    // «более чем», а не «не менее»
	Base      float64 `json:"base"`      // от скольких м² считается
	Reached   float64 `json:"reached"`   // сколько м² набрано
	Required  float64 `json:"required"`
	Gap       float64 `json:"gap"`
	Passed    bool    `json:"passed"`
}

func threshold(label string, rule domain.Rule, reached, base float64) thresholdView {
	return thresholdView{
		Label:     label,
		Threshold: float64(rule.Threshold),
		Strict:    rule.Op == domain.OpGreater,
		Base:      base,
		Reached:   reached,
		Required:  rule.Required(base),
		Gap:       rule.Gap(reached, base),
		Passed:    rule.Passed(reached, base),
	}
}

// dashboard — ход голосования для инициатора: оба порога отдельно,
// чтобы было видно, что мешает — явка или «за».
func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	meeting := meetingFrom(r)
	ctx := r.Context()

	result, err := s.store.Result(ctx, meeting.ID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	coverage, err := s.store.Coverage(ctx, meeting.ID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	pending, err := s.store.PendingCount(ctx, meeting.ID)
	if err != nil {
		serverError(w, r, err)
		return
	}

	tally := result.Tally
	decisionBase := meeting.Rule.Denominator(meeting.TotalArea, tally.Total)

	notVotedArea := 0.0
	for _, gap := range coverage.NotVoted {
		notVotedArea += gap.Area
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total_area":     meeting.TotalArea,
		"tally":          map[string]float64{"for": tally.For, "against": tally.Against, "abstain": tally.Abstain, "total": tally.Total},
		"quorum":         threshold("Кворум", domain.RuleQuorum(), tally.Total, meeting.TotalArea),
		"decision":       threshold(viewRule(meeting.Rule).Label, meeting.Rule, tally.For, decisionBase),
		"accepted":       result.Accepted,
		"flats_total":    coverage.FlatsTotal,
		"flats_voted":    coverage.FlatsVoted,
		"not_voted":      coverage.NotVoted,
		"not_voted_area": notVotedArea,
		"pending_claims": pending,
	})
}

// flatOwners — собственники квартиры по реестру, чтобы инициатор выбрал себя.
func (s *Server) flatOwners(w http.ResponseWriter, r *http.Request) {
	owners, err := s.store.FlatOwners(r.Context(), meetingFrom(r).ID, r.PathValue("number"))
	if err != nil {
		serverError(w, r, err)
		return
	}
	if len(owners) == 0 {
		writeError(w, http.StatusNotFound, "Такой квартиры нет в реестре")
		return
	}
	writeJSON(w, http.StatusOK, owners)
}

func (s *Server) pendingClaims(w http.ResponseWriter, r *http.Request) {
	claims, err := s.store.PendingClaims(r.Context(), meetingFrom(r).ID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if claims == nil {
		claims = []storage.PendingClaim{}
	}
	writeJSON(w, http.StatusOK, claims)
}

func (s *Server) confirmClaim(w http.ResponseWriter, r *http.Request) {
	claimID, ok := pathInt(r, "claim")
	if !ok {
		writeError(w, http.StatusNotFound, "Заявка не найдена")
		return
	}
	var in struct {
		OwnerID int `json:"owner_id"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	if err := s.store.ConfirmClaim(r.Context(), meetingFrom(r).ID, claimID, in.OwnerID); err != nil {
		storeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) rejectClaim(w http.ResponseWriter, r *http.Request) {
	claimID, ok := pathInt(r, "claim")
	if !ok {
		writeError(w, http.StatusNotFound, "Заявка не найдена")
		return
	}
	if err := s.store.RejectClaim(r.Context(), meetingFrom(r).ID, claimID); err != nil {
		storeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
