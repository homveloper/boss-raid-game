package domain

import (
	"errors"
	"time"
)

// RoomState represents the state of a room
type RoomState string

const (
	// RoomStateWaiting means the room is waiting for players
	RoomStateWaiting RoomState = "waiting"
	// RoomStateReady means all players are ready
	RoomStateReady RoomState = "ready"
	// RoomStatePlaying means the game is in progress
	RoomStatePlaying RoomState = "playing"
	// RoomStateCompleted means the game is completed
	RoomStateCompleted RoomState = "completed"
)

// Room represents a game room
type Room struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	GameID      string    `json:"gameId"`
	State       RoomState `json:"state"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	MaxPlayers  int       `json:"maxPlayers"`
	PlayerCount int       `json:"playerCount"`
}

// NewRoom creates a new room
func NewRoom(id, name string) *Room {
	return &Room{
		ID:          id,
		Name:        name,
		State:       RoomStateWaiting,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		MaxPlayers:  3, // 3 players for boss raid
		PlayerCount: 0,
	}
}

// SetGameID sets the game ID for the room
func (r *Room) SetGameID(gameID string) {
	r.GameID = gameID
	r.UpdatedAt = time.Now()
}

// UpdateState updates the room state
func (r *Room) UpdateState(state RoomState) {
	r.State = state
	r.UpdatedAt = time.Now()
}

// IncrementPlayerCount increments the player count
func (r *Room) IncrementPlayerCount() error {
	if r.PlayerCount >= r.MaxPlayers {
		return errors.New("room is full")
	}
	r.PlayerCount++
	r.UpdatedAt = time.Now()
	return nil
}

// DecrementPlayerCount decrements the player count
func (r *Room) DecrementPlayerCount() {
	if r.PlayerCount > 0 {
		r.PlayerCount--
		r.UpdatedAt = time.Now()
	}
}

// IsFull checks if the room is full
func (r *Room) IsFull() bool {
	return r.PlayerCount >= r.MaxPlayers
}

// RoomRepository defines the methods for room data access
type RoomRepository interface {
	Create(room *Room) error
	Get(id string) (*Room, error)
	Update(room *Room) error
	Delete(id string) error
	List() ([]*Room, error)
}

// RoomUseCase defines the methods for room business logic
type RoomUseCase interface {
	Create(name string) (*Room, error)
	Get(id string) (*Room, error)
	List() ([]*Room, error)
	Delete(id string) error
	Join(roomID string, playerID string, playerName string) (*Game, error)
	Leave(roomID string, playerID string) error
}
