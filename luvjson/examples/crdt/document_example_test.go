package crdt_test

import (
	"fmt"
	"testing"
	"time"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
	"tictactoe/luvjson/crdtpatch"

	"github.com/stretchr/testify/assert"
)

// TestDocument_TextFields demonstrates how to add and modify text fields in a CRDT document
func TestDocument_TextFields(t *testing.T) {
	// Create a new document with a new session ID
	sessionID := common.NewSessionID()
	doc := crdt.NewDocument(sessionID)

	// Create a patch builder to help with creating operations
	builder := crdtpatch.NewPatchBuilder(sessionID, 1)

	// Create a constant node with our data
	rootOp := builder.NewConstant(map[string]interface{}{
		"username":    "player123",
		"description": "A new player",
		"status":      "online",
	})

	// Create a patch to set the root value to our constant node
	rootSetOp := &crdtpatch.InsOperation{
		ID:       builder.NextTimestamp(),
		TargetID: common.RootID,
		Value:    rootOp.ID,
	}
	builder.AddOperation(rootSetOp)

	// Create and apply the patch
	patch := builder.Flush()
	err := patch.Apply(doc)
	assert.NoError(t, err)

	// Get the document view
	view, err := doc.View()
	assert.NoError(t, err)

	// Check if view is a map
	viewMap, ok := view.(map[string]interface{})
	assert.True(t, ok, "View should be a map[string]interface{}")

	// Verify the fields
	assert.Equal(t, "player123", viewMap["username"])
	assert.Equal(t, "A new player", viewMap["description"])
	assert.Equal(t, "online", viewMap["status"])

	// Create a new builder for updating fields
	updateBuilder := crdtpatch.NewPatchBuilder(sessionID, 100)

	// Get the current view
	currentView, err := doc.View()
	assert.NoError(t, err)
	currentViewMap, ok := currentView.(map[string]interface{})
	assert.True(t, ok, "View should be a map[string]interface{}")

	// Update the map with new values
	currentViewMap["username"] = "player123_updated"
	currentViewMap["description"] = "An experienced player"
	currentViewMap["status"] = "away"

	// Create a new constant node with updated data
	updateRootOp := updateBuilder.NewConstant(currentViewMap)

	// Create a patch to set the root value to our updated constant node
	updateRootSetOp := &crdtpatch.InsOperation{
		ID:       updateBuilder.NextTimestamp(),
		TargetID: common.RootID,
		Value:    updateRootOp.ID,
	}
	updateBuilder.AddOperation(updateRootSetOp)

	// Create and apply the update patch
	updatePatch := updateBuilder.Flush()
	err = updatePatch.Apply(doc)
	assert.NoError(t, err)

	// Get the updated view
	updatedView, err := doc.View()
	assert.NoError(t, err)
	updatedViewMap, ok := updatedView.(map[string]interface{})
	assert.True(t, ok, "Updated view should be a map[string]interface{}")

	// Verify the updated fields
	assert.Equal(t, "player123_updated", updatedViewMap["username"])
	assert.Equal(t, "An experienced player", updatedViewMap["description"])
	assert.Equal(t, "away", updatedViewMap["status"])

	// Print the document state
	fmt.Println("Document with text fields:")
	fmt.Printf("Username: %s\n", updatedViewMap["username"])
	fmt.Printf("Description: %s\n", updatedViewMap["description"])
	fmt.Printf("Status: %s\n", updatedViewMap["status"])
}

