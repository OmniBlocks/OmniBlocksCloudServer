package server

import "sync"

// maxRooms bounds how many rooms (including empty ones) may exist at once
const maxRooms = 16384

// RoomList tracks all active rooms, keyed by project ID.
type RoomList struct {
	mu     sync.Mutex
	rooms  map[string]*Room
	logger *Logger
}

func newRoomList(logger *Logger) *RoomList {
	return &RoomList{rooms: make(map[string]*Room), logger: logger}
}

func (rl *RoomList) getOrCreate(id string) (*Room, error) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if room, ok := rl.rooms[id]; ok {
		return room, nil
	}
	if len(rl.rooms) >= maxRooms {
		rl.logger.Rooms("cannot create room %q: room limit (%d) reached", id, maxRooms)
		return nil, errTooManyRooms
	}
	room := newRoom(id, rl.logger)
	rl.rooms[id] = room
	rl.logger.Rooms("created room %q (%d active room(s))", id, len(rl.rooms))
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
		rl.logger.Rooms("removed empty room %q (%d active room(s))", id, len(rl.rooms))
	}
}
