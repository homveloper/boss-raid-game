package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"boss-raid-game/nodestorage"
	"boss-raid-game/nodestorage/cache"
)

// UserInventory implements the Cachable interface
type UserInventory struct {
	UserID    string    `bson:"user_id" json:"user_id"`
	Items     []Item    `bson:"items" json:"items"`
	Gold      int       `bson:"gold" json:"gold"`
	LastLogin time.Time `bson:"last_login" json:"last_login"`
	Version_  int64     `bson:"version" json:"-"`
}

// Item represents an inventory item
type Item struct {
	ID       string    `bson:"id" json:"id"`
	Name     string    `bson:"name" json:"name"`
	Type     string    `bson:"type" json:"type"`
	Quantity int       `bson:"quantity" json:"quantity"`
	AddedAt  time.Time `bson:"added_at" json:"added_at"`
}

// Copy creates a deep copy of the inventory
func (i UserInventory) Copy() UserInventory {
	newInventory := UserInventory{
		UserID:    i.UserID,
		Gold:      i.Gold,
		LastLogin: i.LastLogin,
		Version_:  i.Version_,
	}

	// Deep copy items
	newInventory.Items = make([]Item, len(i.Items))
	for j, item := range i.Items {
		newInventory.Items[j] = item
	}

	return newInventory
}

// Version gets or sets the version
func (i UserInventory) Version(v ...int64) int64 {
	if len(v) > 0 {
		i.Version_ = v[0]
	}
	return i.Version_
}

func main() {
	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("Shutting down...")
		cancel()
	}()

	client, err := mongo.Connect(options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	defer func() {
		if err := client.Disconnect(ctx); err != nil {
			log.Fatalf("Failed to disconnect from MongoDB: %v", err)
		}
	}()

	collection := client.Database("mydb").Collection("user_inventories")

	// Create storage options
	options := nodestorage.DefaultOptions()

	// Create cache with options
	cacheStorage := cache.NewMapCache[UserInventory](
		cache.WithMapMaxSize(10000),
		cache.WithMapDefaultTTL(time.Hour),
		cache.WithMapEvictionInterval(time.Minute*5),
	)

	// Alternatively, use BadgerDB cache
	// cacheStorage, err := cache.NewBadgerCache[UserInventory](
	// 	cache.WithBadgerPath("./badger-data"),
	// 	cache.WithBadgerDefaultTTL(time.Hour),
	// )
	// if err != nil {
	// 	log.Fatalf("Failed to create BadgerDB cache: %v", err)
	// }

	// Create storage
	storage, err := nodestorage.NewStorage[UserInventory](ctx, client, collection, cacheStorage, options)
	if err != nil {
		log.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create a user inventory
	inventory := UserInventory{
		UserID: "user123",
		Items: []Item{
			{
				ID:       "item1",
				Name:     "Health Potion",
				Type:     "consumable",
				Quantity: 5,
				AddedAt:  time.Now(),
			},
		},
		Gold:      1000,
		LastLogin: time.Now(),
	}

	// Create document
	docID, err := storage.Create(ctx, inventory)
	if err != nil {
		log.Fatalf("Failed to create document: %v", err)
	}
	fmt.Printf("Created inventory for user %s with ID %v\n", inventory.UserID, docID)

	// Start multiple watchers to demonstrate individual channels
	for i := 0; i < 2; i++ {
		watcherID := i + 1

		// Create a new context for this watcher
		watchCtx, watchCancel := context.WithCancel(ctx)
		defer watchCancel()

		// Start watching for changes
		watchChan, err := storage.Watch(watchCtx)
		if err != nil {
			log.Fatalf("Failed to watch for changes: %v", err)
		}

		// Handle watch events in a goroutine
		go func(id int, ch <-chan nodestorage.WatchEvent[UserInventory]) {
			for event := range ch {
				fmt.Printf("Watcher %d - Event: %s %v\n", id, event.Operation, event.ID)
				if event.Diff != nil {
					fmt.Printf("Watcher %d - Changes: %d operations\n", id, len(event.Diff.Operations))
				}
			}
			fmt.Printf("Watcher %d - Channel closed\n", id)
		}(watcherID, watchChan)
	}

	// Simulate multiple nodes updating the same document
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		nodeID := i + 1
		go func(id int) {
			defer wg.Done()
			simulateNode(ctx, storage, docID, id)
		}(nodeID)
	}

	wg.Wait()
	fmt.Println("All nodes completed")

	// Get final document
	finalInventory, err := storage.Get(ctx, docID)
	if err != nil {
		log.Fatalf("Failed to get document: %v", err)
	}

	fmt.Printf("\nFinal inventory for user %s:\n", finalInventory.UserID)
	fmt.Printf("  Gold: %d\n", finalInventory.Gold)
	fmt.Printf("  Items: %d\n", len(finalInventory.Items))
	for _, item := range finalInventory.Items {
		fmt.Printf("    - %s (x%d)\n", item.Name, item.Quantity)
	}
}

// simulateNode simulates a node updating the document
func simulateNode(ctx context.Context, storage nodestorage.Storage[UserInventory], docID primitive.ObjectID, nodeID int) {
	for i := 0; i < 5; i++ {
		// Add some delay to simulate real-world conditions
		time.Sleep(time.Duration(100+nodeID*50) * time.Millisecond)

		// Edit the document with options
		updatedInventory, diff, err := storage.Edit(ctx, docID, func(inventory UserInventory) (UserInventory, error) {
			// Simulate different operations
			switch i % 3 {
			case 0:
				// Add gold
				inventory.Gold += 100 * nodeID
				fmt.Printf("Node %d: Added %d gold\n", nodeID, 100*nodeID)

			case 1:
				// Add an item
				newItem := Item{
					ID:       fmt.Sprintf("item%d-%d", nodeID, i),
					Name:     fmt.Sprintf("Item from Node %d", nodeID),
					Type:     "equipment",
					Quantity: nodeID,
					AddedAt:  time.Now(),
				}
				inventory.Items = append(inventory.Items, newItem)
				fmt.Printf("Node %d: Added item %s\n", nodeID, newItem.Name)

			case 2:
				// Update existing item if any
				if len(inventory.Items) > 0 {
					itemIdx := (nodeID + i) % len(inventory.Items)
					inventory.Items[itemIdx].Quantity += nodeID
					fmt.Printf("Node %d: Updated item %s quantity to %d\n",
						nodeID, inventory.Items[itemIdx].Name, inventory.Items[itemIdx].Quantity)
				}
			}

			// Update last login
			inventory.LastLogin = time.Now()

			// Return updated inventory
			return inventory, nil
		})

		if err != nil {
			fmt.Printf("Node %d: Failed to edit document: %v\n", nodeID, err)
			continue
		}

		fmt.Printf("Node %d: Successfully edited document (version: %d, changes: %d)\n",
			nodeID, updatedInventory.Version(), len(diff.Operations))
	}
}
