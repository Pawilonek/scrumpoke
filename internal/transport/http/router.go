package httptransport

import (
	"net/http"
	"strings"

	"github.com/Pawilonek/scrumpoke/internal/auth"
	"github.com/Pawilonek/scrumpoke/internal/game"
	"github.com/Pawilonek/scrumpoke/internal/store"
	"github.com/Pawilonek/scrumpoke/internal/transport/ws"
	"github.com/labstack/echo/v5"
)

type RouterDeps struct {
	Rooms *store.InMemoryRooms
	Users *store.InMemoryUsers
	JWT   *auth.JWTIssuer
	WS    *ws.WSHandler
}

func RegisterRoutes(e *echo.Echo, deps RouterDeps) {
	e.GET("/", func(c *echo.Context) error {
		return c.File("static/index.html")
	})

	e.GET("/room/:room", func(c *echo.Context) error {
		return c.File("static/index.html")
	})

	e.POST("/join", func(c *echo.Context) error {
		var req joinRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid json"})
		}

		name := strings.TrimSpace(req.Name)
		if _, err := game.ValidatePlayerName(name); err != nil {
			name = game.RandomPlayerName()
		} else {
			name, _ = game.ValidatePlayerName(name)
		}

		roomSlug := strings.ToLower(strings.TrimSpace(req.Room))
		if err := game.ValidateRoomSlug(roomSlug); err != nil {
			roomSlug = game.RandomRoomSlug()
		}

		uuid := strings.TrimSpace(req.UUID)
		if uuid == "" || !isUUIDLike(uuid) {
			uuid = game.RandomUUID()
		}

		deps.Rooms.GetOrCreate(roomSlug).UpsertPlayer(uuid, name)
		deps.Users.Upsert(uuid, name)

		token, _, err := deps.JWT.Issue(uuid)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to issue token"})
		}

		return c.JSON(http.StatusOK, joinResponse{
			Room:  roomSlug,
			UUID:  uuid,
			Token: token,
		})
	})

	// WebSocket upgrade. Token is provided as a query parameter: ?token=<jwt>.
	e.GET("/room/:room/ws", deps.WS.ServeHTTP())
}

type joinRequest struct {
	Room string `json:"room"`
	Name string `json:"name"`
	UUID string `json:"uuid,omitempty"`
}

type joinResponse struct {
	Room  string `json:"room"`
	UUID  string `json:"uuid"`
	Token string `json:"token"`
}

// isUUIDLike is a minimal UUID validation (UUIDv4 style: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx).
func isUUIDLike(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) != 36 {
		return false
	}
	parts := strings.Split(s, "-")
	if len(parts) != 5 {
		return false
	}
	lens := []int{8, 4, 4, 4, 12}
	for i, p := range parts {
		if len(p) != lens[i] {
			return false
		}
		for _, r := range p {
			if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F') {
				return false
			}
		}
	}
	return true
}

