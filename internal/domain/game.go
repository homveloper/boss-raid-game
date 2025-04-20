package domain

import (
	"errors"
	"fmt"
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
	ID             string               `json:"id"`
	RoomID         string               `json:"roomId"`
	State          GameState            `json:"state"`
	Result         GameResult           `json:"result"`
	Players        map[string]*Player   `json:"players"`
	Boss           *Boss                `json:"boss"`
	Rewards        map[string][]*Reward `json:"rewards"` // Map of player ID to rewards
	StartTime      time.Time            `json:"startTime"`
	EndTime        time.Time            `json:"endTime"`
	Events         []GameEvent          `json:"events"`
	CreatedAt      time.Time            `json:"createdAt"`
	UpdatedAt      time.Time            `json:"updatedAt"`
	CraftingSystem *CraftingSystem      `json:"craftingSystem"` // 협동 아이템 제작 시스템
}

// NewGame creates a new game
func NewGame(id, roomID string) *Game {
	// 제작 시스템 초기화
	craftingSystem := NewCraftingSystem()
	craftingSystem.InitDefaultCraftableItems()

	return &Game{
		ID:             id,
		RoomID:         roomID,
		State:          GameStateWaiting,
		Result:         GameResultNone,
		Players:        make(map[string]*Player),
		Rewards:        make(map[string][]*Reward),
		Events:         make([]GameEvent, 0),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		CraftingSystem: craftingSystem,
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

	// 이미 준비 상태인 경우 준비 취소로 변경
	if player.Ready {
		player.Ready = false
		g.AddEvent("player_not_ready", player.Name+" is not ready", player)
	} else {
		player.Ready = true
		g.AddEvent("player_ready", player.Name+" is ready", player)
	}

	g.UpdatedAt = time.Now()

	// Check if all players are ready
	allReady := true
	for _, p := range g.Players {
		if !p.Ready {
			allReady = false
			break
		}
	}

	// If all players are ready and there are at least 1 player, start the game
	if allReady && len(g.Players) >= 1 {
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

// StartCrafting starts crafting an item
func (g *Game) StartCrafting(playerID, itemID string) (*CraftingItem, error) {
	// 플레이어 확인
	player, exists := g.Players[playerID]
	if !exists {
		return nil, errors.New("player not in game")
	}

	// 제작 가능한 아이템인지 확인
	craftableItem, exists := g.CraftingSystem.CraftableItems[itemID]
	if !exists {
		return nil, errors.New("item not available for crafting")
	}

	// 플레이어가 이미 제작 중인 아이템이 있는지 확인
	for _, item := range g.CraftingSystem.CraftingItems {
		if item.CrafterID == playerID && item.Status == "in_progress" {
			return nil, errors.New("player already crafting an item")
		}
	}

	// 새 제작 아이템 생성
	craftingID := generateCraftingID()
	craftingItem := CraftingItem{
		ID:                  craftingID,
		ItemID:              itemID,
		CrafterID:           playerID,
		CrafterName:         player.Name,
		StartTime:           time.Now(),
		OriginalTimeMinutes: craftableItem.BaseTimeMinutes,
		CurrentTimeMinutes:  craftableItem.BaseTimeMinutes,
		Helpers:             make(map[string]int),
		Status:              "in_progress",
	}

	// 제작 목록에 추가
	g.CraftingSystem.CraftingItems[craftingID] = craftingItem
	g.CraftingSystem.LastUpdated = time.Now()

	// 이벤트 추가
	g.AddEvent("crafting_started", player.Name+" started crafting "+craftableItem.Name, map[string]interface{}{
		"craftingId": craftingID,
		"itemId":     itemID,
		"playerId":   playerID,
	})

	return &craftingItem, nil
}

// HelpCrafting helps another player's crafting
func (g *Game) HelpCrafting(helperID, craftingID string) (*CraftingItem, error) {
	// 플레이어 확인
	helper, exists := g.Players[helperID]
	if !exists {
		return nil, errors.New("helper player not in game")
	}

	// 제작 중인 아이템 확인
	craftingItem, exists := g.CraftingSystem.CraftingItems[craftingID]
	if !exists {
		return nil, errors.New("crafting item not found")
	}

	// 제작 중인 상태인지 확인
	if craftingItem.Status != "in_progress" {
		return nil, errors.New("item is not in progress")
	}

	// 자신의 아이템인지 확인
	if craftingItem.CrafterID == helperID {
		return nil, errors.New("cannot help your own crafting")
	}

	// 이미 도움을 준 시간 확인 (1시간에 한 번만 도움 가능)
	lastHelpTime := g.CraftingSystem.GetLastHelpTime(helperID, craftingID)
	if !lastHelpTime.IsZero() && time.Since(lastHelpTime) < time.Hour {
		return nil, errors.New("already helped this item recently")
	}

	// 도움 횟수 증가
	if _, exists := craftingItem.Helpers[helperID]; !exists {
		craftingItem.Helpers[helperID] = 0
	}
	craftingItem.Helpers[helperID]++

	// 제작 시간 감소 (1분)
	timeReduction := 1
	craftingItem.CurrentTimeMinutes -= timeReduction
	if craftingItem.CurrentTimeMinutes < 0 {
		craftingItem.CurrentTimeMinutes = 0
	}

	// 제작 완료 확인
	if g.CraftingSystem.IsCraftingCompleted(craftingID) {
		craftingItem.Status = "completed"
		g.AddEvent("crafting_completed", craftingItem.CrafterName+"'s "+
			g.CraftingSystem.CraftableItems[craftingItem.ItemID].Name+" crafting completed", map[string]interface{}{
			"craftingId": craftingID,
			"itemId":     craftingItem.ItemID,
			"playerId":   craftingItem.CrafterID,
		})
	}

	// 상태 업데이트
	g.CraftingSystem.CraftingItems[craftingID] = craftingItem
	g.CraftingSystem.LastUpdated = time.Now()

	// 이벤트 추가
	g.AddEvent("crafting_helped", helper.Name+" helped "+craftingItem.CrafterName+"'s crafting", map[string]interface{}{
		"craftingId":    craftingID,
		"itemId":        craftingItem.ItemID,
		"helperId":      helperID,
		"timeReduction": timeReduction,
	})

	return &craftingItem, nil
}

// GetCraftingItems gets all crafting items
func (g *Game) GetCraftingItems() []CraftingItem {
	// 상태 업데이트 (완료된 아이템 확인)
	g.CraftingSystem.UpdateCraftingStatus()

	// 제작 중인 아이템만 필터링
	var items []CraftingItem
	for _, item := range g.CraftingSystem.CraftingItems {
		items = append(items, item)
	}

	return items
}

// GetCraftableItems gets all craftable items
func (g *Game) GetCraftableItems() []CraftableItem {
	var items []CraftableItem
	for _, item := range g.CraftingSystem.CraftableItems {
		items = append(items, item)
	}

	return items
}

// generateCraftingID generates a random ID for crafting items
func generateCraftingID() string {
	// 실제 구현에서는 UUID 라이브러리를 사용하는 것이 좋습니다.
	// 여기서는 간단히 난수를 사용합니다.
	return fmt.Sprintf("craft_%d", time.Now().UnixNano())
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
	StartCrafting(gameID, playerID, itemID string) (*Game, error)
	HelpCrafting(gameID, playerID, craftingID string) (*Game, error)
	GetCraftingItems(gameID string) (*Game, []CraftingItem, error)
	GetCraftableItems(gameID string) (*Game, []CraftableItem, error)
	List() ([]*Game, error)
}
