package server

import (
	"encoding/json"
	"sync"
)

const (
	maxClientsPerRoom   = 128
	maxVariablesPerRoom = 128
)

// Room holds the cloud variables and connected clients for a single
// project ID.
type Room struct {
	id string

	mu        sync.Mutex
	variables map[string]json.RawMessage
	order     []string // insertion order, for deterministic initial sync
	clients   map[*Client]struct{}
}

func newRoom(id string) *Room {
	return &Room{
		id:        id,
		variables: make(map[string]json.RawMessage),
		clients:   make(map[*Client]struct{}),
	}
}

func (r *Room) addClient(c *Client) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.clients) >= maxClientsPerRoom {
		return errRoomFull
	}
	r.clients[c] = struct{}{}
	return nil
}

func (r *Room) removeClient(c *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.clients, c)
}

func (r *Room) clientCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.clients)
}

// setVariable creates or updates a variable's value.
func (r *Room) setVariable(name string, value json.RawMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.variables[name]; !exists {
		if len(r.variables) >= maxVariablesPerRoom {
			return errTooManyVariables
		}
		r.order = append(r.order, name)
	}
	r.variables[name] = value
	return nil
}

func (r *Room) deleteVariable(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.variables[name]; !exists {
		return errVariableNotFound
	}
	delete(r.variables, name)
	r.removeFromOrderLocked(name)
	return nil
}

// renameVariable moves a variable's value to a new name, overwriting any
// variable already present at newName.
func (r *Room) renameVariable(oldName, newName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, exists := r.variables[oldName]
	if !exists {
		return errVariableNotFound
	}
	delete(r.variables, oldName)
	r.removeFromOrderLocked(oldName)
	if _, exists := r.variables[newName]; !exists {
		r.order = append(r.order, newName)
	}
	r.variables[newName] = value
	return nil
}

func (r *Room) removeFromOrderLocked(name string) {
	for i, n := range r.order {
		if n == name {
			r.order = append(r.order[:i], r.order[i+1:]...)
			return
		}
	}
}

// encodeAllVariables builds the newline-joined bulk set payload sent to a
// client immediately after it joins a room. Returns nil if there is nothing
// to send.
func (r *Room) encodeAllVariables() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.order) == 0 {
		return nil
	}
	messages := make([][]byte, 0, len(r.order))
	for _, name := range r.order {
		messages = append(messages, encodeSetMessage(name, r.variables[name]))
	}
	return joinMessages(messages)
}

// broadcastExcept sends payload to every client in the room other than
// sender.
func (r *Room) broadcastExcept(sender *Client, payload []byte) {
	r.mu.Lock()
	targets := make([]*Client, 0, len(r.clients))
	for c := range r.clients {
		if c != sender {
			targets = append(targets, c)
		}
	}
	r.mu.Unlock()

	for _, c := range targets {
		c.send(payload)
	}
}
