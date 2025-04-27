package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	v2 "nodestorage/v2"
	"nodestorage/v2/cache"
	"tictactoe/transport"
)

func main() {
	// Parse command line flags
	mongoURI := flag.String("mongo-uri", "mongodb://localhost:27017", "MongoDB connection URI")
	dbName := flag.String("db-name", "transport_db", "Database name")
	demoMode := flag.Bool("demo", false, "Run in demo mode with sample data")
	envFile := flag.String("env", ".env", "Path to .env file")
	flag.Parse()

	// Load environment variables from .env file if it exists
	if _, err := os.Stat(*envFile); err == nil {
		if err := godotenv.Load(*envFile); err != nil {
			log.Printf("Warning: Error loading .env file: %v", err)
		}
	}

	// Override with environment variables if they exist
	if uri := os.Getenv("MONGO_URI"); uri != "" {
		*mongoURI = uri
	}
	if name := os.Getenv("DB_NAME"); name != "" {
		*dbName = name
	}

	// Create context that can be canceled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to MongoDB
	log.Printf("Connecting to MongoDB at %s...", *mongoURI)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(*mongoURI))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)

	// Ping the database to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("Failed to ping MongoDB: %v", err)
	}
	log.Printf("Connected to MongoDB successfully")

	// Create collections
	mineCollection := client.Database(*dbName).Collection("mines")
	mineConfigCollection := client.Database(*dbName).Collection("mine_configs")
	transportCollection := client.Database(*dbName).Collection("transports")
	ticketCollection := client.Database(*dbName).Collection("tickets")
	generalCollection := client.Database(*dbName).Collection("generals")

	// Create caches
	mineCache := cache.NewMemoryCache[*transport.Mine](nil)
	mineConfigCache := cache.NewMemoryCache[*transport.MineConfig](nil)
	transportCache := cache.NewMemoryCache[*transport.Transport](nil)
	ticketCache := cache.NewMemoryCache[*transport.TransportTicket](nil)
	generalCache := cache.NewMemoryCache[*transport.General](nil)

	// Create storage options
	storageOptions := &v2.Options{
		VersionField: "VectorClock", // Must match the struct field name
		CacheTTL:     time.Hour,
	}

	// Create storages
	mineStorage, err := v2.NewStorage[*transport.Mine](ctx, client, mineCollection, mineCache, storageOptions)
	if err != nil {
		log.Fatalf("Failed to create mine storage: %v", err)
	}
	defer mineStorage.Close()

	mineConfigStorage, err := v2.NewStorage[*transport.MineConfig](ctx, client, mineConfigCollection, mineConfigCache, storageOptions)
	if err != nil {
		log.Fatalf("Failed to create mine config storage: %v", err)
	}
	defer mineConfigStorage.Close()

	transportStorage, err := v2.NewStorage[*transport.Transport](ctx, client, transportCollection, transportCache, storageOptions)
	if err != nil {
		log.Fatalf("Failed to create transport storage: %v", err)
	}
	defer transportStorage.Close()

	ticketStorage, err := v2.NewStorage[*transport.TransportTicket](ctx, client, ticketCollection, ticketCache, storageOptions)
	if err != nil {
		log.Fatalf("Failed to create ticket storage: %v", err)
	}
	defer ticketStorage.Close()

	generalStorage, err := v2.NewStorage[*transport.General](ctx, client, generalCollection, generalCache, storageOptions)
	if err != nil {
		log.Fatalf("Failed to create general storage: %v", err)
	}
	defer generalStorage.Close()

	// Create services
	ticketService := transport.NewTicketService(ticketStorage)
	generalService := transport.NewGeneralService(generalStorage)
	mineService := transport.NewMineService(mineStorage, mineConfigStorage, generalService, ticketService)
	transportService := transport.NewTransportService(transportStorage, mineService, ticketService)

	// Run in demo mode if requested
	if *demoMode {
		runDemo(ctx, mineService, ticketService, transportService)
	} else {
		// Start the application
		log.Printf("Transport system started. Press Ctrl+C to exit.")

		// Setup signal handling for graceful shutdown
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		// Wait for termination signal
		<-sigCh
		log.Printf("Received termination signal. Shutting down...")
	}

	log.Printf("Transport system shutdown complete")
}

