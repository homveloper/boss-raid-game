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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"

	"nodestorage"
	"nodestorage/cache"
	"nodestorage/core/nstlog"
)

// runMultipleSimulations runs multiple simulations and compares the results
func runMultipleSimulations() {
	// 로거 설정 - 함수 위치 표시 활성화
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
		// MongoDB 연결 종료
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

	// 여러 번의 시뮬레이션 실행
	numSimulations := 3
	results := make([]int, numSimulations)

	for sim := 0; sim < numSimulations; sim++ {
		// 시뮬레이션 시작 메시지
		nstlog.Info("Starting simulation", zap.Int("simulation", sim+1), zap.Int("total", numSimulations))

		// 이전 데이터 삭제
		_, err := collection.DeleteMany(ctx, bson.M{})
		if err != nil {
			nstlog.Fatal("Failed to clear collection", zap.Error(err))
			return
		}

		// Create storage options
		options := nodestorage.DefaultOptions()
		options.VersionField = "version"

		// Create cache with options
		cacheStorage := cache.NewMapCache[*UserInventory](
			cache.WithMapMaxSize(10000),
			cache.WithMapDefaultTTL(time.Hour),
			cache.WithMapEvictionInterval(time.Minute*5),
		)

		// Create storage
		storage, err := nodestorage.NewStorage[*UserInventory](ctx, client, collection, cacheStorage, options)
		if err != nil {
			nstlog.Fatal("Failed to create storage", zap.Error(err))
			return
		}

		// Create a user inventory
		inventory := &UserInventory{
			UserID: fmt.Sprintf("user%d", sim+1),
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
		createDoc, err := storage.CreateAndGet(ctx, inventory)
		if err != nil {
			nstlog.Fatal("Failed to create document", zap.Error(err))
			return
		}
		nstlog.Debug("Created inventory", zap.String("userID", inventory.UserID), zap.String("docID", createDoc.ID.Hex()))

		// Start multiple watchers to demonstrate individual channels
		var watchCancels []context.CancelFunc
		for i := 0; i < 2; i++ {
			watcherID := i + 1

			// Create a new context for this watcher
			watchCtx, watchCancel := context.WithCancel(ctx)
			watchCancels = append(watchCancels, watchCancel)

			// Start watching for changes
			watchChan, err := storage.Watch(watchCtx)
			if err != nil {
				nstlog.Fatal("Failed to start watching", zap.Error(err))
				return
			}

			// Handle watch events in a goroutine
			go func(id int, ch <-chan nodestorage.WatchEvent[*UserInventory]) {
				for event := range ch {
					nstlog.Debug("Watcher event", zap.Int("watcherID", id), zap.String("operation", event.Operation), zap.String("documentID", event.ID.String()))
					if event.Diff != nil {
						nstlog.Debug("Watcher %d - Changes", zap.Int("watcherID", id), zap.Int("changes", len(event.Diff.Operations)))
					}
				}
				nstlog.Debug("Watcher channel closed", zap.Int("watcherID", id))
			}(watcherID, watchChan)
		}

		// Simulate multiple nodes updating the same document
		var wg sync.WaitGroup

		// 노드 ID는 1부터 N까지 랜덤

		for i := 0; i < 10; i++ {
			wg.Add(1)
			nodeID := i + 1
			go func(id int) {
				defer wg.Done()
				simulateNodeMulti(ctx, storage, createDoc.ID, id)
			}(nodeID)
		}

		wg.Wait()
		nstlog.Debug("All nodes completed", zap.Int("simulation", sim+1))

		// Get final document
		finalInventory, err := storage.Get(ctx, createDoc.ID)
		if err != nil {
			nstlog.Fatal("Failed to get document", zap.Error(err))
			return
		}

		// 결과 저장
		results[sim] = finalInventory.Gold

		nstlog.Info("Simulation completed",
			zap.Int("simulation", sim+1),
			zap.Int("gold", finalInventory.Gold),
			zap.Int("items", len(finalInventory.Items)))

		for _, item := range finalInventory.Items {
			nstlog.Debug("Final inventory item", zap.String("itemName", item.Name), zap.Int("quantity", item.Quantity))
		}

		// 워처 종료
		for _, cancel := range watchCancels {
			cancel()
		}

		// 스토리지 종료
		storage.Close()

		// 잠시 대기하여 리소스가 정리되도록 함
		time.Sleep(500 * time.Millisecond)
	}

	// 결과 비교
	nstlog.Info("Simulation results summary")
	allEqual := true
	for i := 1; i < len(results); i++ {
		if results[i] != results[0] {
			allEqual = false
			break
		}
	}

	if allEqual {
		nstlog.Info("All simulations produced the same gold amount", zap.Int("gold", results[0]))
	} else {
		nstlog.Warn("Simulations produced different gold amounts")
		for i, gold := range results {
			nstlog.Info("Simulation result", zap.Int("simulation", i+1), zap.Int("gold", gold))
		}
	}

	// 예상 골드 계산
	expectedGold := 1000 // 초기 골드
	for i := 1; i <= 10; i++ {
		expectedGold += 100 * i
	}

	nstlog.Info("Expected gold amount", zap.Int("expected", expectedGold))

	// 모든 시뮬레이션이 예상 골드와 일치하는지 확인
	allMatchExpected := true
	for _, gold := range results {
		if gold != expectedGold {
			allMatchExpected = false
			break
		}
	}

	if allMatchExpected {
		nstlog.Info("All simulations match the expected gold amount")
	} else {
		nstlog.Warn("Some simulations do not match the expected gold amount")
	}
}

// simulateNodeMulti simulates a node updating the document
func simulateNodeMulti(ctx context.Context, storage nodestorage.Storage[*UserInventory], docID primitive.ObjectID, nodeID int) {
	nstlog.Debug("simulateNodeMulti called", zap.Int("nodeID", nodeID))

	for i := nodeID; i < nodeID+3; i++ {
		// Add some delay to simulate real-world conditions
		time.Sleep(time.Duration(100+nodeID*50) * time.Millisecond)

		// Edit the document with options
		nstlog.Debug("Node %d : Before Edit", zap.Int("nodeID", nodeID), zap.Int("i", i))
		updatedInventory, diff, err := storage.Edit(ctx, docID, func(inventory *UserInventory) (*UserInventory, error) {
			// Simulate different operations
			nstlog.Debug("Node %d : Inside Edit", zap.Int("nodeID", nodeID), zap.Int("i", i), zap.Int("gold", inventory.Gold))
			switch i % 3 {
			case 0:
				// Add gold
				inventory.Gold += 100 * nodeID
				nstlog.Debug("Node %d : Added gold", zap.Int("nodeID", nodeID), zap.Int("gold", 100*nodeID), zap.Int("newGold", inventory.Gold))

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
			zap.Int("currentGold", updatedInventory.Gold))
	}
}
