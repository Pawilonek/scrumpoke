package store

import "sync"

type User struct {
	UUID string
	Name string
}

type Users interface {
	Get(uuid string) (User, bool)
	Upsert(uuid, name string)
}

// InMemoryUsers keeps a single latest display name per uuid.
type InMemoryUsers struct {
	mu    sync.RWMutex
	users map[string]User
}

func NewInMemoryUsers() *InMemoryUsers {
	return &InMemoryUsers{
		users: make(map[string]User),
	}
}

func (s *InMemoryUsers) Get(uuid string) (User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[uuid]
	return u, ok
}

func (s *InMemoryUsers) Upsert(uuid, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[uuid] = User{UUID: uuid, Name: name}
}

