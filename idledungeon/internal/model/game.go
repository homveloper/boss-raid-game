package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GameState represents the state of the game
type GameState string

const (
	GameStateWaiting  GameState = "waiting"
	GameStatePlaying  GameState = "playing"
	GameStateFinished GameState = "finished"
)

// Game represents the game state
type Game struct {
	ID          primitive.ObjectID `bson:"_id" json:"id"`
	Name        string             `bson:"name" json:"name"`
	State       GameState          `bson:"state" json:"state"`
	World       *World             `bson:"world" json:"world"`
	Players     map[string]*Player `bson:"players" json:"players"`
	Monsters    map[string]*Unit   `bson:"monsters" json:"monsters"`
	CreatedAt   time.Time          `bson:"created_at" json:"createdAt"`
	UpdatedAt   time.Time          `bson:"updated_at" json:"updatedAt"`
	VectorClock int64              `bson:"vector_clock" json:"vectorClock"`
}

// NewGame creates a new game instance
func NewGame(name string) *Game {
	world := NewWorld(1000, 1000) // 1000x1000 world size

	// Create a new game
	game := &Game{
		ID:          primitive.NewObjectID(),
		Name:        name,
		State:       GameStateWaiting,
		World:       world,
		Players:     make(map[string]*Player),
		Monsters:    make(map[string]*Unit),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		VectorClock: 1,
	}

	// Generate initial monsters
	game.GenerateMonsters(10)

	return game
}

// GenerateMonsters generates a specified number of monsters in the world
func (g *Game) GenerateMonsters(count int) {
	for i := 0; i < count; i++ {
		monster := NewMonster()
		// Place monster at random position in the world
		monster.X = float64(g.World.RandomX())
		monster.Y = float64(g.World.RandomY())
		g.Monsters[monster.ID] = monster
	}
}

// AddPlayer adds a new player to the game
func (g *Game) AddPlayer(name string) *Player {
	player := NewPlayer(name)
	// Place player at random position in the world
	player.X = float64(g.World.RandomX())
	player.Y = float64(g.World.RandomY())
	g.Players[player.ID] = player
	g.UpdatedAt = time.Now()
	g.VectorClock++
	return player
}

// RemovePlayer removes a player from the game
func (g *Game) RemovePlayer(playerID string) {
	delete(g.Players, playerID)
	g.UpdatedAt = time.Time{}
	g.VectorClock++
}

// UpdateGame updates the game state (moves units, handles combat, etc.)
func (g *Game) UpdateGame() {
	// Update all monsters
	for _, monster := range g.Monsters {
		// Move monster randomly
		monster.MoveRandomly(g.World)

		// Check for nearby players to attack
		for _, player := range g.Players {
			if monster.IsNear(player.Unit, 50) { // Attack range of 50 units
				monster.Attack(player.Unit)
			}
		}
	}

	// Remove dead monsters
	for id, monster := range g.Monsters {
		if monster.Health <= 0 {
			delete(g.Monsters, id)

			// Respawn a new monster
			newMonster := NewMonster()
			newMonster.X = float64(g.World.RandomX())
			newMonster.Y = float64(g.World.RandomY())
			g.Monsters[newMonster.ID] = newMonster
		}
	}

	// Update game state
	g.UpdatedAt = time.Now()
	g.VectorClock++
}

// Copy implements the nodestorage.Cachable interface
func (g *Game) Copy() *Game {
	// Create a deep copy of the game
	gameCopy := &Game{
		ID:          g.ID,
		Name:        g.Name,
		State:       g.State,
		CreatedAt:   g.CreatedAt,
		UpdatedAt:   g.UpdatedAt,
		VectorClock: g.VectorClock,
	}

	// Copy world
	if g.World != nil {
		gameCopy.World = g.World.Copy()
	}

	// Copy players
	gameCopy.Players = make(map[string]*Player)
	for id, player := range g.Players {
		gameCopy.Players[id] = player.Copy()
	}

	// Copy monsters
	gameCopy.Monsters = make(map[string]*Unit)
	for id, monster := range g.Monsters {
		gameCopy.Monsters[id] = monster.Copy()
	}

	return gameCopy
}

// Version implements the nodestorage.Cachable interface
func (g *Game) Version(v ...int64) int64 {
	if len(v) > 0 {
		g.VectorClock = v[0]
	}
	return g.VectorClock
}
