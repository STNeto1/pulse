package server

import (
	"context"
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
}

type Server struct {
	port int

	db database.Service

	clients    map[*websocket.Conn]*client
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	broadcast  chan []byte
}

func NewServer() *http.Server {
	port, _ := strconv.Atoi(os.Getenv("PORT"))

	// ch := make(chan database.DBNotification)

	NewServer := &Server{
		port: port,

		db: database.New(),

		clients:    make(map[*websocket.Conn]*client),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
		broadcast:  make(chan []byte),
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
		case conn := <-s.register:
			s.clients[conn] = &client{}
		case conn := <-s.unregister:
			delete(s.clients, conn)
		case msg := <-s.broadcast:
			for connection, cli := range s.clients {
				go func(conn *websocket.Conn, c *client) {
					c.mut.Lock()
					defer c.mut.Unlock()

					if c.isClosing {
						return
					}

					if err := conn.Write(context.Background(), websocket.MessageText, msg); err != nil {
						c.isClosing = true

						log.Println("write error:", err)
						// connection.WriteMessage(websocket.CloseMessage, []byte{})
						// connection.Close()
						// unregister <- connection
					}
				}(connection, cli)
			}
		}
	}
}
