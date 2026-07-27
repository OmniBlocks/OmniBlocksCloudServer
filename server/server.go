// Package server: An implementation of the Scratch 3 Websocket Protocol.
package server

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

const maxMessageSize = 1024 * 1024 // 1 MB is enough for any sane project

// Server is an http.Handler that upgrades requests to WebSocket connections
// and speaks the cloud-server protocol to them.
type Server struct {
	rooms    *RoomList
	upgrader websocket.Upgrader

	// Logf is called with diagnostic messages. Defaults to log.Printf.
	Logf func(format string, args ...any)
}

// New creates a Server ready to be used as an http.Handler.
func New() *Server {
	return &Server{
		rooms: newRoomList(),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
		Logf: log.Printf,
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.Logf("upgrade failed: %v", err)
		return
	}
	s.handleConnection(conn, r)
}

func (s *Server) handleConnection(conn *websocket.Conn, r *http.Request) {
	conn.SetReadLimit(maxMessageSize)

	client := newClient(conn, r.RemoteAddr)

	defer func() {
		if client.room != nil {
			client.room.removeClient(client)
			s.rooms.removeIfEmpty(client.room.id)
		}
		_ = conn.Close()
	}()

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if msgType != websocket.TextMessage {
			// Binary frames are not part of the protocol.
			// This client is clearly insane, so reject them.
			continue
		}

		if err := s.processMessage(client, data); err != nil {
			code := CodeGenericError
			var ce *connError
			if errors.As(err, &ce) {
				code = ce.code
			}
			s.Logf("closing %s: %v", client.ip, err)
			client.closeWithCode(code, err.Error())
			return
		}
	}
}

func (s *Server) processMessage(client *Client, data []byte) error {
	var msg inMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return newConnError(CodeGenericError, "invalid JSON message")
	}

	switch msg.Method {
	case "handshake":
		return s.handleHandshake(client, &msg)
	case "set", "create":
		return s.handleSet(client, &msg)
	case "rename":
		return s.handleRename(client, &msg)
	case "delete":
		return s.handleDelete(client, &msg)
	case "":
		return newConnError(CodeGenericError, "missing method")
	default:
		return newConnError(CodeGenericError, "unknown method: "+msg.Method)
	}
}

func (s *Server) handleHandshake(client *Client, msg *inMessage) error {
	if client.room != nil {
		return newConnError(CodeGenericError, "handshake already performed")
	}

	projectID := projectIDString(msg.ProjectID)
	if !isValidRoomID(projectID) {
		return newConnError(CodeGenericError, "invalid project_id")
	}
	if !isValidUsername(msg.User) {
		return newConnError(CodeUsernameError, "invalid username")
	}

	room, err := s.rooms.getOrCreate(projectID)
	if err != nil {
		return newConnError(CodeOverloaded, err.Error())
	}
	if err := room.addClient(client); err != nil {
		return newConnError(CodeOverloaded, err.Error())
	}

	client.room = room
	client.username = msg.User

	if initial := room.encodeAllVariables(); initial != nil {
		client.send(initial)
	}
	return nil
}

func (s *Server) handleSet(client *Client, msg *inMessage) error {
	if client.room == nil {
		return newConnError(CodeGenericError, "handshake required before set")
	}

	if !isValidVariableValue(msg.Value) {
		// invalid values are ignored.
		return nil
	}

	if err := client.room.setVariable(msg.Name, msg.Value); err != nil {
		return newConnError(CodeGenericError, err.Error())
	}

	payload := encodeSetMessage(msg.Name, msg.Value)
	client.room.broadcastExcept(client, payload)
	return nil
}

func (s *Server) handleRename(client *Client, msg *inMessage) error {
	if client.room == nil {
		return newConnError(CodeGenericError, "handshake required before rename")
	}
	if !isValidVariableName(msg.NewName) {
		return newConnError(CodeGenericError, "invalid new_name")
	}
	if err := client.room.renameVariable(msg.Name, msg.NewName); err != nil {
		return newConnError(CodeGenericError, err.Error())
	}
	return nil
}

func (s *Server) handleDelete(client *Client, msg *inMessage) error {
	if client.room == nil {
		return newConnError(CodeGenericError, "handshake required before delete")
	}
	if err := client.room.deleteVariable(msg.Name); err != nil {
		return newConnError(CodeGenericError, err.Error())
	}
	return nil
}
