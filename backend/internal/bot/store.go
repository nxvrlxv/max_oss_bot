package bot

import (
	"context"
	"sync"

	"oss-max/internal/domain"
	"oss-max/internal/storage"
)

// Типы общие со storage: бот ходит в базу напрямую, без своего API.
type (
	User    = storage.User
	Meeting = storage.Meeting
)

// Store — всё, что обработчики просят у базы. Реализации две:
// storage.Store поверх Postgres и MemoryStore для запуска без базы.
//
// Все методы идемпотентны: одно обновление может прийти дважды.
// Отсутствие записи — storage.ErrNotFound.
type Store interface {
	SaveUser(ctx context.Context, user User) error
	SaveDialog(ctx context.Context, maxID int64) error
	CloseDialog(ctx context.Context, maxID int64) error
	BindChat(ctx context.Context, meetingID int, chatID, initiatorMaxID int64) error
	UnbindChat(ctx context.Context, chatID int64) error
	Meeting(ctx context.Context, meetingID int) (Meeting, error)
	MeetingsByInitiator(ctx context.Context, maxID int64) ([]Meeting, error)
	MeetingsByChat(ctx context.Context, chatID int64) ([]Meeting, error)
	Result(ctx context.Context, meetingID int) (domain.Result, error)
}

// MemoryStore — хранилище в памяти: бот запускается без базы,
// но собраний в нём нет, создавать их нечем.
type MemoryStore struct {
	mu       sync.Mutex
	users    map[int64]User
	dialogs  map[int64]bool
	meetings map[int]Meeting
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		users:    make(map[int64]User),
		dialogs:  make(map[int64]bool),
		meetings: make(map[int]Meeting),
	}
}

func (s *MemoryStore) SaveUser(_ context.Context, user User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[user.MaxID] = user
	return nil
}

func (s *MemoryStore) SaveDialog(_ context.Context, maxID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dialogs[maxID] = true
	return nil
}

func (s *MemoryStore) CloseDialog(_ context.Context, maxID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dialogs[maxID] = false
	return nil
}

func (s *MemoryStore) BindChat(_ context.Context, meetingID int, chatID, initiatorMaxID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	meeting, ok := s.meetings[meetingID]
	if !ok || meeting.InitiatorID != initiatorMaxID {
		return storage.ErrNotFound
	}
	meeting.ChatID = chatID
	s.meetings[meetingID] = meeting
	return nil
}

func (s *MemoryStore) UnbindChat(_ context.Context, chatID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, meeting := range s.meetings {
		if meeting.ChatID == chatID {
			meeting.ChatID = 0
			s.meetings[id] = meeting
		}
	}
	return nil
}

func (s *MemoryStore) Meeting(_ context.Context, meetingID int) (Meeting, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	meeting, ok := s.meetings[meetingID]
	if !ok {
		return Meeting{}, storage.ErrNotFound
	}
	return meeting, nil
}

func (s *MemoryStore) MeetingsByInitiator(_ context.Context, maxID int64) ([]Meeting, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var found []Meeting
	for _, meeting := range s.meetings {
		if meeting.InitiatorID == maxID {
			found = append(found, meeting)
		}
	}
	return found, nil
}

func (s *MemoryStore) MeetingsByChat(_ context.Context, chatID int64) ([]Meeting, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var found []Meeting
	for _, meeting := range s.meetings {
		if meeting.ChatID == chatID && meeting.Status != storage.MeetingFinished {
			found = append(found, meeting)
		}
	}
	return found, nil
}

// Result — голосов в памяти нет, итог пустой.
func (s *MemoryStore) Result(ctx context.Context, meetingID int) (domain.Result, error) {
	meeting, err := s.Meeting(ctx, meetingID)
	if err != nil {
		return domain.Result{}, err
	}
	return domain.Evaluate(meeting.TotalArea, nil, meeting.Rule), nil
}
