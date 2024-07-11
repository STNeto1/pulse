package server

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/joho/godotenv/autoload"

	"pulse/internal/database"
)

type Server struct {
	port int

	db database.Service

	ch chan database.DBNotification
}

func NewServer() *http.Server {
	port, _ := strconv.Atoi(os.Getenv("PORT"))

	ch := make(chan database.DBNotification)

	NewServer := &Server{
		port: port,

		db: database.New(),

		ch: ch,
	}

	if err := NewServer.db.SyncTables(); err != nil {
		log.Fatalf("Failed to sync tables, can't continue %s\n", err.Error())
	}

	go NewServer.db.Watch(NewServer.ch)

	// Declare Server config
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", NewServer.port),
		Handler:      NewServer.RegisterRoutes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return server
}
