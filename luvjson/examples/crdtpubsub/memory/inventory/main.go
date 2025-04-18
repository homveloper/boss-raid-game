package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
	"tictactoe/luvjson/crdtpatch"
	"tictactoe/luvjson/crdtpubsub"
	"tictactoe/luvjson/crdtpubsub/memory"

	"github.com/google/uuid"
)

// Item represents an inventory item
type Item struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Type        string    `json:"type"`
	Rarity      string    `json:"rarity"`
	Quantity    int       `json:"quantity"`
	AcquiredAt  time.Time `json:"acquiredAt"`
}

// Inventory represents a player's inventory
type Inventory struct {
	UserID    string          `json:"userId"`
	Username  string          `json:"username"`
	Capacity  int             `json:"capacity"`
	Gold      int             `json:"gold"`
	Items     map[string]Item `json:"items"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

func main() {
	// Note: This example currently has issues with the new SessionID format
	// The SessionID is now encoded as a byte array instead of a string
	// This causes JSON encoding/decoding issues
	// Create a context
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a new Memory PubSub
	pubsub, err := memory.NewPubSub()
	if err != nil {
		fmt.Printf("Failed to create Memory PubSub: %v\n", err)
		return
	}
	defer pubsub.Close()

	// Create a CRDT document for the inventory
	doc := crdt.NewDocument(common.NewSessionID())

	// Create a CRDT tracker
	tracker := crdtpubsub.NewTracker(doc)

	// Subscribe to inventory updates
	topic := "inventory-updates"
	subscriberID := "inventory-subscriber"
	if err := pubsub.Subscribe(ctx, topic, subscriberID, func(ctx context.Context, topic string, data []byte, format crdtpubsub.EncodingFormat) error {
		// Decode the patch
		encoderDecoder, err := crdtpubsub.GetEncoderDecoder(format)
		if err != nil {
			fmt.Printf("Failed to get encoder/decoder: %v\n", err)
			return err
		}

		receivedPatch, err := encoderDecoder.Decode(data)
		if err != nil {
			fmt.Printf("Failed to decode patch: %v\n", err)
			return err
		}

		// Apply the patch to the document
		if err := tracker.ApplyPatch(receivedPatch); err != nil {
			fmt.Printf("Failed to apply patch: %v\n", err)
			return err
		}

		// Get the current inventory
		inventoryValue, err := doc.View()
		if err != nil {
			fmt.Printf("Failed to get inventory: %v\n", err)
			return err
		}

		// Convert to JSON for display
		inventoryJSON, err := json.MarshalIndent(inventoryValue, "", "  ")
		if err != nil {
			fmt.Printf("Failed to marshal inventory: %v\n", err)
			return err
		}

		fmt.Printf("Received inventory update:\n%s\n\n", string(inventoryJSON))
		return nil
	}); err != nil {
		fmt.Printf("Failed to subscribe to topic: %v\n", err)
		return
	}

	// Create initial inventory
	inventory := Inventory{
		UserID:    "user123",
		Username:  "GameMaster",
		Capacity:  20,
		Gold:      1000,
		Items:     make(map[string]Item),
		UpdatedAt: time.Now(),
	}

	// Add some initial items
	inventory.Items["potion1"] = Item{
		ID:          "potion1",
		Name:        "Health Potion",
		Description: "Restores 50 HP",
		Type:        "consumable",
		Rarity:      "common",
		Quantity:    5,
		AcquiredAt:  time.Now(),
	}

	// Create a patch with the inventory
	uuidVal, err := uuid.NewV7()
	if err != nil {
		fmt.Printf("Failed to create UUID: %v\n", err)
		return
	}
	sid := common.SessionID(uuidVal)
	id := common.LogicalTimestamp{SID: sid, Counter: 1}
	patch := crdtpatch.NewPatch(id)

	// Create a new constant node operation with the inventory
	op := &crdtpatch.NewOperation{
		ID:       id,
		NodeType: common.NodeTypeCon,
		Value:    inventory,
	}
	patch.AddOperation(op)

	// Publish the patch
	fmt.Println("Publishing initial inventory...")
	if err := pubsub.Publish(ctx, topic, patch, crdtpubsub.EncodingFormatJSON); err != nil {
		fmt.Printf("Failed to publish patch: %v\n", err)
		return
	}

	// Wait a bit
	time.Sleep(1 * time.Second)

	// Update the inventory - add a new item
	inventory.Items["sword1"] = Item{
		ID:          "sword1",
		Name:        "Iron Sword",
		Description: "A basic iron sword",
		Type:        "weapon",
		Rarity:      "uncommon",
		Quantity:    1,
		AcquiredAt:  time.Now(),
	}
	inventory.Gold = 800 // Spent 200 gold
	inventory.UpdatedAt = time.Now()

	// Create a new patch with the updated inventory
	id2 := common.LogicalTimestamp{SID: sid, Counter: 2}
	patch2 := crdtpatch.NewPatch(id2)

	// Create a new constant node operation with the updated inventory
	op2 := &crdtpatch.NewOperation{
		ID:       id2,
		NodeType: common.NodeTypeCon,
		Value:    inventory,
	}
	patch2.AddOperation(op2)

	// Publish the updated patch
	fmt.Println("Publishing updated inventory (added sword, spent gold)...")
	if err := pubsub.Publish(ctx, topic, patch2, crdtpubsub.EncodingFormatJSON); err != nil {
		fmt.Printf("Failed to publish patch: %v\n", err)
		return
	}

	// Wait a bit
	time.Sleep(1 * time.Second)

	// Update the inventory again - sell the sword, get more gold
	delete(inventory.Items, "sword1")
	inventory.Gold = 1100 // 800 + 300 (sold the sword)
	inventory.UpdatedAt = time.Now()

	// Create a new patch with the final inventory
	id3 := common.LogicalTimestamp{SID: sid, Counter: 3}
	patch3 := crdtpatch.NewPatch(id3)

	// Create a new constant node operation with the final inventory
	op3 := &crdtpatch.NewOperation{
		ID:       id3,
		NodeType: common.NodeTypeCon,
		Value:    inventory,
	}
	patch3.AddOperation(op3)

	// Publish the final patch
	fmt.Println("Publishing final inventory (sold sword, gained gold)...")
	if err := pubsub.Publish(ctx, topic, patch3, crdtpubsub.EncodingFormatJSON); err != nil {
		fmt.Printf("Failed to publish patch: %v\n", err)
		return
	}

	// Wait a bit before exiting
	time.Sleep(1 * time.Second)
	fmt.Println("Example completed successfully.")
}
