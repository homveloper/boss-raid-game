package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"time"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdtpatch"
	"tictactoe/luvjson/crdtpubsub"
)

// Item represents a game item in the inventory
type Item struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Type        string    `json:"type"`
	Rarity      string    `json:"rarity"`
	Quantity    int       `json:"quantity"`
	AcquiredAt  time.Time `json:"acquiredAt"`
}

// UserInventory represents a user's inventory
type UserInventory struct {
	UserID    string          `json:"userId"`
	Username  string          `json:"username"`
	Items     map[string]Item `json:"items"`
	Gold      int             `json:"gold"`
	Capacity  int             `json:"capacity"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

func main() {
	// Create a context that will be canceled when Ctrl+C is pressed
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt)
	go func() {
		<-signalCh
		fmt.Println("\nReceived interrupt signal. Shutting down...")
		cancel()
	}()

	// Create Memory PubSub options
	options := crdtpubsub.NewOptions()
	options.DefaultFormat = crdtpubsub.EncodingFormatJSON

	// Create Memory PubSub
	memoryPubSub, err := crdtpubsub.NewMemoryPubSub(options)
	if err != nil {
		fmt.Printf("Failed to create Memory PubSub: %v\n", err)
		return
	}
	defer memoryPubSub.Close()

	// Create a wait group to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Start publisher
	wg.Add(1)
	go func() {
		defer wg.Done()
		runInventoryPublisher(ctx, memoryPubSub)
	}()

	// Start subscriber
	wg.Add(1)
	go func() {
		defer wg.Done()
		runInventorySubscriber(ctx, memoryPubSub)
	}()

	// Wait for all goroutines to finish
	wg.Wait()
	fmt.Println("Shutdown complete")
}

func runInventoryPublisher(ctx context.Context, pubsub crdtpubsub.PubSub) {
	// Create an initial inventory
	inventory := createInitialInventory()

	// Print the initial inventory
	inventoryJSON, _ := json.MarshalIndent(inventory, "", "  ")
	fmt.Printf("Initial inventory:\n%s\n", string(inventoryJSON))

	// Create a ticker to publish updates periodically
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	// Counter for updates
	updateCounter := 0

	// Topic for inventory updates
	topic := "game-inventory"

	// Publish the initial inventory
	publishInventory(ctx, pubsub, topic, inventory, updateCounter)
	updateCounter++

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Update the inventory
			updateInventory(inventory, updateCounter)

			// Publish the updated inventory
			publishInventory(ctx, pubsub, topic, inventory, updateCounter)

			// Increment the counter
			updateCounter++
		}
	}
}

func runInventorySubscriber(ctx context.Context, pubsub crdtpubsub.PubSub) {
	// Topic for inventory updates
	topic := "game-inventory"

	// Subscribe to the topic
	fmt.Printf("Subscribing to topic %s\n", topic)
	if err := pubsub.Subscribe(ctx, topic, func(msg crdtpubsub.PatchMessage) error {
		fmt.Printf("Received inventory update from topic %s with format %s\n", msg.Topic, msg.Format)

		// Decode the patch
		decoder, err := crdtpubsub.GetEncoderDecoder(msg.Format)
		if err != nil {
			return fmt.Errorf("failed to get decoder: %w", err)
		}

		patch, err := decoder.Decode(msg.Payload)
		if err != nil {
			return fmt.Errorf("failed to decode patch: %w", err)
		}

		// Extract the inventory from the patch
		// In a real application, you would apply the patch to a document
		// Here we just extract the value for demonstration
		operations := patch.Operations()
		if len(operations) > 0 {
			op, ok := operations[0].(*crdtpatch.NewOperation)
			if ok && op.Value != nil {
				// Convert the value to JSON for display
				valueJSON, _ := json.MarshalIndent(op.Value, "", "  ")
				fmt.Printf("Received inventory:\n%s\n", string(valueJSON))
			}
		}

		return nil
	}); err != nil {
		fmt.Printf("Failed to subscribe to topic: %v\n", err)
		return
	}

	// Wait for context cancellation
	<-ctx.Done()

	// Unsubscribe from the topic
	unsubCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pubsub.Unsubscribe(unsubCtx, topic); err != nil {
		fmt.Printf("Failed to unsubscribe from topic: %v\n", err)
	}
}

func createInitialInventory() map[string]interface{} {
	// Create an inventory as a map
	inventory := map[string]interface{}{
		"userId":    "user123",
		"username":  "GameMaster",
		"gold":      1000,
		"capacity":  20,
		"updatedAt": time.Now().Format(time.RFC3339),
		"items": map[string]interface{}{
			"sword1": map[string]interface{}{
				"id":          "sword1",
				"name":        "Iron Sword",
				"description": "A basic iron sword",
				"type":        "weapon",
				"rarity":      "common",
				"quantity":    1,
				"acquiredAt":  time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
			},
			"potion1": map[string]interface{}{
				"id":          "potion1",
				"name":        "Health Potion",
				"description": "Restores 50 HP",
				"type":        "consumable",
				"rarity":      "common",
				"quantity":    5,
				"acquiredAt":  time.Now().Add(-12 * time.Hour).Format(time.RFC3339),
			},
		},
	}

	return inventory
}

func updateInventory(inventory map[string]interface{}, updateCounter int) {
	// Get the items map
	items, ok := inventory["items"].(map[string]interface{})
	if !ok {
		fmt.Println("Error: items is not a map")
		return
	}

	// Update the inventory based on the counter
	switch updateCounter % 4 {
	case 0:
		// Add a new legendary item
		items["axe1"] = map[string]interface{}{
			"id":          "axe1",
			"name":        "Thunderfury, Blessed Axe",
			"description": "A legendary axe that crackles with lightning",
			"type":        "weapon",
			"rarity":      "legendary",
			"quantity":    1,
			"acquiredAt":  time.Now().Format(time.RFC3339),
		}
		fmt.Println("Added a legendary axe to the inventory")
	case 1:
		// Use some health potions
		potion, ok := items["potion1"].(map[string]interface{})
		if ok {
			quantity, ok := potion["quantity"].(int)
			if ok && quantity > 2 {
				potion["quantity"] = quantity - 2
				fmt.Println("Used 2 health potions")
			}
		}
	case 2:
		// Increase gold (found some treasure)
		if gold, ok := inventory["gold"].(int); ok {
			inventory["gold"] = gold + 500
			fmt.Println("Found 500 gold")
		}
	case 3:
		// Sell an item
		if _, ok := items["sword1"]; ok {
			delete(items, "sword1")
			if gold, ok := inventory["gold"].(int); ok {
				inventory["gold"] = gold + 100
			}
			fmt.Println("Sold the iron sword for 100 gold")
		}
	}

	// Update the timestamp
	inventory["updatedAt"] = time.Now().Format(time.RFC3339)
}

func publishInventory(ctx context.Context, pubsub crdtpubsub.PubSub, topic string, inventory map[string]interface{}, counter int) {
	// Create a patch with the inventory
	id := common.LogicalTimestamp{SID: 1, Counter: uint64(counter + 1)}
	patch := crdtpatch.NewPatch(id)

	// Create a new constant node operation with the inventory
	op := &crdtpatch.NewOperation{
		ID:       id,
		NodeType: common.NodeTypeCon,
		Value:    inventory,
	}
	patch.AddOperation(op)

	// Publish the patch
	fmt.Printf("Publishing inventory update #%d to topic %s\n", counter+1, topic)
	if err := pubsub.Publish(ctx, topic, patch, crdtpubsub.EncodingFormatJSON); err != nil {
		fmt.Printf("Failed to publish inventory: %v\n", err)
	}
}
