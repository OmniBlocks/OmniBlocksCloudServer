package server

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const writeWait = 5 * time.Second

// Conn wraps a *websocket.Conn with a thread-safe write path
type Conn struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
}

// NewConn wraps conn for safe concurrent use.
func NewConn(conn *websocket.Conn) *Conn {
	return &Conn{conn: conn}
}

// Send writes a text frame to the connection.
func (c *Conn) Send(data []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// CloseWithCode sends a WebSocket close frame with the given status code
// and then closes the underlying connection.
func (c *Conn) CloseWithCode(code int, reason string) error {
	c.writeMu.Lock()
	msg := websocket.FormatCloseMessage(code, reason)
	_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	err := c.conn.WriteControl(websocket.CloseMessage, msg, time.Now().Add(writeWait))
	c.writeMu.Unlock()
	_ = c.conn.Close()
	return err
}

// Close closes the underlying connection immediately, without a close
// handshake.
func (c *Conn) Close() error {
	return c.conn.Close()
}

// ReadMessage reads the next message from the underlying connection.
func (c *Conn) ReadMessage() (int, []byte, error) {
	return c.conn.ReadMessage()
}

// Client wraps a single WebSocket connection for the server, layering
// room/session state on top of Conn.
type Client struct {
	*Conn
	logger *Logger

	ip string

	room     *Room
	username string
}

func newClient(conn *websocket.Conn, ip string, logger *Logger) *Client {
	return &Client{Conn: NewConn(conn), ip: ip, logger: logger}
}

// send writes a text frame to the client. Errors are logged and swallowed.
func (c *Client) send(data []byte) {
	if err := c.Conn.Send(data); err != nil {
		c.logger.Connection("write to %s failed: %v", c.ip, err)
	}
}

// closeWithCode sends a WebSocket close frame with the given status code
// and then closes the underlying connection.
func (c *Client) closeWithCode(code int, reason string) {
	if err := c.Conn.CloseWithCode(code, reason); err != nil {
		c.logger.Connection("close handshake with %s failed: %v", c.ip, err)
	}
}
