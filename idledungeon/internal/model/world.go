package model

import (
	"math/rand"
)

// World represents the game world
type World struct {
	Width  int `bson:"width" json:"width"`
	Height int `bson:"height" json:"height"`
}

// NewWorld creates a new world with the specified dimensions
func NewWorld(width, height int) *World {
	return &World{
		Width:  width,
		Height: height,
	}
}

// RandomX returns a random X coordinate within the world
func (w *World) RandomX() int {
	return rand.Intn(w.Width)
}

// RandomY returns a random Y coordinate within the world
func (w *World) RandomY() int {
	return rand.Intn(w.Height)
}

// Copy creates a deep copy of the world
func (w *World) Copy() *World {
	return &World{
		Width:  w.Width,
		Height: w.Height,
	}
}
