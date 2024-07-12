package server

import (
	"net/http"

	"fmt"
	"log"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"nhooyr.io/websocket"
)

func (s *Server) RegisterRoutes() http.Handler {
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.GET("/", s.HelloWorldHandler)

	e.GET("/health", s.healthHandler)

	e.GET("/ws/all", s.allWsHandler)
	e.GET("/ws/:table", s.singleTableWsHandler)
	e.GET("/ws/:table/:id", s.singleRowWsHandler)

	return e
}

func (s *Server) HelloWorldHandler(c echo.Context) error {
	resp := map[string]string{
		"message": "Hello World",
	}

	return c.JSON(http.StatusOK, resp)
}

func (s *Server) healthHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, s.db.Health())
}

func (s *Server) websocketHandler(c echo.Context) error {
	w := c.Response().Writer
	r := c.Request()
	socket, err := websocket.Accept(w, r, nil)

	if err != nil {
		log.Printf("could not open websocket: %v", err)
		_, _ = w.Write([]byte("could not open websocket"))
		w.WriteHeader(http.StatusInternalServerError)
		return nil
	}

	defer socket.Close(websocket.StatusGoingAway, "server closing websocket")

	ctx := r.Context()
	socketCtx := socket.CloseRead(ctx)

	for {
		payload := fmt.Sprintf("server timestamp: %d", time.Now().UnixNano())
		err := socket.Write(socketCtx, websocket.MessageText, []byte(payload))
		if err != nil {
			break
		}
		time.Sleep(time.Second * 2)
	}
	return nil
}

func (s *Server) allWsHandler(c echo.Context) error {
	w := c.Response().Writer
	r := c.Request()

	socket, err := websocket.Accept(w, r, nil)
	if err != nil {
		log.Printf("could not open websocket: %v", err)
		_, _ = w.Write([]byte("could not open websocket"))
		w.WriteHeader(http.StatusInternalServerError)
		return nil
	}
	defer socket.Close(websocket.StatusGoingAway, "server closing websocket")

	s.clients[socket] = &client{}

	ctx := r.Context()
	socketCtx := socket.CloseRead(ctx)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

_for:
	for {
		select {
		case <-socketCtx.Done():
			break _for
		case <-ticker.C:
			if err := socket.Ping(socketCtx); err != nil {
				log.Println("Failed to ping socket", err)
				break _for
			}
		}
	}

	return nil
}

func (s *Server) singleTableWsHandler(c echo.Context) error {
	w := c.Response().Writer
	r := c.Request()

	socket, err := websocket.Accept(w, r, nil)
	if err != nil {
		log.Printf("could not open websocket: %v", err)
		_, _ = w.Write([]byte("could not open websocket"))
		w.WriteHeader(http.StatusInternalServerError)
		return nil
	}
	defer socket.Close(websocket.StatusGoingAway, "server closing websocket")

	s.clients[socket] = &client{table: c.Param("table")}

	ctx := r.Context()
	socketCtx := socket.CloseRead(ctx)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

_for:
	for {
		select {
		case <-socketCtx.Done():
			break _for
		case <-ticker.C:
			if err := socket.Ping(socketCtx); err != nil {
				log.Println("Failed to ping socket", err)
				break _for
			}
		}
	}

	return nil
}

func (s *Server) singleRowWsHandler(c echo.Context) error {
	w := c.Response().Writer
	r := c.Request()

	socket, err := websocket.Accept(w, r, nil)
	if err != nil {
		log.Printf("could not open websocket: %v", err)
		_, _ = w.Write([]byte("could not open websocket"))
		w.WriteHeader(http.StatusInternalServerError)
		return nil
	}
	defer socket.Close(websocket.StatusGoingAway, "server closing websocket")

	s.clients[socket] = &client{table: c.Param("table"), id: c.Param("id")}

	ctx := r.Context()
	socketCtx := socket.CloseRead(ctx)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

_for:
	for {
		select {
		case <-socketCtx.Done():
			break _for
		case <-ticker.C:
			if err := socket.Ping(socketCtx); err != nil {
				log.Println("Failed to ping socket", err)
				break _for
			}
		}
	}

	return nil
}
