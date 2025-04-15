package memory

import (
	"errors"
	"sync"
	"tictactoe/internal/domain"
)

// RoomRepository is an in-memory implementation of domain.RoomRepository
type RoomRepository struct {
	rooms map[string]*domain.Room
	mu    sync.RWMutex
}

// NewRoomRepository creates a new in-memory room repository
func NewRoomRepository() *RoomRepository {
	return &RoomRepository{
		rooms: make(map[string]*domain.Room),
	}
}

// Create creates a new room
func (r *RoomRepository) Create(room *domain.Room) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.rooms[room.ID]; exists {
		return errors.New("room already exists")
	}

	r.rooms[room.ID] = room
	return nil
}

// Get retrieves a room by ID
func (r *RoomRepository) Get(id string) (*domain.Room, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	room, exists := r.rooms[id]
	if !exists {
		return nil, errors.New("room not found")
	}

	return room, nil
}

// Update updates a room
func (r *RoomRepository) Update(room *domain.Room) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.rooms[room.ID]; !exists {
		return errors.New("room not found")
	}

	r.rooms[room.ID] = room
	return nil
}

// Delete deletes a room
func (r *RoomRepository) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.rooms[id]; !exists {
		return errors.New("room not found")
	}

	delete(r.rooms, id)
	return nil
}

// List returns all rooms
func (r *RoomRepository) List() ([]*domain.Room, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rooms := make([]*domain.Room, 0, len(r.rooms))
	for _, room := range r.rooms {
		rooms = append(rooms, room)
	}

	return rooms, nil
}
