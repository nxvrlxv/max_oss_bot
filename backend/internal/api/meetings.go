package api

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"oss-max/internal/bot"
	"oss-max/internal/domain"
	"oss-max/internal/registry"
	"oss-max/internal/storage"
)

// Заготовки правил, которые инициатор выбирает в мини-приложении.
// Свои пороги не даём: неправильный порог — это незаконное решение.
var rulePresets = map[string]domain.Rule{
	"soft": domain.RuleSoft(),
	"hard": domain.RuleHard(),
	"all":  domain.RuleAll(),
}

type ruleView struct {
	Kind      string  `json:"kind"`  // soft, hard, all или custom
	Label     string  `json:"label"` // коротко: «2/3 «за»»
	Base      string  `json:"base"`
	Threshold float64 `json:"threshold"`
	Op        string  `json:"op"`
}

func viewRule(rule domain.Rule) ruleView {
	view := ruleView{Kind: "custom", Label: "Порог «за»", Base: string(rule.Base),
		Threshold: float64(rule.Threshold), Op: string(rule.Op)}
	for kind, preset := range rulePresets {
		if preset == rule {
			view.Kind = kind
		}
	}
	switch view.Kind {
	case "soft":
		view.Label = "Большинство «за»"
	case "hard":
		view.Label = "2/3 «за»"
	case "all":
		view.Label = "Все «за»"
	}
	return view
}

type registryView struct {
	Flats int     `json:"flats"`
	Area  float64 `json:"area"`
}

// summaryView — итог для карточки. Доли — от площади дома, от 0 до 1.
type summaryView struct {
	Turnout  float64 `json:"turnout"`
	For      float64 `json:"for"`
	Against  float64 `json:"against"`
	Abstain  float64 `json:"abstain"`
	Quorum   bool    `json:"quorum"`
	Accepted bool    `json:"accepted"`
}

type meetingView struct {
	ID             int             `json:"id"`
	Question       string          `json:"question"`
	Address        string          `json:"address"`
	Status         string          `json:"status"` // с учётом истёкшего срока
	StartsAt       *time.Time      `json:"starts_at,omitempty"`
	EndsAt         *time.Time      `json:"ends_at,omitempty"`
	Rule           ruleView        `json:"rule"`
	TotalArea      float64         `json:"total_area"`
	EntrancesCount int             `json:"entrances_count"`
	IsInitiator    bool            `json:"is_initiator"`
	ChatBound      bool            `json:"chat_bound"`
	InviteLink     string          `json:"invite_link,omitempty"` // пока идёт голосование: позвать соседей
	Claims         []storage.Claim `json:"claims"`
	Choice         domain.Choice   `json:"choice,omitempty"`
	VotedAt        *time.Time      `json:"voted_at,omitempty"`
	Registry       *registryView   `json:"registry,omitempty"`
	Summary        *summaryView    `json:"summary,omitempty"`
}

