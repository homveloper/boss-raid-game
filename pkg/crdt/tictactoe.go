package crdt

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/gpestana/rdoc"
)

// TicTacToe represents a CRDT-based Tic-tac-toe game
type TicTacToe struct {
	doc  *rdoc.Doc
	mu   sync.RWMutex
	id   string
	data map[string]interface{}
}

// NewTicTacToe creates a new CRDT-based Tic-tac-toe game
func NewTicTacToe(id string) *TicTacToe {
	doc := rdoc.Init(id)
	
	// Initialize the game with all required fields in a single patch
	initPatch := []byte(`[
		{"op": "add", "path": "/", "value": {}},
		{"op": "add", "path": "/board", "value": [["","",""],["","",""],["","",""]]},
		{"op": "add", "path": "/players", "value": {}},
		{"op": "add", "path": "/state", "value": "waiting"},
		{"op": "add", "path": "/result", "value": "none"},
		{"op": "add", "path": "/winner", "value": ""},
		{"op": "add", "path": "/turn", "value": ""}
	]`)
	
	err := doc.Apply(initPatch)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize CRDT document: %v", err))
	}

	return &TicTacToe{
		doc:  doc,
		id:   id,
		data: make(map[string]interface{}),
	}
}

// AddPlayer adds a player to the game
func (t *TicTacToe) AddPlayer(playerID, playerName, mark string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	player := map[string]interface{}{
		"id":    playerID,
		"name":  playerName,
		"mark":  mark,
		"ready": false,
	}

	playerJSON, err := json.Marshal(player)
	if err != nil {
		return fmt.Errorf("failed to marshal player: %w", err)
	}

	patch := []byte(fmt.Sprintf(`[{"op": "add", "path": "/players/%s", "value": %s}]`, playerID, playerJSON))
	err = t.doc.Apply(patch)
	if err != nil {
		return fmt.Errorf("failed to add player: %w", err)
	}

	return nil
}

// SetPlayerReady sets a player's ready status
func (t *TicTacToe) SetPlayerReady(playerID string, ready bool) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	patch := []byte(fmt.Sprintf(`[{"op": "replace", "path": "/players/%s/ready", "value": %t}]`, playerID, ready))
	err := t.doc.Apply(patch)
	if err != nil {
		return fmt.Errorf("failed to set player ready: %w", err)
	}

	return nil
}

// SetState sets the game state
func (t *TicTacToe) SetState(state string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	patch := []byte(fmt.Sprintf(`[{"op": "replace", "path": "/state", "value": "%s"}]`, state))
	err := t.doc.Apply(patch)
	if err != nil {
		return fmt.Errorf("failed to set state: %w", err)
	}

	return nil
}

// SetTurn sets the player whose turn it is
func (t *TicTacToe) SetTurn(playerID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	patch := []byte(fmt.Sprintf(`[{"op": "replace", "path": "/turn", "value": "%s"}]`, playerID))
	err := t.doc.Apply(patch)
	if err != nil {
		return fmt.Errorf("failed to set turn: %w", err)
	}

	return nil
}

// MakeMove makes a move on the board
func (t *TicTacToe) MakeMove(playerID string, row, col int, mark string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	patch := []byte(fmt.Sprintf(`[{"op": "replace", "path": "/board/%d/%d", "value": "%s"}]`, row, col, mark))
	err := t.doc.Apply(patch)
	if err != nil {
		return fmt.Errorf("failed to make move: %w", err)
	}

	return nil
}

// SetResult sets the game result
func (t *TicTacToe) SetResult(result, winner string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	patches := []byte(fmt.Sprintf(`[
		{"op": "replace", "path": "/result", "value": "%s"},
		{"op": "replace", "path": "/winner", "value": "%s"}
	]`, result, winner))
	
	err := t.doc.Apply(patches)
	if err != nil {
		return fmt.Errorf("failed to set result: %w", err)
	}

	return nil
}

// GetState returns the current state of the game
func (t *TicTacToe) GetState() (map[string]interface{}, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	data, err := t.doc.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal document: %w", err)
	}

	var state map[string]interface{}
	err = json.Unmarshal(data, &state)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal document: %w", err)
	}

	return state, nil
}

// GetOperations returns the operations that have been applied to the document
func (t *TicTacToe) GetOperations() ([]byte, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	ops, err := t.doc.Operations()
	if err != nil {
		return nil, fmt.Errorf("failed to get operations: %w", err)
	}

	return ops, nil
}

// ApplyOperations applies operations from another replica
func (t *TicTacToe) ApplyOperations(ops []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	err := t.doc.Apply(ops)
	if err != nil {
		return fmt.Errorf("failed to apply operations: %w", err)
	}

	return nil
}
