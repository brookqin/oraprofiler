package main

import (
	"encoding/json"
	"errors"
	"log"
	"oraprofiler/server/database"
	"time"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/simplifiedchinese"
)

type Server struct {
	// Registered clients.
	clients map[*Client]bool

	// Inbound messages from the clients.
	broadcast chan []byte

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client
}

var (
	db       database.Database
	sessions []database.Session
)

func newServer(connStr string) (*Server, error) {
	db = database.NewOracleDatabase(connStr)
	if db == nil {
		return nil, errors.New("newServer: db is nil")
	}

	sessions = []database.Session{}

	if err := db.Open(); err != nil {
		return nil, err
	}

	return &Server{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}, nil
}

func (s *Server) broadcastSession() {
	if len(s.clients) == 0 {
		return
	}

	ss := db.QuerySessions(ProgramName)
	if !sessionSliceCompare(ss, sessions) {
		sessions = ss
		if data, err := json.Marshal(&Command{Command: "sessions", Data: ss}); err == nil {
			s.broadcast <- data
		}
	}
}

func sessionSliceCompare(s1, s2 []database.Session) bool {
	if len(s1) != len(s2) {
		return false
	}

	for i := range s1 {
		if s1[i].Sid != s2[i].Sid || s1[i].Serial != s2[i].Serial {
			return false
		}
	}

	return true
}

func (s *Server) GetEncoding(service string) encoding.Encoding {
	enc := db.GetEncoding(service)
	switch enc {
	case "ZHS16GBK":
		return simplifiedchinese.GBK
	default:
		return nil
	}
}

func (s *Server) EnableTrace(sid, serial int) bool {
	return db.Tracing(sid, serial)
}

func (s *Server) DisableTrace(sid, serial int) bool {
	return db.UnTracing(sid, serial)
}

func (s *Server) GetTraceFile(sid int) string {
	return db.GetTraceFile(sid)
}

func (h *Server) run() {
	ticker := time.NewTicker(2 * time.Second)
	defer func() {
		ticker.Stop()
		db.Close()
	}()

	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			log.Println("new connection", client.Id)
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				log.Println("connection closed", client.Id)
			}
		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		case <-ticker.C:
			go h.broadcastSession()
		}
	}
}
