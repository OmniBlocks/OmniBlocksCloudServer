// Package server: An implementation of the Scratch 3 Websocket Protocol.
package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/websocket"
)

const maxMessageSize = 1024 * 1024 // 1 MB is enough for any sane project

// Server is an http.Handler that upgrades requests to WebSocket connections
// and speaks the cloud-server protocol to them.
type Server struct {
	rooms    *RoomList
	upgrader websocket.Upgrader
	logger   *Logger
}

// New creates a Server ready to be used as an http.Handler. A nil logger
// disables all diagnostic logging.
func New(logger *Logger) *Server {
	if logger == nil {
		logger = NewDiscardLogger()
	}
	return &Server{
		rooms: newRoomList(logger),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
		logger: logger,
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Connection("upgrade failed for %s: %v", r.RemoteAddr, err)
		return
	}
	s.logger.Connection("accepted connection from %s", r.RemoteAddr)
	s.handleConnection(conn, r)
}

func (s *Server) handleConnection(conn *websocket.Conn, r *http.Request) {
	conn.SetReadLimit(maxMessageSize)

	client := newClient(conn, r.RemoteAddr, s.logger)

	defer func() {
		if client.room != nil {
			client.room.removeClient(client)
			s.rooms.removeIfEmpty(client.room.id)
		}
		s.logger.Connection("disconnected %s", client.ip)
		_ = conn.Close()
	}()

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			s.logger.Connection("read error from %s: %v", client.ip, err)
			return
		}
		if msgType != websocket.TextMessage {
			// Binary frames are not part of the protocol.
			// This client is clearly insane, so reject them.
			s.logger.Messages("ignoring non-text frame from %s", client.ip)
			continue
		}

		if err := s.processMessage(client, data); err != nil {
			code := CodeGenericError
			var ce *connError
			if errors.As(err, &ce) {
				code = ce.code
			}
			s.logger.Errors("closing %s: %v (code %d)", client.ip, err, code)
			client.closeWithCode(code, err.Error())
			return
		}
	}
}

func (s *Server) processMessage(client *Client, data []byte) error {
	var msg inMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		s.logger.Errors("invalid JSON from %s: %v", client.ip, err)
		return newConnError(CodeGenericError, "invalid JSON message")
	}

	s.logger.Messages("received %q from %s", msg.Method, client.ip)

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
		s.logger.Errors("missing method from %s", client.ip)
		return newConnError(CodeGenericError, "missing method")
	default:
		s.logger.Errors("unknown method %q from %s", msg.Method, client.ip)
		return newConnError(CodeGenericError, "unknown method: "+msg.Method)
	}
}

func (s *Server) handleHandshake(client *Client, msg *inMessage) error {
	if client.room != nil {
		return newConnError(CodeGenericError, "handshake already performed")
	}

	projectID := projectIDString(msg.ProjectID)
	if !isValidRoomID(projectID) {
		s.logger.Handshake("rejected handshake from %s: invalid project_id", client.ip)
		return newConnError(CodeGenericError, "invalid project_id")
	}
	if !isValidUsername(msg.User) {
		s.logger.Handshake("rejected handshake from %s: invalid username %q", client.ip, msg.User)
		return newConnError(CodeUsernameError, "invalid username")
	}

	room, err := s.rooms.getOrCreate(projectID)
	if err != nil {
		s.logger.Handshake("rejected handshake from %s: %v", client.ip, err)
		return newConnError(CodeOverloaded, err.Error())
	}
	if err := room.addClient(client); err != nil {
		s.logger.Handshake("rejected handshake from %s: %v", client.ip, err)
		return newConnError(CodeOverloaded, err.Error())
	}

	client.room = room
	client.username = msg.User

	s.logger.Handshake("%s joined room %q as %q", client.ip, projectID, msg.User)

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
		s.logger.Variables("ignoring invalid value for %q from %s in room %q", msg.Name, client.ip, client.room.id)
		return nil
	}

	if err := client.room.setVariable(msg.Name, msg.Value); err != nil {
		s.logger.Errors("set %q failed in room %q: %v", msg.Name, client.room.id, err)
		return newConnError(CodeGenericError, err.Error())
	}

	s.logger.Variables("%s set %q=%s in room %q", client.ip, msg.Name, msg.Value, client.room.id)

	payload := encodeSetMessage(msg.Name, msg.Value)
	client.room.broadcastExcept(client, payload)
	return nil
}

func (s *Server) handleRename(client *Client, msg *inMessage) error {
	if client.room == nil {
		return newConnError(CodeGenericError, "handshake required before rename")
	}
	if !isValidVariableName(msg.NewName) {
		s.logger.Errors("rename to invalid name %q from %s in room %q", msg.NewName, client.ip, client.room.id)
		return newConnError(CodeGenericError, "invalid new_name")
	}
	if err := client.room.renameVariable(msg.Name, msg.NewName); err != nil {
		s.logger.Errors("rename %q -> %q failed in room %q: %v", msg.Name, msg.NewName, client.room.id, err)
		return newConnError(CodeGenericError, err.Error())
	}
	s.logger.Variables("%s renamed %q -> %q in room %q", client.ip, msg.Name, msg.NewName, client.room.id)
	return nil
}

func (s *Server) handleDelete(client *Client, msg *inMessage) error {
	if client.room == nil {
		return newConnError(CodeGenericError, "handshake required before delete")
	}
	if err := client.room.deleteVariable(msg.Name); err != nil {
		s.logger.Errors("delete %q failed in room %q: %v", msg.Name, client.room.id, err)
		return newConnError(CodeGenericError, err.Error())
	}
	s.logger.Variables("%s deleted %q in room %q", client.ip, msg.Name, client.room.id)
	return nil
}
