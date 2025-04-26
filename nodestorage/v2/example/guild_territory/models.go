package guild_territory

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Guild represents a guild in the game
type Guild struct {
	ID          primitive.ObjectID `bson:"_id"`
	Name        string             `bson:"name"`
	Level       int                `bson:"level"`
	MemberCount int                `bson:"member_count"`
	CreatedAt   time.Time          `bson:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at"`
	// VectorClock must be exported (capitalized) and have the exact field name that matches
	// the version field specified in the storage options
	VectorClock int64 `bson:"vector_clock"` // For optimistic concurrency control
}

// Copy creates a deep copy of the Guild
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

// Territory represents a guild's territory
type Territory struct {
	ID        primitive.ObjectID `bson:"_id"`
	GuildID   primitive.ObjectID `bson:"guild_id"`
	Name      string             `bson:"name"`
	Level     int                `bson:"level"`
	Size      int                `bson:"size"` // Size of the territory (affects max buildings)
	Buildings []Building         `bson:"buildings"`
	Resources Resources          `bson:"resources"` // Resources stored in the territory
	UpdatedAt time.Time          `bson:"updated_at"`
	// VectorClock must be exported (capitalized) and have the exact field name that matches
	// the version field specified in the storage options
	VectorClock int64 `bson:"vector_clock"` // For optimistic concurrency control
}

// Copy creates a deep copy of the Territory
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

// GetVectorClock returns the vector clock value
func (t *Territory) GetVectorClock() int64 {
	return t.VectorClock
}

// SetVectorClock sets the vector clock value
func (t *Territory) SetVectorClock(value int64) {
	t.VectorClock = value
}

// BuildingType defines the type of building
type BuildingType string

const (
	BuildingTypeHeadquarters BuildingType = "headquarters"
	BuildingTypeBarracks     BuildingType = "barracks"
	BuildingTypeMine         BuildingType = "mine"
	BuildingTypeLumberMill   BuildingType = "lumber_mill"
	BuildingTypeStoneQuarry  BuildingType = "stone_quarry"
	BuildingTypeFarm         BuildingType = "farm"
)

// BuildingStatus defines the status of a building
type BuildingStatus string

const (
	BuildingStatusPlanned           BuildingStatus = "planned"
	BuildingStatusUnderConstruction BuildingStatus = "under_construction"
	BuildingStatusCompleted         BuildingStatus = "completed"
)

// Building represents a structure in the guild territory
type Building struct {
	ID                   primitive.ObjectID `bson:"_id"`
	Type                 BuildingType       `bson:"type"`
	Level                int                `bson:"level"`
	Status               BuildingStatus     `bson:"status"`
	Position             Position           `bson:"position"`
	ConstructionProgress float64            `bson:"construction_progress"` // 0.0 to 1.0
	RequiredResources    Resources          `bson:"required_resources"`    // Resources needed to complete
	Contributors         []Contribution     `bson:"contributors"`          // Who contributed to construction
	StartedAt            time.Time          `bson:"started_at"`
	CompletedAt          *time.Time         `bson:"completed_at,omitempty"`
	// VectorClock must be exported (capitalized) and have the exact field name that matches
	// the version field specified in the storage options for section-based concurrency control
	VectorClock int64 `bson:"vector_clock"` // For section-based concurrency control
}

// Copy creates a deep copy of the Building
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
		ID:                   b.ID,
		Type:                 b.Type,
		Level:                b.Level,
		Status:               b.Status,
		Position:             b.Position,
		ConstructionProgress: b.ConstructionProgress,
		RequiredResources:    b.RequiredResources.Copy(),
		Contributors:         contributorsCopy,
		StartedAt:            b.StartedAt,
		CompletedAt:          completedAtCopy,
		VectorClock:          b.VectorClock,
	}
}

// Resources represents the resources in the game
type Resources struct {
	Gold  int `bson:"gold"`
	Wood  int `bson:"wood"`
	Stone int `bson:"stone"`
	Iron  int `bson:"iron"`
	Food  int `bson:"food"`
}

// Copy creates a deep copy of the Resources
func (r Resources) Copy() Resources {
	return Resources{
		Gold:  r.Gold,
		Wood:  r.Wood,
		Stone: r.Stone,
		Iron:  r.Iron,
		Food:  r.Food,
	}
}

// Add adds the given resources to this resource
func (r *Resources) Add(other Resources) {
	r.Gold += other.Gold
	r.Wood += other.Wood
	r.Stone += other.Stone
	r.Iron += other.Iron
	r.Food += other.Food
}

// Subtract subtracts the given resources from this resource
// Returns true if there were enough resources, false otherwise
func (r *Resources) Subtract(other Resources) bool {
	if r.Gold < other.Gold || r.Wood < other.Wood || r.Stone < other.Stone ||
		r.Iron < other.Iron || r.Food < other.Food {
		return false
	}

	r.Gold -= other.Gold
	r.Wood -= other.Wood
	r.Stone -= other.Stone
	r.Iron -= other.Iron
	r.Food -= other.Food
	return true
}

// IsEnough checks if this resource is enough to cover the required resources
func (r Resources) IsEnough(required Resources) bool {
	return r.Gold >= required.Gold &&
		r.Wood >= required.Wood &&
		r.Stone >= required.Stone &&
		r.Iron >= required.Iron &&
		r.Food >= required.Food
}

// Contribution represents a contribution of resources by a guild member
type Contribution struct {
	MemberID   primitive.ObjectID `bson:"member_id"`
	MemberName string             `bson:"member_name"`
	Resources  Resources          `bson:"resources"`
	Timestamp  time.Time          `bson:"timestamp"`
}

// Copy creates a deep copy of the Contribution
func (c Contribution) Copy() Contribution {
	return Contribution{
		MemberID:   c.MemberID,
		MemberName: c.MemberName,
		Resources:  c.Resources.Copy(),
		Timestamp:  c.Timestamp,
	}
}

// Position represents a 2D position in the territory
type Position struct {
	X int `bson:"x"`
	Y int `bson:"y"`
}