// view собирает собрание глазами конкретного человека.
func (s *Server) view(r *http.Request, meeting storage.Meeting, withRegistry bool) (meetingView, error) {
	ctx := r.Context()
	user := currentUser(r)

	view := meetingView{
		ID:             meeting.ID,
		Question:       meeting.Question,
		Address:        meeting.Address,
		Status:         meeting.Status,
		StartsAt:       meeting.StartsAt,
		EndsAt:         meeting.EndsAt,
		Rule:           viewRule(meeting.Rule),
		TotalArea:      meeting.TotalArea,
		EntrancesCount: meeting.EntrancesCount,
		IsInitiator:    meeting.InitiatorID == user.ID,
		Claims:         []storage.Claim{},
	}
	if meeting.Closed(time.Now()) {
		view.Status = storage.MeetingFinished
	}
	if view.IsInitiator {
		view.ChatBound = meeting.ChatID != 0
	}
	// Ссылку видит каждый, кто видит собрание: позвать соседа может и собственник.
	if view.Status == storage.MeetingActive && s.cfg.BotName != "" {
		view.InviteLink = bot.InviteLink(s.cfg.BotName, meeting.InviteToken)
	}

	claims, err := s.store.UserClaims(ctx, meeting.ID, user.ID)
	if err != nil {
		return view, err
	}
	view.Claims = claims
	for _, claim := range claims {
		if claim.Choice != "" {
			view.Choice, view.VotedAt = claim.Choice, claim.VotedAt
		}
	}

	if view.IsInitiator && withRegistry {
		flats, err := s.store.Flats(ctx, meeting.ID)
		if err != nil {
			return view, err
		}
		reg := registryView{Flats: len(flats)}
		for _, flat := range flats {
			reg.Area += flat.Area
		}
		view.Registry = &reg
	}

	// Итог нужен инициатору на карточке и всем — после завершения.
	if view.Status != storage.MeetingDraft && (view.IsInitiator || view.Status == storage.MeetingFinished) {
		result, err := s.store.Result(ctx, meeting.ID)
		if err != nil {
			return view, err
		}
		summary := summaryView{Quorum: result.Quorum, Accepted: result.Accepted}
		if total := result.TotalArea; total > 0 {
			summary.Turnout = result.Tally.Total / total
			summary.For = result.Tally.For / total
			summary.Against = result.Tally.Against / total
			summary.Abstain = result.Tally.Abstain / total
		}
		view.Summary = &summary
	}

	return view, nil
}

// join — вход по приглашению: токен из ссылки, QR или кнопки в чате
// превращается в номер собрания. Черновик по приглашению не открывается.
func (s *Server) join(w http.ResponseWriter, r *http.Request) {
	meeting, err := s.store.MeetingByToken(r.Context(), r.PathValue("token"))
	if err != nil || meeting.Status == storage.MeetingDraft {
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			serverError(w, r, err)
			return
		}
		writeError(w, http.StatusNotFound, "Приглашение недействительно — попросите у инициатора новую ссылку")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"id": meeting.ID})
}

// me — главный экран: кто я и мои собрания.
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)

	meetings, err := s.store.MeetingsForUser(r.Context(), user.ID)
	if err != nil {
		serverError(w, r, err)
		return
	}

	views := make([]meetingView, 0, len(meetings))
	for _, meeting := range meetings {
		view, err := s.view(r, meeting, false)
		if err != nil {
			serverError(w, r, err)
			return
		}
		views = append(views, view)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user":     map[string]any{"id": user.ID, "name": user.Name()},
		"meetings": views,
	})
}

