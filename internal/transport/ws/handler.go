package ws

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Pawilonek/scrumpoke/internal/auth"
	"github.com/Pawilonek/scrumpoke/internal/game"
	"github.com/Pawilonek/scrumpoke/internal/store"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v5"
)

type inboundMessage struct {
	Type      string `json:"type"`
	Card      string `json:"card,omitempty"`
	CardsText string `json:"cards,omitempty"`
	Name      string `json:"name,omitempty"`
}

type WSHandler struct {
	Rooms *store.InMemoryRooms
	Users *store.InMemoryUsers
	JWT   *auth.JWTIssuer
	Hub   *Hub
	Upg   websocket.Upgrader
}

func NewWSHandler(rooms *store.InMemoryRooms, users *store.InMemoryUsers, jwt *auth.JWTIssuer, hub *Hub) *WSHandler {
	return &WSHandler{
		Rooms: rooms,
		Users: users,
		JWT:   jwt,
		Hub:   hub,
		Upg: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				// For a single-user app, allow same-origin and localhost.
				return true
			},
		},
	}
}

func (h *WSHandler) ServeHTTP() echo.HandlerFunc {
	return func(c *echo.Context) error {
		roomSlug := c.Param("room")
		roomSlug = strings.TrimSpace(roomSlug)
		if err := game.ValidateRoomSlug(strings.ToLower(roomSlug)); err != nil {
			return c.String(http.StatusBadRequest, "invalid room")
		}
		roomSlug = strings.ToLower(roomSlug)

		token := c.QueryParam("token")
		uuid, _, err := h.JWT.Validate(token)
		if err != nil {
			return c.String(http.StatusUnauthorized, "invalid token")
		}

		// Ensure the user exists in the global user store.
		u, ok := h.Users.Get(uuid)
		if !ok || u.Name == "" {
			// If the client opens /room directly, keep the app functional.
			// The join flow normally sets a name.
			u.Name = "Player " + strings.ReplaceAll(uuid, "-", "")[:6]
			// Validate best-effort; if it fails, fall back to something safe.
			if _, vErr := game.ValidatePlayerName(u.Name); vErr != nil {
				u.Name = "Player 1"
			}
			h.Users.Upsert(uuid, u.Name)
		}

		room := h.Rooms.GetOrCreate(roomSlug)

		// Upgrade HTTP -> WS.
		wsConn, err := h.Upg.Upgrade(c.Response(), c.Request(), nil)
		if err != nil {
			return err
		}

		// Register client.
		client := &Client{
			hub:  h.Hub,
			uuid: uuid,
			room: roomSlug,
			conn: wsConn,
			send: make(chan []byte, 16),
		}

		if prev := h.Hub.Register(roomSlug, uuid, client); prev != nil {
			// Replace the previous connection for this uuid in this room.
			close(prev.send)
			_ = prev.conn.Close()
		}

		// Attach player to room and mark connected.
		room.UpsertPlayer(uuid, u.Name)
		room.SetConnected(uuid, true)

		// Send an initial snapshot tailored to this client.
		snap := room.Snapshot()
		msg := stateMessageForClient(snap, uuid)
		if b, mErr := json.Marshal(msg); mErr == nil {
			client.send <- b
		}

		// Start pumps.
		go client.writePump()
		client.readPump(room, uuid, h.Users)
		return nil
	}
}

func (c *Client) writePump() {
	defer func() {
		_ = c.conn.Close()
	}()

	// A simple heartbeat keeps proxies from killing idle connections.
	_ = c.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	pongHandler := func(string) error {
		_ = c.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
		return nil
	}
	c.conn.SetPongHandler(pongHandler)

	for b := range c.send {
		_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err := c.conn.WriteMessage(websocket.TextMessage, b); err != nil {
			return
		}
	}
}

func (c *Client) readPump(room *game.Room, uuid string, users *store.InMemoryUsers) {
	defer func() {
		// Publish updated room state (connected=false) for everyone before removing this
		// socket from the hub, so others always receive the full player list change.
		room.SetConnected(uuid, false)
		c.hub.unregister(c.room, uuid)
		_ = c.conn.Close()
	}()

	// We keep this minimal; app protocol is small.
	c.conn.SetReadLimit(1 << 20)
	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var msg inboundMessage
		if err := c.conn.ReadJSON(&msg); err != nil {
			return
		}

		switch msg.Type {
		case "vote":
			if msg.Card == "" {
				continue
			}
			_ = room.CastVote(uuid, msg.Card)
		case "cards_update":
			if strings.TrimSpace(msg.CardsText) == "" {
				continue
			}
			cards, err := game.ParseCardsTokens(msg.CardsText)
			if err != nil {
				continue
			}
			room.UpdateCards(cards)
		case "name_update":
			name, err := game.ValidatePlayerName(msg.Name)
			if err != nil {
				continue
			}
			_ = room.UpdatePlayerName(uuid, name)
			users.Upsert(uuid, name)
		case "reveal":
			_ = room.RevealNow()
		case "new_voting":
			_ = room.StartNewVoting()
		}
	}
}

