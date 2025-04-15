package domain

import (
	"math/rand"
	"time"
)

// BossType represents the type of a boss
type BossType string

const (
	// BossTypeDragon represents a dragon boss
	BossTypeDragon BossType = "dragon"
	// BossTypeOgre represents an ogre boss
	BossTypeOgre BossType = "ogre"
	// BossTypeDemon represents a demon boss
	BossTypeDemon BossType = "demon"
	// BossTypeUndead represents an undead boss
	BossTypeUndead BossType = "undead"
)

// Boss represents a boss in the game
type Boss struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        BossType  `json:"type"`
	Health      int       `json:"health"`
	MaxHealth   int       `json:"maxHealth"`
	AttackPower int       `json:"attackPower"`
	Defense     int       `json:"defense"`
	AttackSpeed int       `json:"attackSpeed"` // in milliseconds
	LastAttack  time.Time `json:"lastAttack"`
}

// NewBoss creates a new boss of the specified type
func NewBoss(bossType BossType) *Boss {
	boss := &Boss{
		ID:         generateBossID(),
		Type:       bossType,
		LastAttack: time.Now(),
	}

	switch bossType {
	case BossTypeDragon:
		boss.Name = "Ancient Dragon"
		boss.MaxHealth = 1000
		boss.Health = 1000
		boss.AttackPower = 30
		boss.Defense = 20
		boss.AttackSpeed = 3000 // 3 seconds
	case BossTypeOgre:
		boss.Name = "Giant Ogre"
		boss.MaxHealth = 800
		boss.Health = 800
		boss.AttackPower = 25
		boss.Defense = 15
		boss.AttackSpeed = 2500 // 2.5 seconds
	case BossTypeDemon:
		boss.Name = "Infernal Demon"
		boss.MaxHealth = 700
		boss.Health = 700
		boss.AttackPower = 35
		boss.Defense = 10
		boss.AttackSpeed = 2000 // 2 seconds
	case BossTypeUndead:
		boss.Name = "Lich King"
		boss.MaxHealth = 600
		boss.Health = 600
		boss.AttackPower = 40
		boss.Defense = 5
		boss.AttackSpeed = 1800 // 1.8 seconds
	default:
		boss.Name = "Unknown Boss"
		boss.MaxHealth = 500
		boss.Health = 500
		boss.AttackPower = 20
		boss.Defense = 10
		boss.AttackSpeed = 2000 // 2 seconds
	}

	return boss
}

// TakeDamage applies damage to the boss and returns the actual damage dealt
func (b *Boss) TakeDamage(damage int) int {
	// Apply defense reduction
	actualDamage := damage - b.Defense
	if actualDamage < 1 {
		actualDamage = 1 // Minimum damage is 1
	}

	b.Health -= actualDamage
	if b.Health < 0 {
		b.Health = 0
	}

	return actualDamage
}

// IsDefeated checks if the boss is defeated
func (b *Boss) IsDefeated() bool {
	return b.Health <= 0
}

// CanAttack checks if the boss can attack based on attack speed
func (b *Boss) CanAttack() bool {
	return time.Since(b.LastAttack) >= time.Duration(b.AttackSpeed)*time.Millisecond
}

// Attack performs an attack and returns the damage
func (b *Boss) Attack() int {
	b.LastAttack = time.Now()
	// Add some randomness to the attack
	damageVariation := float64(b.AttackPower) * 0.2 // 20% variation
	minDamage := float64(b.AttackPower) - damageVariation
	maxDamage := float64(b.AttackPower) + damageVariation
	return int(minDamage + rand.Float64()*(maxDamage-minDamage))
}

// GetRandomBossType returns a random boss type
func GetRandomBossType() BossType {
	bossTypes := []BossType{
		BossTypeDragon,
		BossTypeOgre,
		BossTypeDemon,
		BossTypeUndead,
	}
	return bossTypes[rand.Intn(len(bossTypes))]
}

// generateBossID generates a unique ID for a boss
func generateBossID() string {
	return "boss_" + generateRandomID()
}

// generateRandomID generates a random ID
func generateRandomID() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, 8)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result)
}

// NewRandomBoss creates a new boss of a random type
func NewRandomBoss() *Boss {
	return NewBoss(GetRandomBossType())
}
