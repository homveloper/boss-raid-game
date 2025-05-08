package guild_territory

import (
	"context"
	"fmt"
	"log"
	"nodestorage/v2"
	"sync"
	"time"

	"nodestorage/v2/cache"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Example demonstrates the guild territory construction system
func Example() {
	// Connect to MongoDB
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)

	// Create collection
	collection := client.Database("game_db").Collection("territories")

	// Create memory cache
	memCache := cache.NewMemoryCache[*Territory](nil)
	defer memCache.Close()

	// Create storage
	storageOptions := &nodestorage.Options{
		VersionField: "VectorClock", // Must match the struct field name, not the bson tag
		CacheTTL:     time.Hour,
	}
	storage, err := nodestorage.NewStorage[*Territory](ctx, collection, memCache, storageOptions)
	if err != nil {
		log.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create territory service
	territoryService := NewTerritoryService(storage)

	// Create a guild
	guildID := primitive.NewObjectID()
	log.Printf("Created guild with ID: %s", guildID.Hex())

	// Create a territory
	territory, err := territoryService.CreateTerritory(ctx, guildID, "Dragon's Lair")
	if err != nil {
		log.Fatalf("Failed to create territory: %v", err)
	}
	log.Printf("Created territory: %s (ID: %s)", territory.Name, territory.ID.Hex())

	// Start watching for changes
	events, err := territoryService.WatchTerritory(ctx, territory.ID)
	if err != nil {
		log.Fatalf("Failed to watch territory: %v", err)
	}

	// Process events in a goroutine
	go func() {
		for event := range events {
			log.Printf("Territory changed: %s", event.Operation)
			if event.Data != nil {
				log.Printf("Building count: %d", len(event.Data.Buildings))

				// Print building status
				for _, building := range event.Data.Buildings {
					log.Printf("Building %s: Status=%s, Progress=%.2f%%",
						building.Type, building.Status, building.ConstructionProgress*100)
				}
			}
		}
	}()

	// Plan headquarters building
	territory, err = territoryService.PlanBuilding(ctx, territory.ID, BuildingTypeHeadquarters, Position{X: 5, Y: 5})
	if err != nil {
		log.Fatalf("Failed to plan headquarters: %v", err)
	}
	log.Printf("Planned headquarters at position (5,5)")

	// Plan barracks building
	territory, err = territoryService.PlanBuilding(ctx, territory.ID, BuildingTypeBarracks, Position{X: 3, Y: 3})
	if err != nil {
		log.Fatalf("Failed to plan barracks: %v", err)
	}
	log.Printf("Planned barracks at position (3,3)")

	// Get the headquarters building ID
	var headquartersID primitive.ObjectID
	var barracksID primitive.ObjectID
	for _, building := range territory.Buildings {
		if building.Type == BuildingTypeHeadquarters {
			headquartersID = building.ID
		} else if building.Type == BuildingTypeBarracks {
			barracksID = building.ID
		}
	}

	// Simulate multiple guild members contributing to the headquarters
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(memberID int) {
			defer wg.Done()

			// Contribute resources
			memberObjectID := primitive.NewObjectID()
			memberName := fmt.Sprintf("Member%d", memberID)
			resources := Resources{
				Gold:  200,
				Wood:  100,
				Stone: 100,
				Iron:  40,
				Food:  0,
			}

			_, err := territoryService.ContributeToBuilding(
				ctx, territory.ID, headquartersID, memberObjectID, memberName, resources)
			if err != nil {
				log.Printf("Member %d failed to contribute to headquarters: %v", memberID, err)
				return
			}
			log.Printf("Member %d contributed resources to headquarters", memberID)
		}(i)
	}

	// Wait for all headquarters contributions
	wg.Wait()

	// Get the updated territory
	territory, err = territoryService.GetTerritory(ctx, territory.ID)
	if err != nil {
		log.Fatalf("Failed to get territory: %v", err)
	}

	// Print headquarters status
	for _, building := range territory.Buildings {
		if building.Type == BuildingTypeHeadquarters {
			log.Printf("Headquarters: Status=%s, Progress=%.2f%%",
				building.Status, building.ConstructionProgress*100)
			log.Printf("Contributors: %d", len(building.Contributors))
		}
	}

	// Simulate multiple guild members contributing to the barracks
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(memberID int) {
			defer wg.Done()

			// Contribute resources
			memberObjectID := primitive.NewObjectID()
			memberName := fmt.Sprintf("Member%d", memberID+5)
			resources := Resources{
				Gold:  150,
				Wood:  100,
				Stone: 70,
				Iron:  30,
				Food:  0,
			}

			_, err := territoryService.ContributeToBuilding(
				ctx, territory.ID, barracksID, memberObjectID, memberName, resources)
			if err != nil {
				log.Printf("Member %d failed to contribute to barracks: %v", memberID+5, err)
				return
			}
			log.Printf("Member %d contributed resources to barracks", memberID+5)
		}(i)
	}

	// Wait for all barracks contributions
	wg.Wait()

	// Get the updated territory again
	territory, err = territoryService.GetTerritory(ctx, territory.ID)
	if err != nil {
		log.Fatalf("Failed to get territory: %v", err)
	}

	// Print final building status
	log.Printf("Final territory status:")
	for _, building := range territory.Buildings {
		log.Printf("Building %s: Status=%s, Progress=%.2f%%, Level=%d",
			building.Type, building.Status, building.ConstructionProgress*100, building.Level)
		log.Printf("  Contributors: %d", len(building.Contributors))

		if building.Status == BuildingStatusCompleted {
			log.Printf("  Completed at: %s", building.CompletedAt.Format(time.RFC3339))
		}
	}

	// Try to upgrade the headquarters if it's completed
	for _, building := range territory.Buildings {
		if building.Type == BuildingTypeHeadquarters && building.Status == BuildingStatusCompleted {
			territory, err = territoryService.UpgradeBuilding(ctx, territory.ID, building.ID)
			if err != nil {
				log.Printf("Failed to upgrade headquarters: %v", err)
			} else {
				log.Printf("Started headquarters upgrade to level 2")
			}
			break
		}
	}

	// Final territory state
	territory, err = territoryService.GetTerritory(ctx, territory.ID)
	if err != nil {
		log.Fatalf("Failed to get territory: %v", err)
	}

	log.Printf("Example completed successfully")
}
