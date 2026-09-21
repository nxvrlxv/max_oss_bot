package bot

import (
	"context"
	"sync"
)

// User — пользователь MAX, зашедший в бота.
type User struct {
	MaxID    int64
	Name     string
	Username string
}

// Meeting — то, что боту нужно знать о собрании: текст для чата и куда слать.
type Meeting struct {
	ID          int
	InitiatorID int64 // max_id создавшего
	ChatID      int64
	Question    string
	Status      string
}

// Store — всё, что обработчики просят у базы. Реализация подменяется:
// сегодня память, завтра Postgres из internal/storage.
//
// Все методы идемпотентны: одно обновление может прийти дважды.
type Store interface {
	SaveUser(ctx context.Context, user User) error
	SaveDialog(ctx context.Context, maxID int64) error
	CloseDialog(ctx context.Context, maxID int64) error
	BindChat(ctx context.Context, meetingID int, chatID int64) error
	UnbindChat(ctx context.Context, chatID int64) error
	Meeting(ctx context.Context, meetingID int) (Meeting, error)
	MeetingsByInitiator(ctx context.Context, maxID int64) ([]Meeting, error)
}

// MemoryStore — заглушка на время, пока нет базы.
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

func (s *MemoryStore) BindChat(_ context.Context, meetingID int, chatID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	meeting := s.meetings[meetingID]
	meeting.ID = meetingID
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
	return s.meetings[meetingID], nil
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
