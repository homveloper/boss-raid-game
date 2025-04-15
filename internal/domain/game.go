package domain

import (
	"errors"
	"math/rand"
	"time"
)

// Player represents a player in the game
type Player struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Ready      bool       `json:"ready"`
	Character  *Character `json:"character"`
	LastAttack time.Time  `json:"lastAttack"`
}

// GameState represents the state of the game
type GameState string

const (
	GameStateWaiting  GameState = "waiting"  // Waiting for players to join
	GameStateReady    GameState = "ready"    // All players are ready
	GameStatePlaying  GameState = "playing"  // Game is in progress
	GameStateFinished GameState = "finished" // Game is finished
)

// GameResult represents the result of the game
type GameResult string

const (
	GameResultNone    GameResult = "none"    // Game is not finished
	GameResultVictory GameResult = "victory" // Players defeated the boss
	GameResultDefeat  GameResult = "defeat"  // Players were defeated
	GameResultAbort   GameResult = "abort"   // Game was aborted
)

// Reward represents a reward for completing a boss raid
type Reward struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"` // gold, item, etc.
	Value       int    `json:"value"`
}

// GameEvent represents an event in the game
type GameEvent struct {
	Time        time.Time   `json:"time"`
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Data        interface{} `json:"data,omitempty"`
}

// Game represents a boss raid game
type Game struct {
	ID        string               `json:"id"`
	RoomID    string               `json:"roomId"`
	State     GameState            `json:"state"`
	Result    GameResult           `json:"result"`
	Players   map[string]*Player   `json:"players"`
	Boss      *Boss                `json:"boss"`
	Rewards   map[string][]*Reward `json:"rewards"` // Map of player ID to rewards
	StartTime time.Time            `json:"startTime"`
	EndTime   time.Time            `json:"endTime"`
	Events    []GameEvent          `json:"events"`
	CreatedAt time.Time            `json:"createdAt"`
	UpdatedAt time.Time            `json:"updatedAt"`
}

