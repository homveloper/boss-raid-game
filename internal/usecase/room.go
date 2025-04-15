package usecase

import (
	"fmt"
	"tictactoe/internal/domain"
	"time"

	"github.com/google/uuid"
)

// RoomUseCase implements domain.RoomUseCase
type RoomUseCase struct {
	roomRepo domain.RoomRepository
	gameUC   domain.GameUseCase
}

// NewRoomUseCase creates a new room use case
func NewRoomUseCase(roomRepo domain.RoomRepository, gameUC domain.GameUseCase) *RoomUseCase {
	return &RoomUseCase{
		roomRepo: roomRepo,
		gameUC:   gameUC,
	}
}

// Create creates a new room
func (uc *RoomUseCase) Create(name string) (*domain.Room, error) {
	// Generate a unique ID for the room
	roomID := uuid.New().String()

	// Create a new game for the room
	game, err := uc.gameUC.Create(roomID)
	if err != nil {
		return nil, fmt.Errorf("failed to create game: %w", err)
	}

	// Get the game ID
	gameID := game.ID

	// Create a new room
	room := &domain.Room{
		ID:        roomID,
		Name:      name,
		GameID:    gameID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Save the room to the repository
	err = uc.roomRepo.Create(room)
	if err != nil {
		return nil, fmt.Errorf("failed to create room: %w", err)
	}

	return room, nil
}

// Get retrieves a room by ID
func (uc *RoomUseCase) Get(id string) (*domain.Room, error) {
	return uc.roomRepo.Get(id)
}

// List returns all rooms
func (uc *RoomUseCase) List() ([]*domain.Room, error) {
	return uc.roomRepo.List()
}

// Delete deletes a room
func (uc *RoomUseCase) Delete(id string) error {
	// Get the room
	_, err := uc.roomRepo.Get(id)
	if err != nil {
		return fmt.Errorf("failed to get room: %w", err)
	}

	// Delete the room from the repository
	err = uc.roomRepo.Delete(id)
	if err != nil {
		return fmt.Errorf("failed to delete room: %w", err)
	}

	return nil
}

// Join adds a player to a room's game
func (uc *RoomUseCase) Join(roomID string, playerID string, playerName string) (*domain.Game, error) {
	// Get the room
	room, err := uc.roomRepo.Get(roomID)
	if err != nil {
		return nil, fmt.Errorf("failed to get room: %w", err)
	}

	// Join the game
	game, err := uc.gameUC.Join(room.GameID, playerID, playerName)
	if err != nil {
		return nil, fmt.Errorf("failed to join game: %w", err)
	}

	return game, nil
}

// Leave removes a player from a room's game
func (uc *RoomUseCase) Leave(roomID string, playerID string) error {
	// Get the room
	_, err := uc.roomRepo.Get(roomID)
	if err != nil {
		return fmt.Errorf("failed to get room: %w", err)
	}

	// TODO: Implement leave game functionality

	return nil
}
