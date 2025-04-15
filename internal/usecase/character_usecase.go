package usecase

import (
	"errors"
	"tictactoe/internal/domain"
)

// CharacterUseCase implements domain.CharacterUseCase
type CharacterUseCase struct {
	characterRepo domain.CharacterRepository
	itemRepo      domain.ItemRepository
}

// NewCharacterUseCase creates a new character use case
func NewCharacterUseCase(characterRepo domain.CharacterRepository, itemRepo domain.ItemRepository) *CharacterUseCase {
	return &CharacterUseCase{
		characterRepo: characterRepo,
		itemRepo:      itemRepo,
	}
}

// Create creates a new character
func (uc *CharacterUseCase) Create(name string, playerID string) (*domain.Character, error) {
	character := domain.NewCharacter(playerID, name)
	
	// Add default items to inventory
	weapons, err := uc.itemRepo.GetAllWeapons()
	if err == nil && len(weapons) > 0 {
		for _, weapon := range weapons {
			character.AddItemToInventory(weapon)
		}
	}
	
	armors, err := uc.itemRepo.GetAllArmors()
	if err == nil && len(armors) > 0 {
		for _, armor := range armors {
			character.AddItemToInventory(armor)
		}
	}
	
	// Equip default items
	if len(weapons) > 0 {
		_ = character.EquipWeapon(weapons[0].ID)
	}
	
	if len(armors) > 0 {
		_ = character.EquipArmor(armors[0].ID)
	}
	
	err = uc.characterRepo.Create(character)
	if err != nil {
		return nil, err
	}
	
	return character, nil
}

// GetByID retrieves a character by ID
func (uc *CharacterUseCase) GetByID(id string) (*domain.Character, error) {
	return uc.characterRepo.GetByID(id)
}

// EquipItem equips an item to a character
func (uc *CharacterUseCase) EquipItem(characterID, itemID string) (*domain.Character, error) {
	character, err := uc.characterRepo.GetByID(characterID)
	if err != nil {
		return nil, err
	}
	
	item, exists := character.Inventory[itemID]
	if !exists {
		return nil, errors.New("item not found in inventory")
	}
	
	if item.Type == domain.ItemTypeWeapon {
		err = character.EquipWeapon(itemID)
	} else if item.Type == domain.ItemTypeArmor {
		err = character.EquipArmor(itemID)
	} else {
		return nil, errors.New("item cannot be equipped")
	}
	
	if err != nil {
		return nil, err
	}
	
	err = uc.characterRepo.Update(character)
	if err != nil {
		return nil, err
	}
	
	return character, nil
}

// UnequipItem unequips an item from a character
func (uc *CharacterUseCase) UnequipItem(characterID string, itemType domain.ItemType) (*domain.Character, error) {
	character, err := uc.characterRepo.GetByID(characterID)
	if err != nil {
		return nil, err
	}
	
	if itemType == domain.ItemTypeWeapon {
		character.UnequipWeapon()
	} else if itemType == domain.ItemTypeArmor {
		character.UnequipArmor()
	} else {
		return nil, errors.New("invalid item type")
	}
	
	err = uc.characterRepo.Update(character)
	if err != nil {
		return nil, err
	}
	
	return character, nil
}

// AddItemToInventory adds an item to a character's inventory
func (uc *CharacterUseCase) AddItemToInventory(characterID, itemID string) (*domain.Character, error) {
	character, err := uc.characterRepo.GetByID(characterID)
	if err != nil {
		return nil, err
	}
	
	item, err := uc.itemRepo.GetByID(itemID)
	if err != nil {
		return nil, err
	}
	
	character.AddItemToInventory(item)
	
	err = uc.characterRepo.Update(character)
	if err != nil {
		return nil, err
	}
	
	return character, nil
}
