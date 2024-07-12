package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	_ "github.com/joho/godotenv/autoload"
	"nhooyr.io/websocket"

	"pulse/internal/database"
)

type client struct {
	isClosing bool
	mut       sync.Mutex
	table     string
	id        string
}

type Server struct {
	port int

	db database.Service

	clients   map[*websocket.Conn]*client
	broadcast chan database.DBNotification
}

func NewServer() *http.Server {
	port, _ := strconv.Atoi(os.Getenv("PORT"))

	// ch := make(chan database.DBNotification)

	NewServer := &Server{
		port: port,

		db: database.New(),

		clients:   make(map[*websocket.Conn]*client),
		broadcast: make(chan database.DBNotification),
	}

	if err := NewServer.db.SyncTables(); err != nil {
		log.Fatalf("Failed to sync tables, can't continue %s\n", err.Error())
	}

	go NewServer.db.Watch(NewServer.broadcast)
	go NewServer.Hub()

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

func (s *Server) Hub() {
	for {
		select {
		case msg := <-s.broadcast:
			for connection, cli := range s.clients {
				go func(conn *websocket.Conn, c *client) {
					c.mut.Lock()
					defer c.mut.Unlock()

					if c.isClosing {
						return
					}

					if c.table != "" && c.table != msg.Table {
						return
					}

					if c.id != "" && c.id != msg.ID {
						return
					}

					jsonData, _ := json.Marshal(msg)

					if err := conn.Write(context.Background(), websocket.MessageText, jsonData); err != nil {
						c.isClosing = true

						log.Println("write error:", err)

						connection.Write(context.Background(), websocket.MessageText, []byte("closing"))
						connection.Close(websocket.StatusGoingAway, "")
						delete(s.clients, connection)
					}
				}(connection, cli)
			}
		}
	}
}
