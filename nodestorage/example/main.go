package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"

	"nodestorage"
	"nodestorage/cache"
	"nodestorage/core/nstlog"
)

// *UserInventory implements the Cachable interface
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
func (i *UserInventory) Copy() *UserInventory {
	newInventory := &UserInventory{
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
func (i *UserInventory) Version(v ...int64) int64 {
	if len(v) > 0 {
		i.Version_ = v[0]
	}
	return i.Version_
}

func main() {
	// 명령줄 인수 확인
	runMultiSim := false
	if len(os.Args) > 1 && os.Args[1] == "multi" {
		runMultiSim = true
	}

	// 여러 번의 시뮬레이션 실행
	if runMultiSim {
		runMultipleSimulations()
		return
	}

	// 단일 시뮬레이션 실행
	nstlog.SetLogger(true, "debug")

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

	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017/?replicaSet=rs"))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	nstlog.Debug("Connected to MongoDB")

	// 프로그램 종료 시 MongoDB 연결 종료
	defer func() {
		// MongoDB 연결 종료 (storage.Close()는 이미 defer로 호출되어 있음)
		disconnectCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := client.Disconnect(disconnectCtx); err != nil {
			// 오류 메시지를 Fatal 대신 Warning으로 출력
			if !strings.Contains(err.Error(), "client is disconnected") {
				nstlog.Warn("Failed to disconnect from MongoDB", zap.Error(err))
			}
		}
	}()

	collection := client.Database("mydb").Collection("user_inventories")

	// Create storage options
	options := nodestorage.DefaultOptions()

	// Create cache with options
	cacheStorage := cache.NewMapCache[*UserInventory](
		cache.WithMapMaxSize(10000),
		cache.WithMapDefaultTTL(time.Hour),
		cache.WithMapEvictionInterval(time.Minute*5),
	)

	// Alternatively, use BadgerDB cache
	// cacheStorage, err := cache.NewBadgerCache[*UserInventory](
	// 	cache.WithBadgerPath("./badger-data"),
	// 	cache.WithBadgerDefaultTTL(time.Hour),
	// )
	// if err != nil {
	// 	log.Fatalf("Failed to create BadgerDB cache: %v", err)
	// }

	// Create storage
	storage, err := nodestorage.NewStorage[*UserInventory](ctx, client, collection, cacheStorage, options)
	if err != nil {
		nstlog.Fatal("Failed to create storage", zap.Error(err))
		return
	}
	defer storage.Close()

	// Create a user inventory
	inventory := &UserInventory{
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
		nstlog.Fatal("Failed to create document", zap.Error(err))
		return
	}
	nstlog.Debug("Created inventory", zap.String("userID", inventory.UserID), zap.String("docID", docID.Hex()))

	// Start multiple watchers to demonstrate individual channels
	for i := 0; i < 2; i++ {
		watcherID := i + 1

		// Create a new context for this watcher
		watchCtx, watchCancel := context.WithCancel(ctx)
		defer watchCancel()

		// Start watching for changes
		watchChan, err := storage.Watch(watchCtx)
		if err != nil {
			nstlog.Fatal("Failed to start watching", zap.Error(err))
			return
		}

		// Handle watch events in a goroutine
		go func(id int, ch <-chan nodestorage.WatchEvent[*UserInventory]) {
			for event := range ch {
				nstlog.Debug("Watcher event", zap.Int("watcherID", id), zap.String("operation", event.Operation), zap.String("documentID", event.ID.Hex()))
				if event.Diff != nil {
					nstlog.Debug("Watcher %d - Changes", zap.Int("watcherID", id), zap.Int("changes", len(event.Diff.Operations)))
				}
			}
			nstlog.Debug("Watcher channel closed", zap.Int("watcherID", id))
		}(watcherID, watchChan)
	}

	// Simulate multiple nodes updating the same document
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		nodeID := i + 1
		go func(id int) {
			defer wg.Done()
			simulateNode(ctx, storage, docID, id)
		}(nodeID)
	}

	wg.Wait()
	nstlog.Debug("All nodes completed")

	// Get final document
	finalInventory, err := storage.Get(ctx, docID)
	if err != nil {
		nstlog.Fatal("Failed to get document", zap.Error(err))
		return
	}

	nstlog.Debug("Final inventory", zap.Int("gold", finalInventory.Gold), zap.Int("items", len(finalInventory.Items)))
	for _, item := range finalInventory.Items {
		nstlog.Debug("Final inventory item", zap.String("itemName", item.Name), zap.Int("quantity", item.Quantity))
	}
}

// simulateNode simulates a node updating the document
func simulateNode(ctx context.Context, storage nodestorage.Storage[*UserInventory], docID primitive.ObjectID, nodeID int) {
	for i := nodeID; i < nodeID+3; i++ {
		// Add some delay to simulate real-world conditions
		time.Sleep(time.Duration(100+nodeID*50) * time.Millisecond)

		// Edit the document with options
		updatedInventory, diff, err := storage.Edit(ctx, docID, func(inventory *UserInventory) (*UserInventory, error) {
			// Simulate different operations
			switch i % 3 {
			case 0:
				// Add gold
				inventory.Gold += 100 * nodeID
				nstlog.Debug("Node %d : Added gold", zap.Int("nodeID", nodeID), zap.Int("gold", 100*nodeID))

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
				nstlog.Debug("Node %d : Added item", zap.Int("nodeID", nodeID), zap.String("itemName", newItem.Name))

			case 2:
				// Update existing item if any
				if len(inventory.Items) > 0 {
					itemIdx := (nodeID + i) % len(inventory.Items)
					inventory.Items[itemIdx].Quantity += nodeID

					nstlog.Debug("Node %d : Updated item quantity", zap.Int("nodeID", nodeID), zap.Int("itemIdx", itemIdx), zap.Int("quantity", inventory.Items[itemIdx].Quantity))
				}
			}

			// Update last login
			inventory.LastLogin = time.Now()

			// Return updated inventory
			return inventory, nil
		})

		if err != nil {
			nstlog.Debug("Node %d : Failed to edit document", zap.Int("nodeID", nodeID), zap.Error(err))
			continue
		}

		nstlog.Debug("Node %d : Successfully edited document",
			zap.Int("nodeID", nodeID),
			zap.Int64("version", updatedInventory.Version_),
			zap.Int("changes", len(diff.Operations)),
			zap.Int("currentGold", updatedInventory.Gold),
			zap.Int("i", i),
			zap.Int("i%3", i%3))
	}
}
