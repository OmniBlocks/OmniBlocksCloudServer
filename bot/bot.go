package bot

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	CloudPrefixEmoji = "☁ "
	CloudPrefixText  = ":cloud: "
	DefaultCloudHost = "wss://clouddata.omniblocks.com"
	DefaultUsername  = "player"
)

// Config holds configuration options for the Bot.
type Config struct {
	ProjectID string
	Username  string
	CloudHost string
	UserAgent string
}

// Bot represents a lightweight cloud variable bot client (akin to TurboWarp's Mist).
type Bot struct {
	config Config

	mu           sync.RWMutex
	conn         *websocket.Conn
	connected    bool
	closed       bool
	variables    map[string]any
	rawVariables map[string]json.RawMessage
	sendQueue    []string

	// Event handlers
	onConnected    func()
	OnReconnecting func()
	onSet          func(name string, value any)
	onError        func(error)

	handlersMu sync.Mutex
	doneChan   chan struct{}
}

// New creates a new Bot client with the given config.
func New(config Config) (*Bot, error) {
	if config.ProjectID == "" {
		return nil, errors.New("projectId is required")
	}
	if config.Username == "" {
		config.Username = DefaultUsername
	}
	if config.CloudHost == "" {
		config.CloudHost = DefaultCloudHost
	}

	return &Bot{
		config:       config,
		variables:    make(map[string]any),
		rawVariables: make(map[string]json.RawMessage),
		doneChan:     make(chan struct{}),
	}, nil
}

// On registers event listeners ("connected", "reconnecting", "set", "error").
func (b *Bot) On(event string, handler any) {
	b.handlersMu.Lock()
	defer b.handlersMu.Unlock()

	switch event {
	case "connected":
		if fn, ok := handler.(func()); ok {
			b.onConnected = fn
		}
	case "reconnecting":
		if fn, ok := handler.(func()); ok {
			b.OnReconnecting = fn
		}
	case "set":
		if fn, ok := handler.(func(string, any)); ok {
			b.onSet = fn
		}
	case "error":
		if fn, ok := handler.(func(error)); ok {
			b.onError = fn
		}
	}
}

func normalizeName(name string) string {
	if !strings.HasPrefix(name, CloudPrefixEmoji) && !strings.HasPrefix(name, CloudPrefixText) {
		return CloudPrefixEmoji + name
	}
	return name
}

// Connect starts the WebSocket connection and automatic reconnection loop.
func (b *Bot) Connect() {
	go b.runConnectionLoop()
}

func (b *Bot) runConnectionLoop() {
	backoff := 100 * time.Millisecond
	maxBackoff := 5 * time.Second

	for {
		b.mu.RLock()
		if b.closed {
			b.mu.RUnlock()
			return
		}
		b.mu.RUnlock()

		if b.OnReconnecting != nil && b.connected {
			b.OnReconnecting()
		}

		conn, err := b.dial()
		if err != nil {
			time.Sleep(backoff)
			if backoff < maxBackoff {
				backoff *= 2
			}
			continue
		}

		backoff = 100 * time.Millisecond
		b.mu.Lock()
		b.conn = conn
		b.connected = true
		b.mu.Unlock()

		// Send Handshake
		if err := b.sendHandshake(); err != nil {
			_ = conn.Close()
			b.mu.Lock()
			b.connected = false
			b.conn = nil
			b.mu.Unlock()
			if b.onError != nil {
				b.onError(err)
			}
			if strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "username") {
				b.Close()
				return
			}
			time.Sleep(1 * time.Second)
			continue
		}

		if b.onConnected != nil {
			b.onConnected()
		}

		// Flush send queue
		b.flushQueue()

		// Read loop
		if err := b.readLoop(); err != nil {
			b.mu.Lock()
			b.connected = false
			b.conn = nil
			b.mu.Unlock()
		}

		b.mu.RLock()
		closed := b.closed
		b.mu.RUnlock()

		if closed {
			return
		}
	}
}

func (b *Bot) dial() (*websocket.Conn, error) {
	headers := http.Header{}
	if b.config.UserAgent != "" {
		headers.Set("User-Agent", b.config.UserAgent)
	}
	conn, _, err := websocket.DefaultDialer.Dial(b.config.CloudHost, headers)
	return conn, err
}

func (b *Bot) sendHandshake() error {
	msg := map[string]any{
		"method":     "handshake",
		"project_id": b.config.ProjectID,
		"user":       b.config.Username,
	}
	data, _ := json.Marshal(msg)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.conn == nil {
		return errors.New("not connected")
	}
	return b.conn.WriteMessage(websocket.TextMessage, data)
}

func (b *Bot) flushQueue() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.conn == nil {
		return
	}
	for _, item := range b.sendQueue {
		_ = b.conn.WriteMessage(websocket.TextMessage, []byte(item))
	}
	b.sendQueue = nil
}

func (b *Bot) readLoop() error {
	for {
		_, data, err := b.conn.ReadMessage()
		b.mu.RLock()
		closed := b.closed
		b.mu.RUnlock()
		if closed {
			return nil
		}
		if err != nil {
			return err
		}

		b.handleIncomingData(data)
	}
}

func (b *Bot) handleIncomingData(data []byte) {
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var msg struct {
			Method string          `json:"method"`
			Name   string          `json:"name"`
			Value  json.RawMessage `json:"value"`
		}

		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}

		if msg.Method == "set" {
			var val any
			if err := json.Unmarshal(msg.Value, &val); err == nil {
				b.mu.Lock()
				b.variables[msg.Name] = val
				b.rawVariables[msg.Name] = msg.Value
				b.mu.Unlock()

				b.handlersMu.Lock()
				onSet := b.onSet
				b.handlersMu.Unlock()

				if onSet != nil {
					onSet(msg.Name, val)
				}
			}
		}
	}
}

// Get returns the value of a cloud variable, or nil if unknown.
func (b *Bot) Get(name string) any {
	normalized := normalizeName(name)
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.variables[normalized]
}

// Set updates or creates a cloud variable.
func (b *Bot) Set(name string, value any) {
	normalized := normalizeName(name)
	
	valBytes, err := json.Marshal(value)
	if err != nil {
		return
	}

	b.mu.Lock()
	b.variables[normalized] = value
	b.rawVariables[normalized] = valBytes

	msg := map[string]any{
		"method": "set",
		"name":   normalized,
		"value":  json.RawMessage(valBytes),
	}
	msgBytes, _ := json.Marshal(msg)

	if b.conn != nil && b.connected {
		_ = b.conn.WriteMessage(websocket.TextMessage, msgBytes)
	} else {
		b.sendQueue = append(b.sendQueue, string(msgBytes))
	}
	b.mu.Unlock()
}

// Close permanently terminates the connection.
func (b *Bot) Close() {
	b.mu.Lock()
	b.closed = true
	if b.conn != nil {
		_ = b.conn.Close()
		b.conn = nil
	}
	b.connected = false
	b.mu.Unlock()

	close(b.doneChan)
}
