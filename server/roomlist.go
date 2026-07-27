package server

import "sync"

// maxRooms bounds how many rooms (including empty ones) may exist at once
const maxRooms = 16384

// RoomList tracks all active rooms, keyed by project ID.
type RoomList struct {
	mu    sync.Mutex
	rooms map[string]*Room
}

func newRoomList() *RoomList {
	return &RoomList{rooms: make(map[string]*Room)}
}

func (rl *RoomList) getOrCreate(id string) (*Room, error) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if room, ok := rl.rooms[id]; ok {
		return room, nil
	}
	if len(rl.rooms) >= maxRooms {
		return nil, errTooManyRooms
	}
	room := newRoom(id)
	rl.rooms[id] = room
	return room, nil
}

// removeIfEmpty deletes the room with the given id if it currently has no
// connected clients.
func (rl *RoomList) removeIfEmpty(id string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	room, ok := rl.rooms[id]
	if !ok {
		return
	}
	if room.clientCount() == 0 {
		delete(rl.rooms, id)
	}
}
