package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GameState represents the overall game state
type GameState struct {
	ID          primitive.ObjectID `bson:"_id" json:"id"`
	Name        string             `bson:"name" json:"name"`
	Units       map[string]Unit    `bson:"units" json:"units"`
	LastUpdated time.Time          `bson:"last_updated" json:"lastUpdated"`
	Version     int64              `bson:"version" json:"version"`
}

// Copy creates a deep copy of the GameState
func (g *GameState) Copy() *GameState {
	unitsCopy := make(map[string]Unit, len(g.Units))
	for id, unit := range g.Units {
		unitCopy := unit
		unitsCopy[id] = unitCopy
	}

	return &GameState{
		ID:          g.ID,
		Name:        g.Name,
		Units:       unitsCopy,
		LastUpdated: g.LastUpdated,
		Version:     g.Version,
	}
}

// AddUnit adds a unit to the game state
func (g *GameState) AddUnit(unit Unit) {
	if g.Units == nil {
		g.Units = make(map[string]Unit)
	}
	g.Units[unit.ID] = unit
}

// RemoveUnit removes a unit from the game state
func (g *GameState) RemoveUnit(unitID string) {
	if g.Units != nil {
		delete(g.Units, unitID)
	}
}

// UpdateUnit updates a unit in the game state
func (g *GameState) UpdateUnit(unit Unit) {
	if g.Units == nil {
		g.Units = make(map[string]Unit)
	}
	g.Units[unit.ID] = unit
}

// GetUnit returns a unit by ID
func (g *GameState) GetUnit(unitID string) (Unit, bool) {
	if g.Units == nil {
		return Unit{}, false
	}
	unit, exists := g.Units[unitID]
	return unit, exists
}
