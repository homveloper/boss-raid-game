package model

import (
	"time"
)

// UnitType represents the type of unit
type UnitType string

const (
	// UnitTypePlayer represents a player unit
	UnitTypePlayer UnitType = "player"
	// UnitTypeMonster represents a monster unit
	UnitTypeMonster UnitType = "monster"
)

// Unit represents a unit in the game (player or monster)
type Unit struct {
	ID        string    `bson:"id" json:"id"`
	Type      UnitType  `bson:"type" json:"type"`
	Name      string    `bson:"name" json:"name"`
	X         float64   `bson:"x" json:"x"`
	Y         float64   `bson:"y" json:"y"`
	Health    int       `bson:"health" json:"health"`
	MaxHealth int       `bson:"max_health" json:"maxHealth"`
	Attack    int       `bson:"attack" json:"attack"`
	Defense   int       `bson:"defense" json:"defense"`
	Speed     float64   `bson:"speed" json:"speed"`
	CreatedAt time.Time `bson:"created_at" json:"createdAt"`
	UpdatedAt time.Time `bson:"updated_at" json:"updatedAt"`
	IsAlive   bool      `bson:"is_alive" json:"isAlive"`
}

// NewPlayer creates a new player unit
func NewPlayer(id, name string, x, y float64) Unit {
	now := time.Now()
	return Unit{
		ID:        id,
		Type:      UnitTypePlayer,
		Name:      name,
		X:         x,
		Y:         y,
		Health:    100,
		MaxHealth: 100,
		Attack:    10,
		Defense:   5,
		Speed:     5.0,
		CreatedAt: now,
		UpdatedAt: now,
		IsAlive:   true,
	}
}

// NewMonster creates a new monster unit
func NewMonster(id, name string, x, y float64) Unit {
	now := time.Now()
	return Unit{
		ID:        id,
		Type:      UnitTypeMonster,
		Name:      name,
		X:         x,
		Y:         y,
		Health:    50,
		MaxHealth: 50,
		Attack:    8,
		Defense:   3,
		Speed:     3.0,
		CreatedAt: now,
		UpdatedAt: now,
		IsAlive:   true,
	}
}

// TakeDamage applies damage to the unit and returns true if the unit died
func (u *Unit) TakeDamage(damage int) bool {
	effectiveDamage := damage - u.Defense
	if effectiveDamage < 1 {
		effectiveDamage = 1
	}

	u.Health -= effectiveDamage
	u.UpdatedAt = time.Now()

	if u.Health <= 0 {
		u.Health = 0
		u.IsAlive = false
		return true
	}

	return false
}

// Heal heals the unit by the specified amount
func (u *Unit) Heal(amount int) {
	u.Health += amount
	if u.Health > u.MaxHealth {
		u.Health = u.MaxHealth
	}
	u.UpdatedAt = time.Now()
}

// MoveTo moves the unit to the specified position
func (u *Unit) MoveTo(x, y float64) {
	u.X = x
	u.Y = y
	u.UpdatedAt = time.Now()
}

// Revive revives the unit with the specified health percentage
func (u *Unit) Revive(healthPercent float64) {
	if healthPercent < 0 {
		healthPercent = 0
	}
	if healthPercent > 1 {
		healthPercent = 1
	}

	u.Health = int(float64(u.MaxHealth) * healthPercent)
	u.IsAlive = true
	u.UpdatedAt = time.Now()
}
