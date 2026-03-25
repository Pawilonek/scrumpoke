package ws

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"

	"github.com/Pawilonek/scrumpoke/internal/auth"
	"github.com/Pawilonek/scrumpoke/internal/game"
	"github.com/Pawilonek/scrumpoke/internal/store"
)

type Hub struct {
	rooms *store.InMemoryRooms
	users *store.InMemoryUsers
	jwt   *auth.JWTIssuer

	events <-chan game.RoomSnapshot

	mu      sync.Mutex
	clients map[string]map[string]*Client // roomSlug -> uuid -> client
}

func NewHub(rooms *store.InMemoryRooms, users *store.InMemoryUsers, jwt *auth.JWTIssuer, events <-chan game.RoomSnapshot) *Hub {
	return &Hub{
		rooms:   rooms,
		users:   users,
		jwt:     jwt,
		events:  events,
		clients: make(map[string]map[string]*Client),
	}
}

func (h *Hub) Run() {
	go func() {
		for snap := range h.events {
			h.broadcast(snap)
		}
	}()
}

func (h *Hub) broadcast(snap game.RoomSnapshot) {
	h.mu.Lock()
	clients := h.clients[snap.Room]
	// Copy client pointers so we don't hold the hub lock while sending.
	list := make([]*Client, 0, len(clients))
	for _, c := range clients {
		list = append(list, c)
	}
	h.mu.Unlock()

	for _, c := range list {
		msg := stateMessageForClient(snap, c.uuid)
		b, _ := json.Marshal(msg)
		// Unregister may close c.send while we still hold a pointer from the copy above;
		// sending on a closed channel panics and would kill the hub goroutine for everyone.
		trySendJSON(c, b)
	}
}

// trySendJSON delivers to the client's outbound channel without panicking if it is closed.
func trySendJSON(c *Client, b []byte) {
	defer func() {
		_ = recover()
	}()
	select {
	case c.send <- b:
	default:
		// Drop if the client is too slow; the next broadcast will catch up.
	}
}

// Register adds or replaces a client for a room under the hub lock (safe vs concurrent broadcast).
// The previous client for the same uuid (if any) should be shut down by the caller.
func (h *Hub) Register(roomSlug, uuid string, client *Client) (prev *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	m := h.clients[roomSlug]
	if m == nil {
		m = make(map[string]*Client)
		h.clients[roomSlug] = m
	}
	prev = m[uuid]
	m[uuid] = client
	return prev
}

func (h *Hub) unregister(roomSlug, uuid string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	m := h.clients[roomSlug]
	if m == nil {
		return
	}
	cl := m[uuid]
	if cl != nil {
		delete(m, uuid)
		close(cl.send)
		if cl.conn != nil {
			_ = cl.conn.Close()
		}
	}
	if len(m) == 0 {
		delete(h.clients, roomSlug)
	}
}

type stateMessage struct {
	Type      string                `json:"type"`
	Room      string                `json:"room"`
	Cards     []string              `json:"cards"`
	Revealed  bool                  `json:"revealed"`
	CanReveal bool                  `json:"canReveal"`
	Players   []game.PlayerSnapshot `json:"players"`
}

func stateMessageForClient(snap game.RoomSnapshot, clientUUID string) stateMessage {
	players := make([]game.PlayerSnapshot, 0, len(snap.Players))

	for _, p := range snap.Players {
		outP := p
		if !snap.Revealed {
			// Pre-reveal: only expose the client's own selected card.
			if p.UUID != clientUUID {
				outP.VoteCard = nil
			}
		}
		players = append(players, outP)
	}

	return stateMessage{
		Type:      "state",
		Room:      snap.Room,
		Cards:     snap.Cards,
		Revealed:  snap.Revealed,
		CanReveal: snap.CanReveal,
		Players:   players,
	}
}

type Client struct {
	hub  *Hub
	uuid string
	room string

	conn *websocket.Conn
	send chan []byte
}
