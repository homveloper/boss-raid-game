package usecase

import (
	"tictactoe/internal/domain"
)

// ItemUseCase implements domain.ItemUseCase
type ItemUseCase struct {
	itemRepo domain.ItemRepository
}

// NewItemUseCase creates a new item use case
func NewItemUseCase(itemRepo domain.ItemRepository) *ItemUseCase {
	return &ItemUseCase{
		itemRepo: itemRepo,
	}
}

// GetByID retrieves an item by ID
func (uc *ItemUseCase) GetByID(id string) (*domain.Item, error) {
	return uc.itemRepo.GetByID(id)
}

// GetAllWeapons retrieves all weapons
func (uc *ItemUseCase) GetAllWeapons() ([]*domain.Item, error) {
	return uc.itemRepo.GetAllWeapons()
}

// GetAllArmors retrieves all armors
func (uc *ItemUseCase) GetAllArmors() ([]*domain.Item, error) {
	return uc.itemRepo.GetAllArmors()
}

// GetByType retrieves items by type
func (uc *ItemUseCase) GetByType(itemType domain.ItemType) ([]*domain.Item, error) {
	return uc.itemRepo.GetByType(itemType)
}
