package model

import (
	"math"
	"math/rand"
	"time"
)

// UnitType represents the type of unit
type UnitType string

const (
	UnitTypePlayer  UnitType = "player"
	UnitTypeMonster UnitType = "monster"
)

// Unit represents a game unit (player or monster)
type Unit struct {
	ID           string    `bson:"id" json:"id"`
	Type         UnitType  `bson:"type" json:"type"`
	Name         string    `bson:"name" json:"name"`
	X            float64   `bson:"x" json:"x"`
	Y            float64   `bson:"y" json:"y"`
	Health       int       `bson:"health" json:"health"`
	MaxHealth    int       `bson:"max_health" json:"maxHealth"`
	AttackPower  int       `bson:"attack" json:"attack"`
	DefensePower int       `bson:"defense" json:"defense"`
	Speed        float64   `bson:"speed" json:"speed"`
	Gold         int       `bson:"gold" json:"gold"`
	LastMove     time.Time `bson:"last_move" json:"lastMove"`
}

// Player represents a player in the game
type Player struct {
	*Unit
	Connected bool      `bson:"connected" json:"connected"`
	LastSeen  time.Time `bson:"last_seen" json:"lastSeen"`
}

// NewPlayer creates a new player
func NewPlayer(name string) *Player {
	return &Player{
		Unit: &Unit{
			ID:           generateID(),
			Type:         UnitTypePlayer,
			Name:         name,
			X:            0,
			Y:            0,
			Health:       100,
			MaxHealth:    100,
			AttackPower:  10,
			DefensePower: 5,
			Speed:        20.0, // 플레이어 이동 속도 증가
			Gold:         0,
			LastMove:     time.Now(),
		},
		Connected: true,
		LastSeen:  time.Now(),
	}
}

// NewMonster creates a new monster
func NewMonster() *Unit {
	// Monster names
	monsterNames := []string{
		"Goblin", "Orc", "Troll", "Skeleton", "Zombie",
		"Ghost", "Vampire", "Werewolf", "Dragon", "Demon",
	}

	// Random monster stats
	health := 50 + rand.Intn(50)
	attack := 5 + rand.Intn(10)
	defense := 2 + rand.Intn(5)
	speed := 10.0 + rand.Float64()*5.0 // 몬스터 이동 속도 증가
	gold := 10 + rand.Intn(20)

	return &Unit{
		ID:           generateID(),
		Type:         UnitTypeMonster,
		Name:         monsterNames[rand.Intn(len(monsterNames))],
		X:            0,
		Y:            0,
		Health:       health,
		MaxHealth:    health,
		AttackPower:  attack,
		DefensePower: defense,
		Speed:        speed,
		Gold:         gold,
		LastMove:     time.Now(),
	}
}

// Move moves the unit to a new position
func (u *Unit) Move(x, y float64, world *World) {
	// Calculate distance
	dx := x - u.X
	dy := y - u.Y
	distance := math.Sqrt(dx*dx + dy*dy)

	// Calculate time since last move
	elapsed := time.Since(u.LastMove).Seconds()

	// Calculate maximum distance that can be moved
	maxDistance := u.Speed * elapsed

	// If trying to move too far, scale down the movement
	if distance > maxDistance {
		ratio := maxDistance / distance
		dx *= ratio
		dy *= ratio
	}

	// Update position, ensuring it stays within world bounds
	u.X = math.Max(0, math.Min(float64(world.Width), u.X+dx))
	u.Y = math.Max(0, math.Min(float64(world.Height), u.Y+dy))

	// Update last move time
	u.LastMove = time.Now()
}

// MoveRandomly moves the unit in a random direction
func (u *Unit) MoveRandomly(world *World) {
	// Random direction
	angle := rand.Float64() * 2 * math.Pi
	distance := u.Speed * 1.0 // 랜덤 이동 시 전체 속도로 이동

	// Calculate new position
	newX := u.X + math.Cos(angle)*distance
	newY := u.Y + math.Sin(angle)*distance

	// Move to new position
	u.Move(newX, newY, world)
}

// IsNear checks if another unit is within a certain distance
func (u *Unit) IsNear(other *Unit, distance float64) bool {
	dx := u.X - other.X
	dy := u.Y - other.Y
	return math.Sqrt(dx*dx+dy*dy) <= distance
}

// Attack makes this unit attack another unit
func (u *Unit) Attack(target *Unit) {
	// Calculate damage
	damage := u.AttackPower - target.DefensePower/2
	if damage < 1 {
		damage = 1 // Minimum damage
	}

	// Apply damage
	target.Health -= damage
	if target.Health < 0 {
		target.Health = 0
	}

	// If target is killed and attacker is a player, get gold
	if target.Health == 0 && u.Type == UnitTypePlayer && target.Type == UnitTypeMonster {
		u.Gold += target.Gold
	}
}

// Copy creates a deep copy of the unit
func (u *Unit) Copy() *Unit {
	return &Unit{
		ID:           u.ID,
		Type:         u.Type,
		Name:         u.Name,
		X:            u.X,
		Y:            u.Y,
		Health:       u.Health,
		MaxHealth:    u.MaxHealth,
		AttackPower:  u.AttackPower,
		DefensePower: u.DefensePower,
		Speed:        u.Speed,
		Gold:         u.Gold,
		LastMove:     u.LastMove,
	}
}

// Copy creates a deep copy of the player
func (p *Player) Copy() *Player {
	return &Player{
		Unit:      p.Unit.Copy(),
		Connected: p.Connected,
		LastSeen:  p.LastSeen,
	}
}

// Helper function to generate a unique ID
func generateID() string {
	return time.Now().Format("20060102150405") +
		"-" +
		randomString(8)
}

// Helper function to generate a random string
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result)
}
