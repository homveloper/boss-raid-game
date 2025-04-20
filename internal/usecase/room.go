package usecase

import (
	"fmt"
	"tictactoe/internal/domain"

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
	// 디버깅 로그 추가
	fmt.Printf("Creating room with name: %s\n", name)

	// Generate a unique ID for the room
	roomID := uuid.New().String()
	fmt.Printf("Generated room ID: %s\n", roomID)

	// Create a new game for the room
	fmt.Printf("Creating game for room ID: %s\n", roomID)
	game, err := uc.gameUC.Create(roomID)
	if err != nil {
		fmt.Printf("Error creating game: %v\n", err)
		return nil, fmt.Errorf("failed to create game: %w", err)
	}

	// Get the game ID
	gameID := game.ID
	fmt.Printf("Game created with ID: %s\n", gameID)

	// Create a new room using NewRoom function
	room := domain.NewRoom(roomID, name)
	room.SetGameID(gameID)
	fmt.Printf("Room object created: %+v\n", room)

	// Save the room to the repository
	fmt.Printf("Saving room to repository\n")
	err = uc.roomRepo.Create(room)
	if err != nil {
		fmt.Printf("Error saving room: %v\n", err)
		return nil, fmt.Errorf("failed to create room: %w", err)
	}

	fmt.Printf("Room saved successfully\n")
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

	// Increment the player count in the room
	err = room.IncrementPlayerCount()
	if err != nil {
		return nil, fmt.Errorf("failed to increment player count: %w", err)
	}

	// Update the room in the repository
	err = uc.roomRepo.Update(room)
	if err != nil {
		return nil, fmt.Errorf("failed to update room: %w", err)
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
