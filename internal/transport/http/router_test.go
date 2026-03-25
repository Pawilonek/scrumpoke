package httptransport

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Pawilonek/scrumpoke/internal/auth"
	"github.com/Pawilonek/scrumpoke/internal/game"
	"github.com/Pawilonek/scrumpoke/internal/store"
	"github.com/Pawilonek/scrumpoke/internal/transport/ws"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

func TestJoinDefaultsRoute(t *testing.T) {
	e := echo.New()
	e.Use(middleware.Static("static"))

	events := make(chan game.RoomSnapshot, 8)
	rooms := store.NewInMemoryRooms(game.CardsTokensDefault(), 0, events)
	users := store.NewInMemoryUsers()
	jwtIssuer, err := auth.NewJWTIssuer(24)
	if err != nil {
		t.Fatal(err)
	}
	h := ws.NewWSHandler(rooms, users, jwtIssuer, nil)

	RegisterRoutes(e, RouterDeps{Rooms: rooms, Users: users, JWT: jwtIssuer, WS: h})

	for _, path := range []string{"/api/join-defaults", "/join-defaults"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s: status %d body %s", path, rec.Code, rec.Body.String())
			}
			var body struct {
				Name string `json:"name"`
				Room string `json:"room"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if _, err := game.ValidatePlayerName(body.Name); err != nil {
				t.Fatalf("expected valid player name, got %q: %v", body.Name, err)
			}
			if err := game.ValidateRoomSlug(body.Room); err != nil {
				t.Fatalf("expected valid room slug %q: %v", body.Room, err)
			}
		})
	}
}
