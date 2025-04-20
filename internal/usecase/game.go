package usecase

import (
	"fmt"
	"math/rand"
	"sync"
	"tictactoe/internal/domain"
	"time"

	"github.com/google/uuid"
)

// GameUseCase implements domain.GameUseCase
type GameUseCase struct {
	gameRepo domain.GameRepository
	mu       sync.RWMutex
}

// NewGameUseCase creates a new game use case
func NewGameUseCase(gameRepo domain.GameRepository) *GameUseCase {
	return &GameUseCase{
		gameRepo: gameRepo,
	}
}

// Create creates a new game
func (uc *GameUseCase) Create(roomID string) (*domain.Game, error) {
	// 디버깅 로그 추가
	fmt.Printf("Creating game for room ID: %s\n", roomID)

	// Generate a unique ID for the game
	gameID := uuid.New().String()
	fmt.Printf("Generated game ID: %s\n", gameID)

	// Create a new game
	game := domain.NewGame(gameID, roomID)
	fmt.Printf("Game object created: %+v\n", game)

	// Initialize the boss
	game.Boss = domain.NewRandomBoss()
	fmt.Printf("Boss initialized: %+v\n", game.Boss)

	// Save the game to the repository
	fmt.Printf("Saving game to repository\n")
	err := uc.gameRepo.Create(game)
	if err != nil {
		fmt.Printf("Error saving game: %v\n", err)
		return nil, fmt.Errorf("failed to create game: %w", err)
	}

	fmt.Printf("Game saved successfully\n")
	return game, nil
}

// Get retrieves a game by ID
func (uc *GameUseCase) Get(id string) (*domain.Game, error) {
	return uc.gameRepo.Get(id)
}

// List returns all games
func (uc *GameUseCase) List() ([]*domain.Game, error) {
	return uc.gameRepo.List()
}

// Join adds a player to a game
func (uc *GameUseCase) Join(gameID, playerID, playerName string) (*domain.Game, error) {
	// Get the game from the repository
	game, err := uc.gameRepo.Get(gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to get game: %w", err)
	}

	// Check if the game is in a valid state for joining
	if game.State != domain.GameStateWaiting {
		return nil, fmt.Errorf("game is not in waiting state")
	}

	// Add the player to the game
	_, err = game.AddPlayer(playerID, playerName)
	if err != nil {
		return nil, fmt.Errorf("failed to add player to game: %w", err)
	}

	// Update the game in the repository
	game.UpdatedAt = time.Now()
	err = uc.gameRepo.Update(game)
	if err != nil {
		return nil, fmt.Errorf("failed to update game: %w", err)
	}

	return game, nil
}

// Ready sets a player's ready status
func (uc *GameUseCase) Ready(gameID, playerID string) (*domain.Game, error) {
	// Get the game from the repository
	game, err := uc.gameRepo.Get(gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to get game: %w", err)
	}

	// Check if the game is in a valid state for setting ready
	if game.State != domain.GameStateWaiting {
		return nil, fmt.Errorf("game is not in waiting state")
	}

	// Set the player's ready status
	err = game.SetPlayerReady(playerID)
	if err != nil {
		return nil, fmt.Errorf("failed to set player ready: %w", err)
	}

	// Update the game in the repository
	game.UpdatedAt = time.Now()
	err = uc.gameRepo.Update(game)
	if err != nil {
		return nil, fmt.Errorf("failed to update game: %w", err)
	}

	return game, nil
}

// Attack performs a player attack on the boss
func (uc *GameUseCase) Attack(gameID, playerID string) (*domain.Game, error) {
	// Get the game from the repository
	game, err := uc.gameRepo.Get(gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to get game: %w", err)
	}

	// Check if the game is in a valid state for attacking
	if game.State != domain.GameStatePlaying {
		return nil, fmt.Errorf("game is not in playing state")
	}

	// Check if the player is in the game
	_, exists := game.Players[playerID]
	if !exists {
		return nil, fmt.Errorf("player not in game")
	}

	// Perform the attack
	_, err = game.PlayerAttack(playerID)
	if err != nil {
		return nil, fmt.Errorf("failed to perform attack: %w", err)
	}

	// Update the game in the repository
	game.UpdatedAt = time.Now()
	err = uc.gameRepo.Update(game)
	if err != nil {
		return nil, fmt.Errorf("failed to update game: %w", err)
	}

	return game, nil
}

// ProcessBossAttack processes a boss attack
func (uc *GameUseCase) ProcessBossAttack(gameID string) (*domain.Game, error) {
	// Get the game from the repository
	game, err := uc.gameRepo.Get(gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to get game: %w", err)
	}

	// Check if the game is in a valid state for boss attacking
	if game.State != domain.GameStatePlaying {
		return nil, fmt.Errorf("game is not in playing state")
	}

	// Perform the boss attack
	game.BossAttack()

	// Update the game in the repository
	game.UpdatedAt = time.Now()
	err = uc.gameRepo.Update(game)
	if err != nil {
		return nil, fmt.Errorf("failed to update game: %w", err)
	}

	return game, nil
}

