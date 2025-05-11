package model

import (
	"math/rand"
	"time"
)

// WorldConfig represents the configuration for the game world
type WorldConfig struct {
	Width           int     `json:"width"`
	Height          int     `json:"height"`
	MonsterCount    int     `json:"monsterCount"`
	MonsterSpawnMin float64 `json:"monsterSpawnMin"`
	MonsterSpawnMax float64 `json:"monsterSpawnMax"`
}

// DefaultWorldConfig returns the default world configuration
func DefaultWorldConfig() WorldConfig {
	return WorldConfig{
		Width:           1000,
		Height:          1000,
		MonsterCount:    20,
		MonsterSpawnMin: 60.0,  // Minimum time in seconds between monster spawns
		MonsterSpawnMax: 120.0, // Maximum time in seconds between monster spawns
	}
}

// GenerateRandomPosition generates a random position within the world bounds
func GenerateRandomPosition(width, height int) (float64, float64) {
	return float64(rand.Intn(width)), float64(rand.Intn(height))
}

// GenerateMonsterName generates a random monster name
func GenerateMonsterName() string {
	prefixes := []string{
		"Fierce", "Dark", "Angry", "Savage", "Wild",
		"Brutal", "Deadly", "Vicious", "Feral", "Grim",
	}

	types := []string{
		"Goblin", "Orc", "Troll", "Skeleton", "Zombie",
		"Ghost", "Vampire", "Werewolf", "Dragon", "Demon",
	}

	return prefixes[rand.Intn(len(prefixes))] + " " + types[rand.Intn(len(types))]
}

// InitializeWorld initializes a new game world with random monsters
func InitializeWorld(config WorldConfig) map[string]Unit {
	rand.Seed(time.Now().UnixNano())
	
	units := make(map[string]Unit)
	
	// Generate random monsters
	for i := 0; i < config.MonsterCount; i++ {
		x, y := GenerateRandomPosition(config.Width, config.Height)
		monsterId := "monster-" + GenerateUUID()
		monster := NewMonster(monsterId, GenerateMonsterName(), x, y)
		units[monster.ID] = monster
	}
	
	return units
}

// GenerateUUID generates a simple UUID for unit IDs
func GenerateUUID() string {
	now := time.Now().UnixNano()
	rand.Seed(now)
	return RandomString(8) + "-" + RandomString(4) + "-" + RandomString(4) + "-" + RandomString(12)
}

// RandomString generates a random string of the specified length
func RandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result)
}
