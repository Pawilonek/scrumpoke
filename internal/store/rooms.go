package store

import (
	"sync"
	"time"

	"github.com/Pawilonek/scrumpoke/internal/game"
)

type Rooms interface {
	Get(slug string) (*game.Room, bool)
	GetOrCreate(slug string) *game.Room
}

type InMemoryRooms struct {
	mu           sync.RWMutex
	rooms        map[string]*game.Room
	initialCards []string
	gracePeriod time.Duration
	events       chan<- game.RoomSnapshot
}

func NewInMemoryRooms(initialCards []string, gracePeriod time.Duration, events chan<- game.RoomSnapshot) *InMemoryRooms {
	return &InMemoryRooms{
		rooms:        make(map[string]*game.Room),
		initialCards: append([]string(nil), initialCards...),
		gracePeriod: gracePeriod,
		events:       events,
	}
}

func (s *InMemoryRooms) Get(slug string) (*game.Room, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	room, ok := s.rooms[slug]
	return room, ok
}

func (s *InMemoryRooms) GetOrCreate(slug string) *game.Room {
	s.mu.RLock()
	room, ok := s.rooms[slug]
	s.mu.RUnlock()
	if ok {
		return room
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	// Double-check after acquiring write lock.
	if room, ok := s.rooms[slug]; ok {
		return room
	}
	room = game.NewRoom(slug, s.initialCards, s.gracePeriod, s.events)
	s.rooms[slug] = room
	return room
}

