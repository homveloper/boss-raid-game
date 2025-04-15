package http

import (
	"encoding/json"
	"net/http"
	"tictactoe/internal/domain"
)

// ItemHandler handles HTTP requests for items
type ItemHandler struct {
	itemUC domain.ItemUseCase
}

// NewItemHandler creates a new item handler
func NewItemHandler(itemUC domain.ItemUseCase) *ItemHandler {
	return &ItemHandler{
		itemUC: itemUC,
	}
}

// GetItem handles the retrieval of an item by ID
func (h *ItemHandler) GetItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Item ID is required", http.StatusBadRequest)
		return
	}

	item, err := h.itemUC.GetByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

// GetAllWeapons handles the retrieval of all weapons
func (h *ItemHandler) GetAllWeapons(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	weapons, err := h.itemUC.GetAllWeapons()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(weapons)
}

// GetAllArmors handles the retrieval of all armors
func (h *ItemHandler) GetAllArmors(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	armors, err := h.itemUC.GetAllArmors()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(armors)
}

// GetItemsByType handles the retrieval of items by type
func (h *ItemHandler) GetItemsByType(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	typeStr := r.URL.Query().Get("type")
	if typeStr == "" {
		http.Error(w, "Item type is required", http.StatusBadRequest)
		return
	}

	var itemType domain.ItemType
	if typeStr == "weapon" {
		itemType = domain.ItemTypeWeapon
	} else if typeStr == "armor" {
		itemType = domain.ItemTypeArmor
	} else {
		http.Error(w, "Invalid item type", http.StatusBadRequest)
		return
	}

	items, err := h.itemUC.GetByType(itemType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}