func (s *Server) meeting(w http.ResponseWriter, r *http.Request) {
	meeting, ok := s.visibleMeeting(w, r)
	if !ok {
		return
	}
	view, err := s.view(r, meeting, true)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

type meetingInput struct {
	Address        string     `json:"address"`
	Question       string     `json:"question"`
	Rule           string     `json:"rule"` // soft, hard, all
	TotalArea      float64    `json:"total_area"`
	EntrancesCount int        `json:"entrances_count"`
	EndsAt         *time.Time `json:"ends_at"`
}

func (in meetingInput) toNew(initiator int64) (storage.NewMeeting, error) {
	rule, ok := rulePresets[in.Rule]
	if !ok {
		return storage.NewMeeting{}, storage.ErrInvalid{Reason: "Выберите правило принятия решения"}
	}
	if in.EndsAt != nil && !in.EndsAt.After(time.Now()) {
		return storage.NewMeeting{}, storage.ErrInvalid{Reason: "Срок голосования должен быть в будущем"}
	}
	return storage.NewMeeting{
		InitiatorMaxID: initiator,
		Address:        in.Address,
		Question:       in.Question,
		Rule:           rule,
		TotalArea:      in.TotalArea,
		EntrancesCount: in.EntrancesCount,
		EndsAt:         in.EndsAt,
	}, nil
}

func (s *Server) createMeeting(w http.ResponseWriter, r *http.Request) {
	var in meetingInput
	if !readJSON(w, r, &in) {
		return
	}
	draft, err := in.toNew(currentUser(r).ID)
	if err != nil {
		storeError(w, r, err)
		return
	}
	meeting, err := s.store.CreateMeeting(r.Context(), draft)
	if err != nil {
		storeError(w, r, err)
		return
	}
	s.respondMeeting(w, r, meeting.ID, http.StatusCreated)
}

func (s *Server) updateMeeting(w http.ResponseWriter, r *http.Request) {
	meeting := meetingFrom(r)

	var in meetingInput
	if !readJSON(w, r, &in) {
		return
	}
	draft, err := in.toNew(meeting.InitiatorID)
	if err != nil {
		storeError(w, r, err)
		return
	}
	if err := s.store.UpdateDraft(r.Context(), meeting.ID, draft); err != nil {
		storeError(w, r, err)
		return
	}
	s.respondMeeting(w, r, meeting.ID, http.StatusOK)
}

func (s *Server) respondMeeting(w http.ResponseWriter, r *http.Request, meetingID, status int) {
	meeting, err := s.store.Meeting(r.Context(), meetingID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	view, err := s.view(r, meeting, true)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, status, view)
}

// Реестр на 1000 квартир с долевой собственностью — около 300 КБ.
const maxRegistrySize = 5 << 20

// uploadRegistry принимает CSV телом запроса и заменяет реестр собрания.
// Ошибка в любой строке отклоняет файл целиком: частичный реестр опаснее пустого.
func (s *Server) uploadRegistry(w http.ResponseWriter, r *http.Request) {
	meeting := meetingFrom(r)
	if meeting.Status != storage.MeetingDraft {
		storeError(w, r, storage.ErrNotDraft)
		return
	}

	rows, err := registry.ParseCSV(http.MaxBytesReader(w, r.Body, maxRegistrySize))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "Файл больше 5 МБ")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	flats, report, err := registry.Group(rows)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.store.ImportRegistry(r.Context(), meeting.ID, flats); err != nil {
		storeError(w, r, err)
		return
	}

	warnings := append(report.CheckTotalArea(strconv.FormatFloat(meeting.TotalArea, 'f', 2, 64)), report.Warnings...)
	writeJSON(w, http.StatusOK, map[string]any{
		"house_address": report.HouseAddress,
		"flats":         report.Flats,
		"owners":        report.Owners,
		"flats_area":    report.FlatsArea,
		"owned_area":    report.OwnedArea,
		"warnings":      append([]string{}, warnings...),
	})
}

func (s *Server) publish(w http.ResponseWriter, r *http.Request) {
	meeting := meetingFrom(r)

	flats, err := s.store.Flats(r.Context(), meeting.ID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if len(flats) == 0 {
		writeError(w, http.StatusConflict, "Сначала загрузите реестр собственников")
		return
	}
	if meeting.EndsAt == nil || !meeting.EndsAt.After(time.Now()) {
		writeError(w, http.StatusConflict, "Укажите срок голосования в будущем")
		return
	}

	if err := s.store.Publish(r.Context(), meeting.ID); err != nil {
		storeError(w, r, err)
		return
	}

	// Публикация в чат — не повод откатывать собрание: ссылку можно разослать руками.
	if err := s.notifier.AnnounceMeeting(r.Context(), meeting); err != nil {
		log.Printf("публикация собрания %d в чат %d: %v", meeting.ID, meeting.ChatID, err)
	}

	s.respondMeeting(w, r, meeting.ID, http.StatusOK)
}

func (s *Server) finish(w http.ResponseWriter, r *http.Request) {
	meeting := meetingFrom(r)
	if err := s.store.Finish(r.Context(), meeting.ID); err != nil {
		storeError(w, r, err)
		return
	}
	s.respondMeeting(w, r, meeting.ID, http.StatusOK)
}

// flats — номера помещений для выбора своей квартиры. ФИО здесь нет.
func (s *Server) flats(w http.ResponseWriter, r *http.Request) {
	meeting, ok := s.visibleMeeting(w, r)
	if !ok {
		return
	}
	flats, err := s.store.Flats(r.Context(), meeting.ID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, flats)
}