// TestDocument_RankingScores demonstrates how to add and increment ranking scores in a CRDT document
func TestDocument_RankingScores(t *testing.T) {
	// Create a new document with a new session ID
	sessionID := common.NewSessionID()
	doc := crdt.NewDocument(sessionID)

	// Create a patch builder to help with creating operations
	builder := crdtpatch.NewPatchBuilder(sessionID, 1)

	// Create initial data
	gameModeScores := map[string]interface{}{
		"deathmatch": 350,
		"capture":    275,
		"survival":   375,
	}

	// Create a constant node with our data
	rootOp := builder.NewConstant(map[string]interface{}{
		"playerName":     "GamerX",
		"totalScore":     1000,
		"weeklyRank":     50,
		"lastUpdated":    time.Now().Format(time.RFC3339),
		"gameModeScores": gameModeScores,
	})

	// Create a patch to set the root value to our constant node
	rootSetOp := &crdtpatch.InsOperation{
		ID:       builder.NextTimestamp(),
		TargetID: common.RootID,
		Value:    rootOp.ID,
	}
	builder.AddOperation(rootSetOp)

	// Create and apply the patch
	patch := builder.Flush()
	err := patch.Apply(doc)
	assert.NoError(t, err)

	// Get the document view
	view, err := doc.View()
	assert.NoError(t, err)
	viewMap, ok := view.(map[string]interface{})
	assert.True(t, ok, "View should be a map[string]interface{}")

	// Verify the initial scores
	assert.Equal(t, "GamerX", viewMap["playerName"])
	assert.Equal(t, float64(1000), viewMap["totalScore"])
	assert.Equal(t, float64(50), viewMap["weeklyRank"])

	// Create a new builder for updating scores
	updateBuilder := crdtpatch.NewPatchBuilder(sessionID, 100)

	// Get the current view
	currentView, err := doc.View()
	assert.NoError(t, err)
	currentViewMap, ok := currentView.(map[string]interface{})
	assert.True(t, ok, "View should be a map[string]interface{}")

	// Increment scores (by creating new values)
	currentScore := currentViewMap["totalScore"].(float64)
	currentViewMap["totalScore"] = currentScore + 250

	currentRank := currentViewMap["weeklyRank"].(float64)
	currentViewMap["weeklyRank"] = currentRank - 10 // Lower rank number is better

	// Update timestamp
	currentViewMap["lastUpdated"] = time.Now().Format(time.RFC3339)

	// Update a specific game mode score
	gameModeScoresMap := currentViewMap["gameModeScores"].(map[string]interface{})
	deathmatchScore := gameModeScoresMap["deathmatch"].(float64)

	// Create a new map with updated score
	updatedGameModeScores := map[string]interface{}{
		"deathmatch": deathmatchScore + 50,
		"capture":    gameModeScoresMap["capture"],
		"survival":   gameModeScoresMap["survival"],
	}
	currentViewMap["gameModeScores"] = updatedGameModeScores

	// Create a new constant node with updated data
	updateRootOp := updateBuilder.NewConstant(currentViewMap)

	// Create a patch to set the root value to our updated constant node
	updateRootSetOp := &crdtpatch.InsOperation{
		ID:       updateBuilder.NextTimestamp(),
		TargetID: common.RootID,
		Value:    updateRootOp.ID,
	}
	updateBuilder.AddOperation(updateRootSetOp)

	// Create and apply the update patch
	updatePatch := updateBuilder.Flush()
	err = updatePatch.Apply(doc)
	assert.NoError(t, err)

	// Get the updated view
	updatedView, err := doc.View()
	assert.NoError(t, err)
	updatedViewMap, ok := updatedView.(map[string]interface{})
	assert.True(t, ok, "Updated view should be a map[string]interface{}")

	// Verify the updated scores
	assert.Equal(t, float64(1250), updatedViewMap["totalScore"])
	assert.Equal(t, float64(40), updatedViewMap["weeklyRank"])

	updatedGameModeScoresMap, ok := updatedViewMap["gameModeScores"].(map[string]interface{})
	assert.True(t, ok, "Game mode scores should be a map[string]interface{}")
	assert.Equal(t, float64(400), updatedGameModeScoresMap["deathmatch"])

	// Print the document state
	fmt.Println("\nDocument with ranking scores:")
	fmt.Printf("Player: %s\n", updatedViewMap["playerName"])
	fmt.Printf("Total Score: %.0f\n", updatedViewMap["totalScore"])
	fmt.Printf("Weekly Rank: %.0f\n", updatedViewMap["weeklyRank"])
	fmt.Printf("Last Updated: %s\n", updatedViewMap["lastUpdated"])
	fmt.Println("Game Mode Scores:")
	for mode, score := range updatedGameModeScoresMap {
		fmt.Printf("  %s: %.0f\n", mode, score.(float64))
	}
}

