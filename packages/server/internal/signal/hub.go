package signal

import (
	"log"
	"sync"

	socketio "github.com/googollee/go-socket.io"
)

type Hub struct {
	server  *socketio.Server
	rooms   map[string]map[string]bool
	mu      sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		rooms: make(map[string]map[string]bool),
	}
}

func (h *Hub) SetServer(server *socketio.Server) {
	h.server = server
}

func (h *Hub) OnConnect(s socketio.Conn) error {
	s.SetContext("")
	log.Printf("client connected: %s", s.ID())
	return nil
}

func (h *Hub) OnDisconnect(s socketio.Conn, reason string) {
	h.mu.Lock()
	for roomID, members := range h.rooms {
		if members[s.ID()] {
			delete(members, s.ID())
			if len(members) == 0 {
				delete(h.rooms, roomID)
			}
		}
	}
	h.mu.Unlock()
	log.Printf("client disconnected: %s, reason: %s", s.ID(), reason)
}

func (h *Hub) OnJoinRoom(s socketio.Conn, room string) {
	s.Join(room)

	h.mu.Lock()
	if h.rooms[room] == nil {
		h.rooms[room] = make(map[string]bool)
	}
	h.rooms[room][s.ID()] = true
	h.mu.Unlock()

	log.Printf("user %s joined room: %s", s.ID(), room)
	s.Emit("room:joined", room)
}

func (h *Hub) OnLeaveRoom(s socketio.Conn, room string) {
	s.Leave(room)

	h.mu.Lock()
	if h.rooms[room] != nil {
		delete(h.rooms[room], s.ID())
		if len(h.rooms[room]) == 0 {
			delete(h.rooms, room)
		}
	}
	h.mu.Unlock()

	log.Printf("user %s left room: %s", s.ID(), room)
	s.Emit("room:left", room)
}

func (h *Hub) SetupRoutes(server *socketio.Server) {
	h.SetServer(server)

	server.OnConnect("/", h.OnConnect)
	server.OnDisconnect("/", h.OnDisconnect)
	server.OnEvent("/", "room:join", h.OnJoinRoom)
	server.OnEvent("/", "room:leave", h.OnLeaveRoom)
}

func (h *Hub) GetRoomMembers(room string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	members := h.rooms[room]
	result := make([]string, 0, len(members))
	for id := range members {
		result = append(result, id)
	}
	return result
}