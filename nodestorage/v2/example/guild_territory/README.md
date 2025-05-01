# Guild Territory Construction System

This example application demonstrates how to use the `nodestorage/v2` package to implement a guild territory construction system for an online game. The system allows guild members to collaborate on building and upgrading structures in their guild territory, with real-time updates and proper concurrency control.

## Project Structure

```
guild_territory/
├── README.md           # This documentation file
├── models.go           # Data models for the guild territory system
├── territory.go        # Core business logic for territory management
├── territory_test.go   # Unit tests for territory functionality
├── main.go             # Example usage of the territory system
└── cmd/
    └── main.go         # Executable entry point
```

## Overview

In this example, we'll implement a simplified guild territory construction system with the following features:

1. **Territory Management**: Create and manage guild territories
2. **Building Construction**: Build various structures in the territory
3. **Resource Management**: Manage guild resources used for construction
4. **Collaborative Construction**: Allow multiple guild members to contribute to construction
5. **Real-time Updates**: Notify all guild members about construction progress

## Data Models

### Guild

```go
type Guild struct {
    ID          primitive.ObjectID `bson:"_id"`
    Name        string             `bson:"name"`
    Level       int                `bson:"level"`
    MemberCount int                `bson:"member_count"`
    CreatedAt   time.Time          `bson:"created_at"`
    UpdatedAt   time.Time          `bson:"updated_at"`
    VectorClock int64              `bson:"vector_clock"` // For optimistic concurrency control
}

func (g *Guild) Copy() *Guild {
    if g == nil {
        return nil
    }
    return &Guild{
        ID:          g.ID,
        Name:        g.Name,
        Level:       g.Level,
        MemberCount: g.MemberCount,
        CreatedAt:   g.CreatedAt,
        UpdatedAt:   g.UpdatedAt,
        VectorClock: g.VectorClock,
    }
}
```

### Territory

```go
type Territory struct {
    ID          primitive.ObjectID `bson:"_id"`
    GuildID     primitive.ObjectID `bson:"guild_id"`
    Name        string             `bson:"name"`
    Level       int                `bson:"level"`
    Size        int                `bson:"size"` // Size of the territory (affects max buildings)
    Buildings   []Building         `bson:"buildings"`
    Resources   Resources          `bson:"resources"` // Resources stored in the territory
    UpdatedAt   time.Time          `bson:"updated_at"`
    VectorClock int64              `bson:"vector_clock"` // For optimistic concurrency control
}

func (t *Territory) Copy() *Territory {
    if t == nil {
        return nil
    }

    buildingsCopy := make([]Building, len(t.Buildings))
    for i, b := range t.Buildings {
        buildingsCopy[i] = b.Copy()
    }

    return &Territory{
        ID:          t.ID,
        GuildID:     t.GuildID,
        Name:        t.Name,
        Level:       t.Level,
        Size:        t.Size,
        Buildings:   buildingsCopy,
        Resources:   t.Resources.Copy(),
        UpdatedAt:   t.UpdatedAt,
        VectorClock: t.VectorClock,
    }
}
```

### Building

```go
type BuildingType string

const (
    BuildingTypeHeadquarters BuildingType = "headquarters"
    BuildingTypeBarracks     BuildingType = "barracks"
    BuildingTypeMine         BuildingType = "mine"
    BuildingTypeLumberMill   BuildingType = "lumber_mill"
    BuildingTypeStoneQuarry  BuildingType = "stone_quarry"
    BuildingTypeFarm         BuildingType = "farm"
)

type BuildingStatus string

const (
    BuildingStatusPlanned   BuildingStatus = "planned"
    BuildingStatusUnderConstruction BuildingStatus = "under_construction"
    BuildingStatusCompleted BuildingStatus = "completed"
)

type Building struct {
    ID             primitive.ObjectID `bson:"_id"`
    Type           BuildingType       `bson:"type"`
    Level          int                `bson:"level"`
    Status         BuildingStatus     `bson:"status"`
    Position       Position           `bson:"position"`
    ConstructionProgress float64      `bson:"construction_progress"` // 0.0 to 1.0
    RequiredResources Resources       `bson:"required_resources"`    // Resources needed to complete
    Contributors    []Contribution    `bson:"contributors"`          // Who contributed to construction
    StartedAt      time.Time          `bson:"started_at"`
    CompletedAt    *time.Time         `bson:"completed_at,omitempty"`
    VectorClock    int64              `bson:"vector_clock"` // For section-based concurrency control
}

func (b Building) Copy() Building {
    contributorsCopy := make([]Contribution, len(b.Contributors))
    for i, c := range b.Contributors {
        contributorsCopy[i] = c.Copy()
    }

    var completedAtCopy *time.Time
    if b.CompletedAt != nil {
        t := *b.CompletedAt
        completedAtCopy = &t
    }

    return Building{
        ID:                  b.ID,
        Type:                b.Type,
        Level:               b.Level,
        Status:              b.Status,
        Position:            b.Position,
        ConstructionProgress: b.ConstructionProgress,
        RequiredResources:   b.RequiredResources.Copy(),
        Contributors:        contributorsCopy,
        StartedAt:           b.StartedAt,
        CompletedAt:         completedAtCopy,
        VectorClock:         b.VectorClock,
    }
}
```