// runDemo runs a demonstration of the transport system
func runDemo(ctx context.Context, mineService *transport.MineService, ticketService *transport.TicketService, transportService *transport.TransportService) {
	log.Printf("Running in demo mode...")

	// Create an alliance
	allianceID := primitive.NewObjectID()
	log.Printf("Created alliance with ID: %s", allianceID.Hex())

	// Create mine configurations with development settings
	for level := transport.MineLevel(1); level <= 5; level++ {
		minTransport := 100 * int(level)
		maxTransport := 500 * int(level)
		transportTime := 30 + (10 * int(level)) // 40-80 minutes
		maxParticipants := 3 + int(level)       // 4-8 participants

		var requiredPoints float64
		var transportTicketMax int

		switch level {
		case transport.MineLevel1:
			requiredPoints = 10000 // 약 1일
			transportTicketMax = 5
		case transport.MineLevel2:
			requiredPoints = 30000 // 약 3일
			transportTicketMax = 10
		case transport.MineLevel3:
			requiredPoints = 70000 // 약 7일
			transportTicketMax = 15
		default:
			requiredPoints = float64(10000 * int(level))
			transportTicketMax = 5 * int(level)
		}

		config, err := mineService.CreateOrUpdateMineConfigWithDevelopment(
			ctx, level, minTransport, maxTransport, transportTime, maxParticipants,
			requiredPoints, transportTicketMax)
		if err != nil {
			log.Fatalf("Failed to create mine config: %v", err)
		}
		log.Printf("Created mine config for level %d: min=%d, max=%d, time=%d, participants=%d, requiredPoints=%.0f, ticketMax=%d",
			config.Level, config.MinTransportAmount, config.MaxTransportAmount,
			config.TransportTime, config.MaxParticipants, config.RequiredPoints, config.TransportTicketMax)
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

	ticket3, err := ticketService.GetOrCreateTickets(ctx, player3ID, allianceID, 5)
	if err != nil {
		log.Fatalf("Failed to create tickets: %v", err)
	}
	log.Printf("Created tickets for %s: %d/%d", player3Name, ticket3.CurrentTickets, ticket3.MaxTickets)

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

	// Update transport status to in progress for demonstration purposes
	_, err = transportService.UpdateTransportStatus(ctx, transport2.ID, transport.TransportStatusInProgress)
	if err != nil {
		log.Printf("Failed to update transport status: %v", err)
	}

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

	// Create a new undeveloped mine for testing development
	mine3, err := mineService.CreateMine(ctx, allianceID, "Gold Mine Gamma", 2)
	if err != nil {
		log.Fatalf("Failed to create mine: %v", err)
	}
	log.Printf("Created undeveloped mine: %s (ID: %s), status: %s",
		mine3.Name, mine3.ID.Hex(), mine3.Status)
	log.Printf("Mine development required points: %.0f", mine3.RequiredPoints)

	// Create generals for players
	general1, err := mineService.CreateGeneralForDemo(ctx, player1ID, "General Zhang", 50, 10, transport.GeneralRarityLegendary)
	if err != nil {
		log.Fatalf("Failed to create general: %v", err)
	}
	log.Printf("Created general for %s: %s (Level: %d, Stars: %d, Rarity: %s)",
		player1Name, general1.Name, general1.Level, general1.Stars, general1.Rarity)

	general2, err := mineService.CreateGeneralForDemo(ctx, player2ID, "General Li", 30, 5, transport.GeneralRarityEpic)
	if err != nil {
		log.Fatalf("Failed to create general: %v", err)
	}
	log.Printf("Created general for %s: %s (Level: %d, Stars: %d, Rarity: %s)",
		player2Name, general2.Name, general2.Level, general2.Stars, general2.Rarity)

	general3, err := mineService.CreateGeneralForDemo(ctx, player3ID, "General Wang", 20, 3, transport.GeneralRarityRare)
	if err != nil {
		log.Fatalf("Failed to create general: %v", err)
	}
	log.Printf("Created general for %s: %s (Level: %d, Stars: %d, Rarity: %s)",
		player3Name, general3.Name, general3.Level, general3.Stars, general3.Rarity)

	// Assign generals to the mine for development
	mine3, err = mineService.AssignGeneralToMine(ctx, mine3.ID, player1ID, player1Name, general1.ID)
	if err != nil {
		log.Fatalf("Failed to assign general to mine: %v", err)
	}
	log.Printf("Assigned %s's general %s to mine %s", player1Name, general1.Name, mine3.Name)

	mine3, err = mineService.AssignGeneralToMine(ctx, mine3.ID, player2ID, player2Name, general2.ID)
	if err != nil {
		log.Fatalf("Failed to assign general to mine: %v", err)
	}
	log.Printf("Assigned %s's general %s to mine %s", player2Name, general2.Name, mine3.Name)

	mine3, err = mineService.AssignGeneralToMine(ctx, mine3.ID, player3ID, player3Name, general3.ID)
	if err != nil {
		log.Fatalf("Failed to assign general to mine: %v", err)
	}
	log.Printf("Assigned %s's general %s to mine %s", player3Name, general3.Name, mine3.Name)

	// Check mine status after assigning generals
	log.Printf("Mine %s status: %s, assigned generals: %d",
		mine3.Name, mine3.Status, len(mine3.AssignedGenerals))

	// Simulate development progress
	// For demo purposes, we'll simulate a large time passage to complete development quickly
	mine3, err = mineService.UpdateMineDevelopment(ctx, mine3.ID)
	if err != nil {
		log.Fatalf("Failed to update mine development: %v", err)
	}
	log.Printf("Updated mine development. Current points: %.2f/%.0f",
		mine3.DevelopmentPoints, mine3.RequiredPoints)

	// Force complete development for demo purposes
	mine3, err = mineService.ForceCompleteDevelopment(ctx, mine3.ID)
	if err != nil {
		log.Fatalf("Failed to force complete mine development: %v", err)
	}
	log.Printf("Mine development completed. Status: %s", mine3.Status)

	// Activate the mine
	mine3, err = mineService.ActivateMine(ctx, mine3.ID)
	if err != nil {
		log.Fatalf("Failed to activate mine: %v", err)
	}
	log.Printf("Mine activated. Status: %s", mine3.Status)

	// Check if transport tickets were updated
	ticket1, err = ticketService.GetOrCreateTickets(ctx, player1ID, allianceID, 5)
	if err != nil {
		log.Fatalf("Failed to get tickets: %v", err)
	}
	log.Printf("Player %s tickets after mine development: %d/%d",
		player1Name, ticket1.CurrentTickets, ticket1.MaxTickets)

	log.Printf("Demo completed successfully")
}
