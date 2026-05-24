package socket

import (
	socketio "github.com/googollee/go-socket.io"
	"log"
)

func SetupSocketRoutes(server *socketio.Server) {
	server.OnConnect("/", onConnect)
	server.OnDisconnect("/", onDisconnect)
	//server.OnEvent("/", "chat message", handlers.OnChatMessage)
	server.OnEvent("/", "join room", onJoinRoom)
	server.OnEvent("/", "leave room", onLeaveRoom)

	// 可以支持多个命名空间
	// chat := server.Of("/chat")
	// chat.OnEvent("/", "msg", handlers.OnChatMsg)
}

func onConnect(s socketio.Conn) error {
	s.SetContext("")
	log.Printf("✅ 客户端连接: %s\n", s.ID())
	return nil
}

func onDisconnect(s socketio.Conn, reason string) {
	log.Printf("❌ 客户端断开: %s, 原因: %s\n", s.ID(), reason)
}

func onJoinRoom(s socketio.Conn, room string) {
	s.Join(room)
	log.Printf("👥 用户 %s 加入房间: %s\n", s.ID(), room)
	s.Emit("joined", room)
}

func onLeaveRoom(s socketio.Conn, room string) {
	s.Leave(room)
	log.Printf("👋 用户 %s 离开房间: %s\n", s.ID(), room)
	s.Emit("left", room)
}