### Resources

```go
type Resources struct {
    Gold   int `bson:"gold"`
    Wood   int `bson:"wood"`
    Stone  int `bson:"stone"`
    Iron   int `bson:"iron"`
    Food   int `bson:"food"`
}

func (r Resources) Copy() Resources {
    return Resources{
        Gold:  r.Gold,
        Wood:  r.Wood,
        Stone: r.Stone,
        Iron:  r.Iron,
        Food:  r.Food,
    }
}
```

### Contribution

```go
type Contribution struct {
    MemberID    primitive.ObjectID `bson:"member_id"`
    MemberName  string             `bson:"member_name"`
    Resources   Resources          `bson:"resources"`
    Timestamp   time.Time          `bson:"timestamp"`
}

func (c Contribution) Copy() Contribution {
    return Contribution{
        MemberID:   c.MemberID,
        MemberName: c.MemberName,
        Resources:  c.Resources.Copy(),
        Timestamp:  c.Timestamp,
    }
}
```

### Position

```go
type Position struct {
    X int `bson:"x"`
    Y int `bson:"y"`
}
```

## Key Operations

### 1. Create Territory

Create a new territory for a guild.

```go
func CreateTerritory(ctx context.Context, storage nodestorage.Storage[*Territory], guildID primitive.ObjectID, name string) (*Territory, error) {
    territory := &Territory{
        ID:          primitive.NewObjectID(),
        GuildID:     guildID,
        Name:        name,
        Level:       1,
        Size:        10, // Initial size
        Buildings:   []Building{},
        Resources:   Resources{},
        UpdatedAt:   time.Now(),
    }

    return storage.FindOneAndUpsert(ctx, territory)
}
```

### 2. Plan Building Construction

Plan a new building in the territory.

```go
func PlanBuilding(ctx context.Context, storage nodestorage.Storage[*Territory], territoryID primitive.ObjectID, buildingType BuildingType, position Position) (*Territory, error) {
    return storage.FindOneAndUpdate(ctx, territoryID, func(territory *Territory) (*Territory, error) {
        // Check if position is valid and not occupied
        for _, building := range territory.Buildings {
            if building.Position.X == position.X && building.Position.Y == position.Y {
                return nil, fmt.Errorf("position already occupied")
            }
        }

        // Check if territory has enough space
        if len(territory.Buildings) >= territory.Size {
            return nil, fmt.Errorf("territory is full")
        }

        // Create new building
        building := Building{
            ID:                  primitive.NewObjectID(),
            Type:                buildingType,
            Level:               1,
            Status:              BuildingStatusPlanned,
            Position:            position,
            ConstructionProgress: 0.0,
            RequiredResources:   getBuildingRequiredResources(buildingType, 1),
            Contributors:        []Contribution{},
            StartedAt:           time.Now(),
            VectorClock:         1,
        }

        // Add building to territory
        territory.Buildings = append(territory.Buildings, building)
        territory.UpdatedAt = time.Now()

        return territory, nil
    })
}
```

### 3. Contribute Resources to Building

Contribute resources to a building under construction.

```go
func ContributeToBuilding(ctx context.Context, storage nodestorage.Storage[*Territory], territoryID primitive.ObjectID, buildingID primitive.ObjectID, memberID primitive.ObjectID, memberName string, resources Resources) (*Territory, error) {
    return storage.UpdateSection(ctx, territoryID, fmt.Sprintf("buildings.%s", buildingID.Hex()), func(buildingInterface interface{}) (interface{}, error) {
        buildingMap, ok := buildingInterface.(bson.M)
        if !ok {
            return nil, fmt.Errorf("invalid building data")
        }

        // Check building status
        status, _ := buildingMap["status"].(string)
        if status != string(BuildingStatusPlanned) && status != string(BuildingStatusUnderConstruction) {
            return nil, fmt.Errorf("building is not under construction")
        }

        // Update building status if it's the first contribution
        if status == string(BuildingStatusPlanned) {
            buildingMap["status"] = string(BuildingStatusUnderConstruction)
        }

        // Add contribution
        contribution := bson.M{
            "member_id":   memberID,
            "member_name": memberName,
            "resources": bson.M{
                "gold":  resources.Gold,
                "wood":  resources.Wood,
                "stone": resources.Stone,
                "iron":  resources.Iron,
                "food":  resources.Food,
            },
            "timestamp": time.Now(),
        }

        contributors, _ := buildingMap["contributors"].(bson.A)
        buildingMap["contributors"] = append(contributors, contribution)

        // Update construction progress
        requiredResources, _ := buildingMap["required_resources"].(bson.M)
        totalContributed := calculateTotalContributions(buildingMap["contributors"].(bson.A))
        progress := calculateConstructionProgress(totalContributed, requiredResources)
        buildingMap["construction_progress"] = progress

        // Check if construction is complete
        if progress >= 1.0 {
            buildingMap["status"] = string(BuildingStatusCompleted)
            buildingMap["completed_at"] = time.Now()
            buildingMap["construction_progress"] = 1.0
        }

        return buildingMap, nil
    })
}
```