// ProcessBossAction processes a boss action (random attack on a player)
func (uc *GameUseCase) ProcessBossAction(gameID string) (*domain.Game, *domain.Player, int, error) {
	// Get the game from the repository
	game, err := uc.gameRepo.Get(gameID)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to get game: %w", err)
	}

	// Check if the game is in a valid state for boss attacking
	if game.State != domain.GameStatePlaying {
		return nil, nil, 0, fmt.Errorf("game is not in playing state")
	}

	// Check if the boss can attack based on its attack speed
	if !game.Boss.CanAttack() {
		return nil, nil, 0, fmt.Errorf("boss cannot attack yet")
	}

	// Get all alive players
	alivePlayers := make([]*domain.Player, 0)
	for _, player := range game.Players {
		if player.Character.Stats.Health > 0 {
			alivePlayers = append(alivePlayers, player)
		}
	}

	// Check if there are any alive players
	if len(alivePlayers) == 0 {
		return nil, nil, 0, fmt.Errorf("no alive players to attack")
	}

	// Select a random player to attack
	targetIndex := rand.Intn(len(alivePlayers))
	targetPlayer := alivePlayers[targetIndex]

	// Calculate damage
	damage := game.Boss.Attack()
	actualDamage := damage - targetPlayer.Character.Stats.Defense
	if actualDamage < 1 {
		actualDamage = 1 // Minimum damage is 1
	}

	// Apply damage to the target player
	targetPlayer.Character.Stats.Health -= actualDamage
	if targetPlayer.Character.Stats.Health < 0 {
		targetPlayer.Character.Stats.Health = 0
	}

	// Check if all players are defeated
	allDefeated := true
	for _, player := range game.Players {
		if player.Character.Stats.Health > 0 {
			allDefeated = false
			break
		}
	}

	// If all players are defeated, set the game state to finished and result to defeat
	if allDefeated {
		game.State = domain.GameStateFinished
		game.Result = domain.GameResultDefeat
		game.EndTime = time.Now()
	}

	// Update the game in the repository
	game.UpdatedAt = time.Now()
	err = uc.gameRepo.Update(game)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to update game: %w", err)
	}

	return game, targetPlayer, actualDamage, nil
}

// EquipItem equips an item for a player in a game
func (uc *GameUseCase) EquipItem(gameID, playerID, itemID string) (*domain.Game, error) {
	// Get the game from the repository
	game, err := uc.gameRepo.Get(gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to get game: %w", err)
	}

	// Check if the player is in the game
	player, exists := game.Players[playerID]
	if !exists {
		return nil, fmt.Errorf("player not in game")
	}

	// Check if the item is in the player's inventory
	item, exists := player.Character.Inventory[itemID]
	if !exists {
		return nil, fmt.Errorf("item not in inventory")
	}

	// Equip the item
	if item.Type == domain.ItemTypeWeapon {
		err = player.Character.EquipWeapon(itemID)
	} else if item.Type == domain.ItemTypeArmor {
		err = player.Character.EquipArmor(itemID)
	} else {
		return nil, fmt.Errorf("item cannot be equipped")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to equip item: %w", err)
	}

	// Update the game in the repository
	game.UpdatedAt = time.Now()
	err = uc.gameRepo.Update(game)
	if err != nil {
		return nil, fmt.Errorf("failed to update game: %w", err)
	}

	return game, nil
}

// StartCrafting starts crafting an item
func (uc *GameUseCase) StartCrafting(gameID, playerID, itemID string) (*domain.Game, error) {
	// Get the game from the repository
	game, err := uc.gameRepo.Get(gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to get game: %w", err)
	}

	// Start crafting the item
	_, err = game.StartCrafting(playerID, itemID)
	if err != nil {
		return nil, fmt.Errorf("failed to start crafting: %w", err)
	}

	// Update the game in the repository
	game.UpdatedAt = time.Now()
	err = uc.gameRepo.Update(game)
	if err != nil {
		return nil, fmt.Errorf("failed to update game: %w", err)
	}

	return game, nil
}

// HelpCrafting helps another player's crafting
func (uc *GameUseCase) HelpCrafting(gameID, playerID, craftingID string) (*domain.Game, error) {
	// Get the game from the repository
	game, err := uc.gameRepo.Get(gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to get game: %w", err)
	}

	// Help with the crafting
	_, err = game.HelpCrafting(playerID, craftingID)
	if err != nil {
		return nil, fmt.Errorf("failed to help crafting: %w", err)
	}

	// Update the game in the repository
	game.UpdatedAt = time.Now()
	err = uc.gameRepo.Update(game)
	if err != nil {
		return nil, fmt.Errorf("failed to update game: %w", err)
	}

	return game, nil
}

// GetCraftingItems gets all crafting items for a game
func (uc *GameUseCase) GetCraftingItems(gameID string) (*domain.Game, []domain.CraftingItem, error) {
	// Get the game from the repository
	game, err := uc.gameRepo.Get(gameID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get game: %w", err)
	}

	// Get the crafting items
	craftingItems := game.GetCraftingItems()

	// Update the game in the repository if status changed
	if game.CraftingSystem.UpdateCraftingStatus() {
		game.UpdatedAt = time.Now()
		err = uc.gameRepo.Update(game)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to update game: %w", err)
		}
	}

	return game, craftingItems, nil
}

// GetCraftableItems gets all craftable items for a game
func (uc *GameUseCase) GetCraftableItems(gameID string) (*domain.Game, []domain.CraftableItem, error) {
	// Get the game from the repository
	game, err := uc.gameRepo.Get(gameID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get game: %w", err)
	}

	// Get the craftable items
	craftableItems := game.GetCraftableItems()

	return game, craftableItems, nil
}
