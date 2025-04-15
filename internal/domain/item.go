package domain

import "time"

// ItemType represents the type of an item
type ItemType string

const (
	// ItemTypeWeapon represents a weapon item
	ItemTypeWeapon ItemType = "weapon"
	// ItemTypeArmor represents an armor item
	ItemTypeArmor ItemType = "armor"
)

// WeaponType represents the type of a weapon
type WeaponType string

const (
	// WeaponTypeLongsword represents a longsword weapon
	WeaponTypeLongsword WeaponType = "longsword"
	// WeaponTypeDagger represents a dagger weapon
	WeaponTypeDagger WeaponType = "dagger"
	// WeaponTypeBow represents a bow weapon
	WeaponTypeBow WeaponType = "bow"
	// WeaponTypeAxe represents an axe weapon
	WeaponTypeAxe WeaponType = "axe"
)

// ArmorType represents the type of armor
type ArmorType string

const (
	// ArmorTypeLeather represents leather armor
	ArmorTypeLeather ArmorType = "leather"
)

// Item represents an item in the game
type Item struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Type        ItemType  `json:"type"`
	Stats       ItemStats `json:"stats"`
}

// ItemStats represents the stats of an item
type ItemStats struct {
	// Common stats
	Defense int `json:"defense"`

	// Weapon-specific stats
	Damage      int           `json:"damage"`
	AttackSpeed time.Duration `json:"attackSpeed"` // in milliseconds
	WeaponType  WeaponType    `json:"weaponType,omitempty"`

	// Armor-specific stats
	ArmorType ArmorType `json:"armorType,omitempty"`
}

// ItemRepository defines the interface for item storage
type ItemRepository interface {
	GetByID(id string) (*Item, error)
	GetAllWeapons() ([]*Item, error)
	GetAllArmors() ([]*Item, error)
	GetByType(itemType ItemType) ([]*Item, error)
}

// ItemUseCase defines the interface for item business logic
type ItemUseCase interface {
	GetByID(id string) (*Item, error)
	GetAllWeapons() ([]*Item, error)
	GetAllArmors() ([]*Item, error)
	GetByType(itemType ItemType) ([]*Item, error)
}