### 4. Get Territory with Buildings

Get a territory with all its buildings.

```go
func GetTerritory(ctx context.Context, storage nodestorage.Storage[*Territory], territoryID primitive.ObjectID) (*Territory, error) {
    return storage.FindOne(ctx, territoryID)
}
```

### 5. Watch Territory Changes

Watch for changes to a territory.

```go
func WatchTerritory(ctx context.Context, storage nodestorage.Storage[*Territory], territoryID primitive.ObjectID) (<-chan v2.WatchEvent[*Territory], error) {
    pipeline := mongo.Pipeline{
        bson.D{{Key: "$match", Value: bson.D{
            {Key: "documentKey._id", Value: territoryID},
        }}},
    }

    return storage.Watch(ctx, pipeline)
}
```

## Example Usage

Here's how you might use this system in a simple application:

```go
func main() {
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
        VersionField: "vector_clock",
        CacheTTL:     time.Hour,
    }
    storage, err := nodestorage.NewStorage[*Territory](ctx, client, collection, memCache, storageOptions)
    if err != nil {
        log.Fatalf("Failed to create storage: %v", err)
    }
    defer storage.Close()

    // Create a guild
    guildID := primitive.NewObjectID()

    // Create a territory
    territory, err := CreateTerritory(ctx, storage, guildID, "Dragon's Lair")
    if err != nil {
        log.Fatalf("Failed to create territory: %v", err)
    }
    log.Printf("Created territory: %s", territory.Name)

    // Plan a building
    territory, err = PlanBuilding(ctx, storage, territory.ID, BuildingTypeHeadquarters, Position{X: 5, Y: 5})
    if err != nil {
        log.Fatalf("Failed to plan building: %v", err)
    }
    log.Printf("Planned building: %s", territory.Buildings[0].Type)

    // Start watching for changes
    events, err := WatchTerritory(ctx, storage, territory.ID)
    if err != nil {
        log.Fatalf("Failed to watch territory: %v", err)
    }

    // Process events in a goroutine
    go func() {
        for event := range events {
            log.Printf("Territory changed: %s", event.Operation)
            if event.Data != nil {
                log.Printf("New building count: %d", len(event.Data.Buildings))
            }
        }
    }()

    // Simulate multiple guild members contributing to the building
    var wg sync.WaitGroup
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func(memberID int) {
            defer wg.Done()

            // Contribute resources
            memberObjectID := primitive.NewObjectID()
            memberName := fmt.Sprintf("Member%d", memberID)
            resources := Resources{
                Gold:  100,
                Wood:  200,
                Stone: 150,
                Iron:  50,
                Food:  0,
            }

            _, err := ContributeToBuilding(ctx, storage, territory.ID, territory.Buildings[0].ID, memberObjectID, memberName, resources)
            if err != nil {
                log.Printf("Member %d failed to contribute: %v", memberID, err)
                return
            }
            log.Printf("Member %d contributed resources", memberID)
        }(i)
    }

    // Wait for all contributions
    wg.Wait()

    // Get the updated territory
    updatedTerritory, err := GetTerritory(ctx, storage, territory.ID)
    if err != nil {
        log.Fatalf("Failed to get territory: %v", err)
    }

    // Print building status
    for _, building := range updatedTerritory.Buildings {
        log.Printf("Building %s: Status=%s, Progress=%.2f%%",
            building.Type, building.Status, building.ConstructionProgress*100)
        log.Printf("Contributors: %d", len(building.Contributors))
    }
}
```

## Key Features Demonstrated

This example demonstrates several key features of the `nodestorage/v2` package:

1. **Optimistic Concurrency Control**: Using `VectorClock` to handle concurrent updates to the territory.

2. **Section-Based Concurrency Control**: Using `UpdateSection` to update specific buildings without affecting the entire territory document.

3. **Caching**: Using memory cache to improve read performance for frequently accessed territories.

4. **Change Streams**: Using `Watch` to get real-time notifications about territory changes.

5. **Generic Storage**: Using the generic `Storage[*Territory]` interface to work with strongly-typed documents.

## Potential Extensions

This example could be extended in several ways:

1. **Building Upgrades**: Add functionality to upgrade existing buildings.

2. **Resource Production**: Make buildings produce resources over time.

3. **Building Dependencies**: Require certain buildings to be constructed before others.

4. **Territory Expansion**: Allow territories to expand as the guild grows.

5. **Building Specializations**: Allow buildings to be specialized for different purposes.

## Running the Example

To run this example, you need:

1. MongoDB running locally or accessible via a connection string
2. Go 1.18 or later

Run the example with:

```bash
# From the nodestorage/v2/example/guild_territory/cmd directory
go run main.go
```

Run the tests with:

```bash
# From the nodestorage/v2/example/guild_territory directory
go test -v
```

## Conclusion

This example demonstrates how to use the `nodestorage/v2` package to implement a collaborative guild territory construction system with proper concurrency control and real-time updates. The package's features make it easy to handle the complex data management requirements of such a system, allowing developers to focus on the game mechanics rather than the underlying data storage and synchronization.
