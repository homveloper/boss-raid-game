package http

import (
	"encoding/json"
	"net/http"
	"tictactoe/internal/domain"
)

// CharacterHandler handles HTTP requests for characters
type CharacterHandler struct {
	characterUC domain.CharacterUseCase
}

// NewCharacterHandler creates a new character handler
func NewCharacterHandler(characterUC domain.CharacterUseCase) *CharacterHandler {
	return &CharacterHandler{
		characterUC: characterUC,
	}
}

// GetCharacter handles the retrieval of a character by ID
func (h *CharacterHandler) GetCharacter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Character ID is required", http.StatusBadRequest)
		return
	}

	character, err := h.characterUC.GetByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(character)
}

// CreateCharacter handles the creation of a new character
func (h *CharacterHandler) CreateCharacter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name     string `json:"name"`
		PlayerID string `json:"playerId"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Character name is required", http.StatusBadRequest)
		return
	}

	if req.PlayerID == "" {
		http.Error(w, "Player ID is required", http.StatusBadRequest)
		return
	}

	character, err := h.characterUC.Create(req.Name, req.PlayerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(character)
}

// EquipItem handles equipping an item to a character
func (h *CharacterHandler) EquipItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		CharacterID string `json:"characterId"`
		ItemID      string `json:"itemId"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.CharacterID == "" {
		http.Error(w, "Character ID is required", http.StatusBadRequest)
		return
	}

	if req.ItemID == "" {
		http.Error(w, "Item ID is required", http.StatusBadRequest)
		return
	}

	character, err := h.characterUC.EquipItem(req.CharacterID, req.ItemID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(character)
}

// UnequipItem handles unequipping an item from a character
func (h *CharacterHandler) UnequipItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		CharacterID string `json:"characterId"`
		ItemType    string `json:"itemType"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.CharacterID == "" {
		http.Error(w, "Character ID is required", http.StatusBadRequest)
		return
	}

	if req.ItemType == "" {
		http.Error(w, "Item type is required", http.StatusBadRequest)
		return
	}

	var itemType domain.ItemType
	if req.ItemType == "weapon" {
		itemType = domain.ItemTypeWeapon
	} else if req.ItemType == "armor" {
		itemType = domain.ItemTypeArmor
	} else {
		http.Error(w, "Invalid item type", http.StatusBadRequest)
		return
	}

	character, err := h.characterUC.UnequipItem(req.CharacterID, itemType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(character)
}
