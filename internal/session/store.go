package session

import "sync"

type Store interface {
	Save(s *Session) error
	Current() (*Session, error)
}

type MemoryStore struct {
	mu      sync.Mutex
	session *Session
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

func (m *MemoryStore) Save(s *Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.session = s
	return nil
}

func (m *MemoryStore) Current() (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.session, nil
}