// NewGame creates a new game
func NewGame(id, roomID string) *Game {
	return &Game{
		ID:        id,
		RoomID:    roomID,
		State:     GameStateWaiting,
		Result:    GameResultNone,
		Players:   make(map[string]*Player),
		Rewards:   make(map[string][]*Reward),
		Events:    make([]GameEvent, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// AddPlayer adds a player to the game
func (g *Game) AddPlayer(id, name string) (*Player, error) {
	if len(g.Players) >= 3 {
		return nil, errors.New("game is full")
	}

	if _, exists := g.Players[id]; exists {
		return nil, errors.New("player already in game")
	}

	player := &Player{
		ID:         id,
		Name:       name,
		Ready:      false,
		Character:  NewCharacter(id, name),
		LastAttack: time.Now(),
	}

	g.Players[id] = player
	g.AddEvent("player_join", player.Name+" joined the game", player)
	g.UpdatedAt = time.Now()

	return player, nil
}

// RemovePlayer removes a player from the game
func (g *Game) RemovePlayer(id string) error {
	if _, exists := g.Players[id]; !exists {
		return errors.New("player not in game")
	}

	playerName := g.Players[id].Name
	delete(g.Players, id)
	g.AddEvent("player_leave", playerName+" left the game", nil)
	g.UpdatedAt = time.Now()

	return nil
}

// SetPlayerReady sets a player's ready status
func (g *Game) SetPlayerReady(id string) error {
	player, exists := g.Players[id]
	if !exists {
		return errors.New("player not in game")
	}

	player.Ready = true
	g.AddEvent("player_ready", player.Name+" is ready", player)
	g.UpdatedAt = time.Now()

	// Check if all players are ready
	allReady := true
	for _, p := range g.Players {
		if !p.Ready {
			allReady = false
			break
		}
	}

	// If all players are ready and there are 3 players, start the game
	if allReady && len(g.Players) == 3 {
		g.StartGame()
	}

	return nil
}

// StartGame starts the game
func (g *Game) StartGame() {
	g.State = GameStatePlaying
	g.StartTime = time.Now()
	g.Boss = NewBoss(GetRandomBossType())
	g.AddEvent("game_start", "The battle against "+g.Boss.Name+" has begun!", g.Boss)
	g.UpdatedAt = time.Now()
}

// EndGame ends the game with the specified result
func (g *Game) EndGame(result GameResult) {
	g.State = GameStateFinished
	g.Result = result
	g.EndTime = time.Now()
	g.UpdatedAt = time.Now()

	// Generate rewards if victory
	if result == GameResultVictory {
		g.GenerateRewards()
		g.AddEvent("game_end", "Victory! The "+g.Boss.Name+" has been defeated!", g.Rewards)
	} else {
		g.AddEvent("game_end", "Defeat! The party has been wiped out by the "+g.Boss.Name+"!", nil)
	}
}

// GenerateRewards generates rewards for all players
func (g *Game) GenerateRewards() {
	for playerID := range g.Players {
		g.Rewards[playerID] = generateRandomRewards()
	}
}

// AddEvent adds an event to the game's event log
func (g *Game) AddEvent(eventType, description string, data interface{}) {
	event := GameEvent{
		Time:        time.Now(),
		Type:        eventType,
		Description: description,
		Data:        data,
	}
	g.Events = append(g.Events, event)
}

// PlayerCanAttack checks if a player can attack based on their weapon's attack speed
func (g *Game) PlayerCanAttack(playerID string) bool {
	player, exists := g.Players[playerID]
	if !exists {
		return false
	}

	attackSpeed := player.Character.GetAttackSpeed()
	return time.Since(player.LastAttack) >= time.Duration(attackSpeed)*time.Millisecond
}

// PlayerAttack performs a player attack on the boss
func (g *Game) PlayerAttack(playerID string) (int, error) {
	if g.State != GameStatePlaying {
		return 0, errors.New("game is not in progress")
	}

	player, exists := g.Players[playerID]
	if !exists {
		return 0, errors.New("player not in game")
	}

	if !g.PlayerCanAttack(playerID) {
		return 0, errors.New("player cannot attack yet")
	}

	// Update last attack time
	player.LastAttack = time.Now()

	// Calculate damage
	damage := player.Character.Stats.Attack
	actualDamage := g.Boss.TakeDamage(damage)

	// Add event
	g.AddEvent("player_attack", player.Name+" attacked the "+g.Boss.Name+" for "+string(rune(actualDamage))+" damage", map[string]interface{}{
		"playerID": playerID,
		"damage":   actualDamage,
	})

	// Check if boss is defeated
	if g.Boss.IsDefeated() {
		g.EndGame(GameResultVictory)
	}

	g.UpdatedAt = time.Now()
	return actualDamage, nil
}

// BossAttack performs a boss attack on a random player
func (g *Game) BossAttack() {
	if g.State != GameStatePlaying || g.Boss.IsDefeated() {
		return
	}

	if !g.Boss.CanAttack() {
		return
	}

	// Get a random player
	var targetPlayer *Player
	for _, player := range g.Players {
		targetPlayer = player
		break
	}

	if targetPlayer == nil {
		return
	}

	// Calculate damage
	damage := g.Boss.Attack()
	actualDamage := damage - targetPlayer.Character.Stats.Defense
	if actualDamage < 1 {
		actualDamage = 1 // Minimum damage is 1
	}

	// Apply damage to player
	targetPlayer.Character.Stats.Health -= actualDamage

	// Add event
	g.AddEvent("boss_attack", g.Boss.Name+" attacked "+targetPlayer.Name+" for "+string(rune(actualDamage))+" damage", map[string]interface{}{
		"playerID": targetPlayer.ID,
		"damage":   actualDamage,
	})

	// Check if player is defeated
	if targetPlayer.Character.Stats.Health <= 0 {
		targetPlayer.Character.Stats.Health = 0
		g.AddEvent("player_defeated", targetPlayer.Name+" has been defeated!", targetPlayer)

		// Check if all players are defeated
		allDefeated := true
		for _, p := range g.Players {
			if p.Character.Stats.Health > 0 {
				allDefeated = false
				break
			}
		}

		if allDefeated {
			g.EndGame(GameResultDefeat)
		}
	}

	g.UpdatedAt = time.Now()
}

// generateRandomRewards generates random rewards
func generateRandomRewards() []*Reward {
	rewards := make([]*Reward, 0)

	// Add gold reward
	goldAmount := 100 + rand.Intn(900) // 100-999 gold
	goldReward := &Reward{
		ID:          "reward_" + generateRandomID(),
		Name:        "Gold",
		Description: "A pile of gold coins",
		Type:        "gold",
		Value:       goldAmount,
	}
	rewards = append(rewards, goldReward)

	// Chance for an item reward
	if rand.Float64() < 0.5 { // 50% chance
		itemTypes := []string{"weapon", "armor", "potion"}
		itemType := itemTypes[rand.Intn(len(itemTypes))]
		itemValue := 50 + rand.Intn(200) // 50-249 value

		itemReward := &Reward{
			ID:          "reward_" + generateRandomID(),
			Name:        "Mystery " + itemType,
			Description: "A mysterious " + itemType,
			Type:        itemType,
			Value:       itemValue,
		}
		rewards = append(rewards, itemReward)
	}

	return rewards
}

// GameRepository defines the methods for game data access
type GameRepository interface {
	Create(game *Game) error
	Get(id string) (*Game, error)
	Update(game *Game) error
	Delete(id string) error
	List() ([]*Game, error)
}

// GameUseCase defines the methods for game business logic
type GameUseCase interface {
	Create(roomID string) (*Game, error)
	Get(id string) (*Game, error)
	Join(gameID, playerID, playerName string) (*Game, error)
	Ready(gameID, playerID string) (*Game, error)
	Attack(gameID, playerID string) (*Game, error)
	ProcessBossAttack(gameID string) (*Game, error)
	ProcessBossAction(gameID string) (*Game, *Player, int, error) // Returns game, target player, damage, error
	EquipItem(gameID, playerID, itemID string) (*Game, error)
	List() ([]*Game, error)
}
