package memory

import (
	"errors"
	"sync"
	"tictactoe/internal/domain"
	"time"
)

// ItemRepository is an in-memory implementation of domain.ItemRepository
type ItemRepository struct {
	items map[string]*domain.Item
	mu    sync.RWMutex
}

// NewItemRepository creates a new in-memory item repository with predefined items
func NewItemRepository() *ItemRepository {
	repo := &ItemRepository{
		items: make(map[string]*domain.Item),
	}

	// Add predefined weapons
	repo.addWeapon("longsword", "Longsword", "A standard longsword", domain.WeaponTypeLongsword, 15, 1500)
	repo.addWeapon("dagger", "Dagger", "A quick dagger", domain.WeaponTypeDagger, 8, 800)
	repo.addWeapon("bow", "Bow", "A ranged bow", domain.WeaponTypeBow, 12, 1200)
	repo.addWeapon("axe", "Battle Axe", "A heavy battle axe", domain.WeaponTypeAxe, 20, 2000)

	// Add predefined armor
	repo.addArmor("leather_armor", "Leather Armor", "Basic leather armor", domain.ArmorTypeLeather, 10)

	return repo
}

// addWeapon adds a weapon to the repository
func (r *ItemRepository) addWeapon(id, name, description string, weaponType domain.WeaponType, damage int, attackSpeed int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.items[id] = &domain.Item{
		ID:          id,
		Name:        name,
		Description: description,
		Type:        domain.ItemTypeWeapon,
		Stats: domain.ItemStats{
			Damage:      damage,
			AttackSpeed: time.Duration(attackSpeed) * time.Millisecond,
			WeaponType:  weaponType,
		},
	}
}

// addArmor adds armor to the repository
func (r *ItemRepository) addArmor(id, name, description string, armorType domain.ArmorType, defense int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.items[id] = &domain.Item{
		ID:          id,
		Name:        name,
		Description: description,
		Type:        domain.ItemTypeArmor,
		Stats: domain.ItemStats{
			Defense:   defense,
			ArmorType: armorType,
		},
	}
}

// GetByID retrieves an item by ID
func (r *ItemRepository) GetByID(id string) (*domain.Item, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	item, exists := r.items[id]
	if !exists {
		return nil, errors.New("item not found")
	}

	return item, nil
}

// GetAllWeapons retrieves all weapons
func (r *ItemRepository) GetAllWeapons() ([]*domain.Item, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	weapons := make([]*domain.Item, 0)
	for _, item := range r.items {
		if item.Type == domain.ItemTypeWeapon {
			weapons = append(weapons, item)
		}
	}

	return weapons, nil
}

// GetAllArmors retrieves all armors
func (r *ItemRepository) GetAllArmors() ([]*domain.Item, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	armors := make([]*domain.Item, 0)
	for _, item := range r.items {
		if item.Type == domain.ItemTypeArmor {
			armors = append(armors, item)
		}
	}

	return armors, nil
}

// GetByType retrieves items by type
func (r *ItemRepository) GetByType(itemType domain.ItemType) ([]*domain.Item, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]*domain.Item, 0)
	for _, item := range r.items {
		if item.Type == itemType {
			items = append(items, item)
		}
	}

	return items, nil
}