// TestDocument_ItemQuantities demonstrates how to add and modify item quantities in a CRDT document
func TestDocument_ItemQuantities(t *testing.T) {
	// Create a new document with a new session ID
	sessionID := common.NewSessionID()
	doc := crdt.NewDocument(sessionID)

	// Create a patch builder to help with creating operations
	builder := crdtpatch.NewPatchBuilder(sessionID, 1)

	// Create a root object to hold our inventory
	rootObjOp := builder.NewObject()

	// Add player info
	builder.InsertObjectField(rootObjOp.ID, "playerName", "Adventurer")

	// Create an inventory object
	inventoryObjOp := builder.NewObject()
	builder.InsertObjectField(rootObjOp.ID, "inventory", inventoryObjOp.ID)

	// Add initial items with quantities
	builder.InsertObjectField(inventoryObjOp.ID, "gold", 1000)
	builder.InsertObjectField(inventoryObjOp.ID, "healthPotion", 5)
	builder.InsertObjectField(inventoryObjOp.ID, "manaPotion", 3)
	builder.InsertObjectField(inventoryObjOp.ID, "sword", 1)

	// Create a constant node for the root value
	rootValueOp := builder.NewConstant(rootObjOp.ID)

	// Create a patch to set the root value to our object
	rootSetOp := &crdtpatch.InsOperation{
		ID:       builder.NextTimestamp(),
		TargetID: common.RootID,
		Value:    rootValueOp.ID,
	}
	builder.AddOperation(rootSetOp)

	// Create and apply the patch
	patch := builder.Flush()
	err := patch.Apply(doc)
	assert.NoError(t, err)

	// Get the document view
	view, err := doc.View()
	assert.NoError(t, err)
	viewMap, ok := view.(map[string]interface{})
	assert.True(t, ok, "View should be a map[string]interface{}")

	// Verify the initial inventory
	inventoryMap, ok := viewMap["inventory"].(map[string]interface{})
	assert.True(t, ok, "Inventory should be a map[string]interface{}")
	assert.Equal(t, float64(1000), inventoryMap["gold"])
	assert.Equal(t, float64(5), inventoryMap["healthPotion"])
	assert.Equal(t, float64(3), inventoryMap["manaPotion"])
	assert.Equal(t, float64(1), inventoryMap["sword"])

	// Create a new builder for updating inventory
	updateBuilder := crdtpatch.NewPatchBuilder(sessionID, 100)

	// Get the inventory node
	_, err = doc.GetNode(inventoryObjOp.ID)
	assert.NoError(t, err)

	// Update item quantities
	// Spend gold
	goldAmount := inventoryMap["gold"].(float64)
	updateBuilder.InsertObjectField(inventoryObjOp.ID, "gold", goldAmount-150)

	// Use a health potion
	healthPotionCount := inventoryMap["healthPotion"].(float64)
	updateBuilder.InsertObjectField(inventoryObjOp.ID, "healthPotion", healthPotionCount-1)

	// Add a new item
	updateBuilder.InsertObjectField(inventoryObjOp.ID, "magicScroll", 2)

	// Create and apply the update patch
	updatePatch := updateBuilder.Flush()
	err = updatePatch.Apply(doc)
	assert.NoError(t, err)

	// Get the updated view
	updatedView, err := doc.View()
	assert.NoError(t, err)
	updatedViewMap, ok := updatedView.(map[string]interface{})
	assert.True(t, ok, "Updated view should be a map[string]interface{}")

	updatedInventoryMap, ok := updatedViewMap["inventory"].(map[string]interface{})
	assert.True(t, ok, "Updated inventory should be a map[string]interface{}")

	// Verify the updated inventory
	assert.Equal(t, float64(850), updatedInventoryMap["gold"])
	assert.Equal(t, float64(4), updatedInventoryMap["healthPotion"])
	assert.Equal(t, float64(3), updatedInventoryMap["manaPotion"])  // Unchanged
	assert.Equal(t, float64(2), updatedInventoryMap["magicScroll"]) // New item

	// Print the document state
	fmt.Println("\nDocument with item quantities:")
	fmt.Printf("Player: %s\n", updatedViewMap["playerName"])
	fmt.Println("Inventory:")
	for item, quantity := range updatedInventoryMap {
		fmt.Printf("  %s: %.0f\n", item, quantity.(float64))
	}
}
