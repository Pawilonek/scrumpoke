package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	httptransport "github.com/Pawilonek/scrumpoke/internal/transport/http"
	"github.com/Pawilonek/scrumpoke/internal/auth"
	"github.com/Pawilonek/scrumpoke/internal/game"
	"github.com/Pawilonek/scrumpoke/internal/store"
	"github.com/Pawilonek/scrumpoke/internal/transport/ws"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

func main() {
	logger := log.NewWithOptions(os.Stdout, log.Options{
		ReportTimestamp: true,
		Level:           log.DebugLevel,
	})

	e := echo.New()
	e.Logger = slog.New(logger)

	e.Use(middleware.Recover())
	e.Use(middleware.RequestLogger())
	e.Use(middleware.ContextTimeout(60 * time.Second))
	e.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(20.0)))
	e.Use(middleware.Static("static"))

	events := make(chan game.RoomSnapshot, 128)
	rooms := store.NewInMemoryRooms(game.CardsTokensDefault(), 30*time.Second, events)
	users := store.NewInMemoryUsers()

	jwtIssuer, err := auth.NewJWTIssuer(30 * 24 * time.Hour)
	if err != nil {
		logger.Fatal("failed to create jwt issuer", "err", err)
	}

	hub := ws.NewHub(rooms, users, jwtIssuer, events)
	hub.Run()
	wsHandler := ws.NewWSHandler(rooms, users, jwtIssuer, hub)

	httptransport.RegisterRoutes(e, httptransport.RouterDeps{
		Rooms: rooms,
		Users: users,
		JWT:   jwtIssuer,
		WS:    wsHandler,
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: e,
		ErrorLog: logger.StandardLog(log.StandardLogOptions{
			ForceLevel: log.ErrorLevel,
		}),
	}

	logger.Info("starting a web server", "addr", server.Addr)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("shutting down the server", "err", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal("server shutdown failed", "err", err)
	}
}
