package domain

import "errors"

// Character represents a player character in the game
type Character struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Inventory map[string]*Item `json:"inventory"`
	Equipment Equipment        `json:"equipment"`
	Stats     CharacterStats   `json:"stats"`
}

// Equipment represents the equipped items of a character
type Equipment struct {
	Weapon *Item `json:"weapon"`
	Armor  *Item `json:"armor"`
}

// CharacterStats represents the stats of a character
type CharacterStats struct {
	Health  int `json:"health"`
	Attack  int `json:"attack"`
	Defense int `json:"defense"`
}

// NewCharacter creates a new character with default stats
func NewCharacter(id, name string) *Character {
	return &Character{
		ID:        id,
		Name:      name,
		Inventory: make(map[string]*Item),
		Equipment: Equipment{},
		Stats: CharacterStats{
			Health:  100,
			Attack:  10,
			Defense: 5,
		},
	}
}

// AddItemToInventory adds an item to the character's inventory
func (c *Character) AddItemToInventory(item *Item) {
	c.Inventory[item.ID] = item
}

// RemoveItemFromInventory removes an item from the character's inventory
func (c *Character) RemoveItemFromInventory(itemID string) error {
	if _, exists := c.Inventory[itemID]; !exists {
		return errors.New("item not found in inventory")
	}

	delete(c.Inventory, itemID)
	return nil
}

// EquipWeapon equips a weapon from the inventory
func (c *Character) EquipWeapon(itemID string) error {
	item, exists := c.Inventory[itemID]
	if !exists {
		return errors.New("item not found in inventory")
	}

	if item.Type != ItemTypeWeapon {
		return errors.New("item is not a weapon")
	}

	c.Equipment.Weapon = item
	c.UpdateStats()
	return nil
}

// EquipArmor equips armor from the inventory
func (c *Character) EquipArmor(itemID string) error {
	item, exists := c.Inventory[itemID]
	if !exists {
		return errors.New("item not found in inventory")
	}

	if item.Type != ItemTypeArmor {
		return errors.New("item is not armor")
	}

	c.Equipment.Armor = item
	c.UpdateStats()
	return nil
}

// UnequipWeapon unequips the current weapon
func (c *Character) UnequipWeapon() {
	c.Equipment.Weapon = nil
	c.UpdateStats()
}

// UnequipArmor unequips the current armor
func (c *Character) UnequipArmor() {
	c.Equipment.Armor = nil
	c.UpdateStats()
}

// UpdateStats recalculates the character's stats based on equipment
func (c *Character) UpdateStats() {
	// Reset to base stats
	c.Stats.Health = 100
	c.Stats.Attack = 10
	c.Stats.Defense = 5

	// Add weapon stats
	if c.Equipment.Weapon != nil {
		c.Stats.Attack += c.Equipment.Weapon.Stats.Damage
	}

	// Add armor stats
	if c.Equipment.Armor != nil {
		c.Stats.Defense += c.Equipment.Armor.Stats.Defense
	}
}

// GetAttackSpeed returns the character's attack speed based on equipped weapon
func (c *Character) GetAttackSpeed() int {
	if c.Equipment.Weapon == nil {
		return 1000 // Default 1 second attack speed
	}
	return int(c.Equipment.Weapon.Stats.AttackSpeed.Milliseconds())
}

// CharacterRepository defines the interface for character storage
type CharacterRepository interface {
	Create(character *Character) error
	GetByID(id string) (*Character, error)
	Update(character *Character) error
	Delete(id string) error
}

// CharacterUseCase defines the interface for character business logic
type CharacterUseCase interface {
	Create(name string, playerID string) (*Character, error)
	GetByID(id string) (*Character, error)
	EquipItem(characterID, itemID string) (*Character, error)
	UnequipItem(characterID string, itemType ItemType) (*Character, error)
	AddItemToInventory(characterID, itemID string) (*Character, error)
}
