package server

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const writeWait = 5 * time.Second

// Client wraps a single WebSocket connection.
type Client struct {
	conn    *websocket.Conn
	writeMu sync.Mutex

	ip string

	room     *Room
	username string
}

func newClient(conn *websocket.Conn, ip string) *Client {
	return &Client{conn: conn, ip: ip}
}

// send writes a text frame to the client. Errors are swallowed.
func (c *Client) send(data []byte) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	_ = c.conn.WriteMessage(websocket.TextMessage, data)
}

// closeWithCode sends a WebSocket close frame with the given status code
// and then closes the underlying connection.
func (c *Client) closeWithCode(code int, reason string) {
	c.writeMu.Lock()
	msg := websocket.FormatCloseMessage(code, reason)
	_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	_ = c.conn.WriteControl(websocket.CloseMessage, msg, time.Now().Add(writeWait))
	c.writeMu.Unlock()
	_ = c.conn.Close()
}
