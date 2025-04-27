package transport

import (
	"context"
	"log"
	"time"

	v2 "nodestorage/v2"
	"nodestorage/v2/cache"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Example demonstrates the transport system
func Example() {
	// Connect to MongoDB
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)

	// Create collections
	mineCollection := client.Database("transport_db").Collection("mines")
	mineConfigCollection := client.Database("transport_db").Collection("mine_configs")
	transportCollection := client.Database("transport_db").Collection("transports")
	ticketCollection := client.Database("transport_db").Collection("tickets")
	generalCollection := client.Database("transport_db").Collection("generals")

	// Create caches
	mineCache := cache.NewMemoryCache[*Mine](nil)
	mineConfigCache := cache.NewMemoryCache[*MineConfig](nil)
	transportCache := cache.NewMemoryCache[*Transport](nil)
	ticketCache := cache.NewMemoryCache[*TransportTicket](nil)
	generalCache := cache.NewMemoryCache[*General](nil)

	// Create storage options
	storageOptions := &v2.Options{
		VersionField: "VectorClock", // Must match the struct field name
		CacheTTL:     time.Hour,
	}

	// Create storages
	mineStorage, err := v2.NewStorage[*Mine](ctx, client, mineCollection, mineCache, storageOptions)
	if err != nil {
		log.Fatalf("Failed to create mine storage: %v", err)
	}
	defer mineStorage.Close()

	mineConfigStorage, err := v2.NewStorage[*MineConfig](ctx, client, mineConfigCollection, mineConfigCache, storageOptions)
	if err != nil {
		log.Fatalf("Failed to create mine config storage: %v", err)
	}
	defer mineConfigStorage.Close()

	transportStorage, err := v2.NewStorage[*Transport](ctx, client, transportCollection, transportCache, storageOptions)
	if err != nil {
		log.Fatalf("Failed to create transport storage: %v", err)
	}
	defer transportStorage.Close()

	ticketStorage, err := v2.NewStorage[*TransportTicket](ctx, client, ticketCollection, ticketCache, storageOptions)
	if err != nil {
		log.Fatalf("Failed to create ticket storage: %v", err)
	}
	defer ticketStorage.Close()

	generalStorage, err := v2.NewStorage[*General](ctx, client, generalCollection, generalCache, storageOptions)
	if err != nil {
		log.Fatalf("Failed to create general storage: %v", err)
	}
	defer generalStorage.Close()

	// Create services
	ticketService := NewTicketService(ticketStorage)
	generalService := NewGeneralService(generalStorage)
	mineService := NewMineService(mineStorage, mineConfigStorage, generalService, ticketService)
	transportService := NewTransportService(transportStorage, mineService, ticketService)

	// Create an alliance
	allianceID := primitive.NewObjectID()
	log.Printf("Created alliance with ID: %s", allianceID.Hex())

	// Create mine configurations
	for level := MineLevel(1); level <= 5; level++ {
		minTransport := 100 * int(level)
		maxTransport := 500 * int(level)
		transportTime := 30 + (10 * int(level)) // 40-80 minutes
		maxParticipants := 3 + int(level)       // 4-8 participants

		config, err := mineService.CreateOrUpdateMineConfig(
			ctx, level, minTransport, maxTransport, transportTime, maxParticipants)
		if err != nil {
			log.Fatalf("Failed to create mine config: %v", err)
		}
		log.Printf("Created mine config for level %d: min=%d, max=%d, time=%d, participants=%d",
			config.Level, config.MinTransportAmount, config.MaxTransportAmount,
			config.TransportTime, config.MaxParticipants)
	}

	// Create mines
	mine1, err := mineService.CreateMine(ctx, allianceID, "Gold Mine Alpha", 1)
	if err != nil {
		log.Fatalf("Failed to create mine: %v", err)
	}
	log.Printf("Created mine: %s (ID: %s)", mine1.Name, mine1.ID.Hex())

	mine2, err := mineService.CreateMine(ctx, allianceID, "Gold Mine Beta", 3)
	if err != nil {
		log.Fatalf("Failed to create mine: %v", err)
	}
	log.Printf("Created mine: %s (ID: %s)", mine2.Name, mine2.ID.Hex())

	// Add gold ore to mines
	mine1, err = mineService.AddGoldOre(ctx, mine1.ID, 1000)
	if err != nil {
		log.Fatalf("Failed to add gold ore: %v", err)
	}
	log.Printf("Added 1000 gold ore to %s, total: %d", mine1.Name, mine1.GoldOre)

	mine2, err = mineService.AddGoldOre(ctx, mine2.ID, 3000)
	if err != nil {
		log.Fatalf("Failed to add gold ore: %v", err)
	}
	log.Printf("Added 3000 gold ore to %s, total: %d", mine2.Name, mine2.GoldOre)

	// Create players
	player1ID := primitive.NewObjectID()
	player1Name := "Player One"
	player2ID := primitive.NewObjectID()
	player2Name := "Player Two"
	player3ID := primitive.NewObjectID()
	player3Name := "Player Three"

	// Create transport tickets for players
	ticket1, err := ticketService.GetOrCreateTickets(ctx, player1ID, allianceID, 5)
	if err != nil {
		log.Fatalf("Failed to create tickets: %v", err)
	}
	log.Printf("Created tickets for %s: %d/%d", player1Name, ticket1.CurrentTickets, ticket1.MaxTickets)

	ticket2, err := ticketService.GetOrCreateTickets(ctx, player2ID, allianceID, 5)
	if err != nil {
		log.Fatalf("Failed to create tickets: %v", err)
	}
	log.Printf("Created tickets for %s: %d/%d", player2Name, ticket2.CurrentTickets, ticket2.MaxTickets)

	// Start watching for transport changes
	events, err := transportService.WatchAllTransports(ctx, allianceID)
	if err != nil {
		log.Fatalf("Failed to watch transports: %v", err)
	}

	// Process events in a goroutine
	go func() {
		for event := range events {
			log.Printf("Transport changed: %s", event.Operation)
			if event.Data != nil {
				log.Printf("Transport status: %s, Participants: %d, Gold Ore: %d",
					event.Data.Status, len(event.Data.Participants), event.Data.GoldOreAmount)

				if event.Data.RaidStatus != nil {
					log.Printf("Transport is being raided by %s", event.Data.RaidStatus.RaiderName)
					if event.Data.RaidStatus.IsDefended {
						result := "successful"
						if event.Data.RaidStatus.DefenseResult != nil && !event.Data.RaidStatus.DefenseResult.Successful {
							result = "failed"
						}
						log.Printf("Raid defense was %s", result)
					}
				}
			}
		}
	}()

	// Start a transport from mine 1
	transport1, err := transportService.StartTransport(ctx, player1ID, player1Name, mine1.ID, 200)
	if err != nil {
		log.Fatalf("Failed to start transport: %v", err)
	}
	log.Printf("Started transport from %s with %d gold ore", transport1.MineName, transport1.GoldOreAmount)

	// Join the transport with player 2
	transport1, err = transportService.JoinTransport(ctx, transport1.ID, player2ID, player2Name, 150)
	if err != nil {
		log.Fatalf("Failed to join transport: %v", err)
	}
	log.Printf("Player %s joined transport, total gold ore: %d", player2Name, transport1.GoldOreAmount)

	// Start a transport from mine 2
	transport2, err := transportService.StartTransport(ctx, player3ID, player3Name, mine2.ID, 500)
	if err != nil {
		log.Fatalf("Failed to start transport: %v", err)
	}
	log.Printf("Started transport from %s with %d gold ore", transport2.MineName, transport2.GoldOreAmount)

	// Simulate a raid on transport 2
	raiderID := primitive.NewObjectID()
	raiderName := "Raider X"

	// Wait a bit to let the transport start
	time.Sleep(2 * time.Second)

	transport2, err = transportService.RaidTransport(ctx, transport2.ID, raiderID, raiderName)
	if err != nil {
		log.Printf("Failed to raid transport: %v", err)
	} else {
		log.Printf("Transport from %s is being raided by %s", transport2.MineName, raiderName)
	}

	// Defend the transport
	transport2, err = transportService.DefendTransport(ctx, transport2.ID, player3ID, player3Name, true)
	if err != nil {
		log.Printf("Failed to defend transport: %v", err)
	} else {
		log.Printf("Transport from %s was successfully defended by %s", transport2.MineName, player3Name)
	}

	// Purchase a ticket
	ticket1, price, err := ticketService.PurchaseTicket(ctx, player1ID)
	if err != nil {
		log.Printf("Failed to purchase ticket: %v", err)
	} else {
		log.Printf("Purchased a ticket for %s for %d gems, now has %d tickets",
			player1Name, price, ticket1.CurrentTickets)
	}

	// Get all active transports
	transports, err := transportService.GetActiveTransports(ctx, allianceID)
	if err != nil {
		log.Printf("Failed to get active transports: %v", err)
	} else {
		log.Printf("Found %d active transports", len(transports))
		for _, t := range transports {
			log.Printf("Transport from %s, status: %s, participants: %d",
				t.MineName, t.Status, len(t.Participants))
		}
	}

	log.Printf("Example completed successfully")
}
