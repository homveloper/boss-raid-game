package transport

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MineLevel represents the level of a mine
type MineLevel int

// MineLevel constants
const (
	MineLevel1 MineLevel = 1
	MineLevel2 MineLevel = 2
	MineLevel3 MineLevel = 3
	MineLevel4 MineLevel = 4
	MineLevel5 MineLevel = 5
)

// MineStatus represents the status of a mine
type MineStatus string

// Mine status constants
const (
	MineStatusUndeveloped MineStatus = "undeveloped" // 개발 전
	MineStatusDeveloping  MineStatus = "developing"  // 개발 중
	MineStatusDeveloped   MineStatus = "developed"   // 개발 완료
	MineStatusActive      MineStatus = "active"      // 활성화 (채광 가능)
	MineStatusInactive    MineStatus = "inactive"    // 비활성화
)

// GeneralRarity represents the rarity of a general
type GeneralRarity string

// General rarity constants
const (
	GeneralRarityCommon    GeneralRarity = "common"    // 일반
	GeneralRarityUncommon  GeneralRarity = "uncommon"  // 고급
	GeneralRarityRare      GeneralRarity = "rare"      // 희귀
	GeneralRaritySoldier   GeneralRarity = "soldier"   // 병졸
	GeneralRarityEpic      GeneralRarity = "epic"      // 영웅
	GeneralRarityLegendary GeneralRarity = "legendary" // 전설
)

// GeneralStatus represents the status of a general
type GeneralStatus string

// General status constants
const (
	GeneralStatusIdle     GeneralStatus = "idle"     // 대기 중
	GeneralStatusAssigned GeneralStatus = "assigned" // 배치됨
)

// Mine represents a gold mine
type Mine struct {
	ID                primitive.ObjectID `bson:"_id,omitempty"`
	AllianceID        primitive.ObjectID `bson:"alliance_id"`
	Name              string             `bson:"name"`
	Level             MineLevel          `bson:"level"`
	GoldOre           int                `bson:"gold_ore"`           // Current gold ore in the mine
	Status            MineStatus         `bson:"status"`             // Mine status
	DevelopmentPoints float64            `bson:"development_points"` // Current development points
	RequiredPoints    float64            `bson:"required_points"`    // Required development points
	AssignedGenerals  []AssignedGeneral  `bson:"assigned_generals"`  // Assigned generals for development
	LastUpdatedAt     time.Time          `bson:"last_updated_at"`    // Last time development points were updated
	CreatedAt         time.Time          `bson:"created_at"`
	UpdatedAt         time.Time          `bson:"updated_at"`
	VectorClock       int64              `bson:"vector_clock"` // For optimistic concurrency control
}

// AssignedGeneral represents a general assigned to a mine for development
type AssignedGeneral struct {
	PlayerID         primitive.ObjectID `bson:"player_id"`
	PlayerName       string             `bson:"player_name"`
	GeneralID        primitive.ObjectID `bson:"general_id"`
	GeneralName      string             `bson:"general_name"`
	Level            int                `bson:"level"`
	Stars            int                `bson:"stars"`
	Rarity           GeneralRarity      `bson:"rarity"`
	AssignedAt       time.Time          `bson:"assigned_at"`
	ContributionRate float64            `bson:"contribution_rate"` // Points contributed per hour
}

// Copy creates a deep copy of the AssignedGeneral
func (ag AssignedGeneral) Copy() AssignedGeneral {
	return AssignedGeneral{
		PlayerID:         ag.PlayerID,
		PlayerName:       ag.PlayerName,
		GeneralID:        ag.GeneralID,
		GeneralName:      ag.GeneralName,
		Level:            ag.Level,
		Stars:            ag.Stars,
		Rarity:           ag.Rarity,
		AssignedAt:       ag.AssignedAt,
		ContributionRate: ag.ContributionRate,
	}
}

// Copy creates a deep copy of the Mine
func (m *Mine) Copy() *Mine {
	if m == nil {
		return nil
	}

	assignedGeneralsCopy := make([]AssignedGeneral, len(m.AssignedGenerals))
	for i, ag := range m.AssignedGenerals {
		assignedGeneralsCopy[i] = ag.Copy()
	}

	return &Mine{
		ID:                m.ID,
		AllianceID:        m.AllianceID,
		Name:              m.Name,
		Level:             m.Level,
		GoldOre:           m.GoldOre,
		Status:            m.Status,
		DevelopmentPoints: m.DevelopmentPoints,
		RequiredPoints:    m.RequiredPoints,
		AssignedGenerals:  assignedGeneralsCopy,
		LastUpdatedAt:     m.LastUpdatedAt,
		CreatedAt:         m.CreatedAt,
		UpdatedAt:         m.UpdatedAt,
		VectorClock:       m.VectorClock,
	}
}

// TransportStatus represents the status of a transport
type TransportStatus string

// Transport status constants
const (
	TransportStatusPreparing  TransportStatus = "preparing"   // Waiting for more members to join
	TransportStatusInProgress TransportStatus = "in_progress" // Transport is in progress
	TransportStatusCompleted  TransportStatus = "completed"   // Transport is completed
	TransportStatusRaided     TransportStatus = "raided"      // Transport was raided
)

// Transport represents a transport of gold ore
type Transport struct {
	ID              primitive.ObjectID `bson:"_id,omitempty"`
	AllianceID      primitive.ObjectID `bson:"alliance_id"`
	MineID          primitive.ObjectID `bson:"mine_id"`
	MineName        string             `bson:"mine_name"`
	MineLevel       MineLevel          `bson:"mine_level"`
	Status          TransportStatus    `bson:"status"`
	GoldOreAmount   int                `bson:"gold_ore_amount"`  // Total gold ore being transported
	MaxParticipants int                `bson:"max_participants"` // Maximum number of participants
	Participants    []TransportMember  `bson:"participants"`     // List of participants
	PrepStartTime   time.Time          `bson:"prep_start_time"`  // When preparation started
	PrepEndTime     time.Time          `bson:"prep_end_time"`    // When preparation ends
	TransportTime   time.Duration      `bson:"transport_time"`   // How long the transport takes
	StartTime       *time.Time         `bson:"start_time"`       // When transport started
	EndTime         *time.Time         `bson:"end_time"`         // When transport will end/ended
	RaidStatus      *RaidStatus        `bson:"raid_status"`      // Raid status if being raided
	CreatedAt       time.Time          `bson:"created_at"`
	UpdatedAt       time.Time          `bson:"updated_at"`
	VectorClock     int64              `bson:"vector_clock"` // For optimistic concurrency control
}

// Copy creates a deep copy of the Transport
func (t *Transport) Copy() *Transport {
	if t == nil {
		return nil
	}

	participantsCopy := make([]TransportMember, len(t.Participants))
	for i, p := range t.Participants {
		participantsCopy[i] = p.Copy()
	}

	var startTimeCopy *time.Time
	if t.StartTime != nil {
		st := *t.StartTime
		startTimeCopy = &st
	}

	var endTimeCopy *time.Time
	if t.EndTime != nil {
		et := *t.EndTime
		endTimeCopy = &et
	}

	var raidStatusCopy *RaidStatus
	if t.RaidStatus != nil {
		rs := *t.RaidStatus
		raidStatusCopy = &rs
	}

	return &Transport{
		ID:              t.ID,
		AllianceID:      t.AllianceID,
		MineID:          t.MineID,
		MineName:        t.MineName,
		MineLevel:       t.MineLevel,
		Status:          t.Status,
		GoldOreAmount:   t.GoldOreAmount,
		MaxParticipants: t.MaxParticipants,
		Participants:    participantsCopy,
		PrepStartTime:   t.PrepStartTime,
		PrepEndTime:     t.PrepEndTime,
		TransportTime:   t.TransportTime,
		StartTime:       startTimeCopy,
		EndTime:         endTimeCopy,
		RaidStatus:      raidStatusCopy,
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
		VectorClock:     t.VectorClock,
	}
}

// TransportMember represents a member participating in a transport
type TransportMember struct {
	PlayerID      primitive.ObjectID `bson:"player_id"`
	PlayerName    string             `bson:"player_name"`
	GoldOreAmount int                `bson:"gold_ore_amount"` // Amount of gold ore this member is transporting
	JoinedAt      time.Time          `bson:"joined_at"`
}

// Copy creates a deep copy of the TransportMember
func (tm TransportMember) Copy() TransportMember {
	return TransportMember{
		PlayerID:      tm.PlayerID,
		PlayerName:    tm.PlayerName,
		GoldOreAmount: tm.GoldOreAmount,
		JoinedAt:      tm.JoinedAt,
	}
}

// RaidStatus represents the status of a raid on a transport
type RaidStatus struct {
	RaiderID       primitive.ObjectID `bson:"raider_id"`
	RaiderName     string             `bson:"raider_name"`
	RaidStartTime  time.Time          `bson:"raid_start_time"`
	DefenseEndTime time.Time          `bson:"defense_end_time"` // When the defense window ends
	IsDefended     bool               `bson:"is_defended"`      // Whether the raid has been defended
	DefenseResult  *DefenseResult     `bson:"defense_result"`   // Result of the defense, if completed
}

// DefenseResult represents the result of a defense against a raid
type DefenseResult struct {
	Successful   bool               `bson:"successful"`    // Whether the defense was successful
	DefenderID   primitive.ObjectID `bson:"defender_id"`   // ID of the player who defended
	DefenderName string             `bson:"defender_name"` // Name of the player who defended
	CompletedAt  time.Time          `bson:"completed_at"`  // When the defense was completed
	GoldOreLost  int                `bson:"gold_ore_lost"` // Amount of gold ore lost if defense failed
}

// TransportTicket represents a player's transport tickets
type TransportTicket struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"`
	PlayerID       primitive.ObjectID `bson:"player_id"`
	AllianceID     primitive.ObjectID `bson:"alliance_id"`
	CurrentTickets int                `bson:"current_tickets"`
	MaxTickets     int                `bson:"max_tickets"`
	LastRefillTime time.Time          `bson:"last_refill_time"`
	PurchaseCount  int                `bson:"purchase_count"`   // Number of purchases today
	LastPurchaseAt *time.Time         `bson:"last_purchase_at"` // Time of last purchase
	ResetTime      time.Time          `bson:"reset_time"`       // When purchase count resets
	CreatedAt      time.Time          `bson:"created_at"`
	UpdatedAt      time.Time          `bson:"updated_at"`
	VectorClock    int64              `bson:"vector_clock"` // For optimistic concurrency control
}

// Copy creates a deep copy of the TransportTicket
func (tt *TransportTicket) Copy() *TransportTicket {
	if tt == nil {
		return nil
	}

	var lastPurchaseAtCopy *time.Time
	if tt.LastPurchaseAt != nil {
		lp := *tt.LastPurchaseAt
		lastPurchaseAtCopy = &lp
	}

	return &TransportTicket{
		ID:             tt.ID,
		PlayerID:       tt.PlayerID,
		AllianceID:     tt.AllianceID,
		CurrentTickets: tt.CurrentTickets,
		MaxTickets:     tt.MaxTickets,
		LastRefillTime: tt.LastRefillTime,
		PurchaseCount:  tt.PurchaseCount,
		LastPurchaseAt: lastPurchaseAtCopy,
		ResetTime:      tt.ResetTime,
		CreatedAt:      tt.CreatedAt,
		UpdatedAt:      tt.UpdatedAt,
		VectorClock:    tt.VectorClock,
	}
}

// General represents a general that can be assigned to various activities
type General struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	PlayerID    primitive.ObjectID `bson:"player_id"`
	Name        string             `bson:"name"`
	Level       int                `bson:"level"`
	Stars       int                `bson:"stars"`       // 성급
	Rarity      GeneralRarity      `bson:"rarity"`      // 희귀도
	Status      GeneralStatus      `bson:"status"`      // 상태
	AssignedTo  *AssignmentInfo    `bson:"assigned_to"` // 배치 정보
	CreatedAt   time.Time          `bson:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at"`
	VectorClock int64              `bson:"vector_clock"` // For optimistic concurrency control
}

// Copy creates a deep copy of the General
func (g *General) Copy() *General {
	if g == nil {
		return nil
	}

	var assignedToCopy *AssignmentInfo
	if g.AssignedTo != nil {
		ai := *g.AssignedTo
		assignedToCopy = &ai
	}

	return &General{
		ID:          g.ID,
		PlayerID:    g.PlayerID,
		Name:        g.Name,
		Level:       g.Level,
		Stars:       g.Stars,
		Rarity:      g.Rarity,
		Status:      g.Status,
		AssignedTo:  assignedToCopy,
		CreatedAt:   g.CreatedAt,
		UpdatedAt:   g.UpdatedAt,
		VectorClock: g.VectorClock,
	}
}

// AssignmentInfo represents information about where a general is assigned
type AssignmentInfo struct {
	Type       string             `bson:"type"`        // "mine_development", "mining", "transport", "raid", "defense"
	TargetID   primitive.ObjectID `bson:"target_id"`   // ID of the target (mine, transport, etc.)
	TargetName string             `bson:"target_name"` // Name of the target
	AssignedAt time.Time          `bson:"assigned_at"` // When the general was assigned
}

// Copy creates a deep copy of the AssignmentInfo
func (ai AssignmentInfo) Copy() AssignmentInfo {
	return AssignmentInfo{
		Type:       ai.Type,
		TargetID:   ai.TargetID,
		TargetName: ai.TargetName,
		AssignedAt: ai.AssignedAt,
	}
}

// MineConfig represents configuration for mines based on level
type MineConfig struct {
	ID                 primitive.ObjectID `bson:"_id,omitempty"`
	Level              MineLevel          `bson:"level"`
	MinTransportAmount int                `bson:"min_transport_amount"` // Minimum amount that can be transported
	MaxTransportAmount int                `bson:"max_transport_amount"` // Maximum amount that can be transported
	TransportTime      int                `bson:"transport_time"`       // Transport time in minutes
	MaxParticipants    int                `bson:"max_participants"`     // Maximum number of participants per transport
	RequiredPoints     float64            `bson:"required_points"`      // Required development points
	TransportTicketMax int                `bson:"transport_ticket_max"` // Max transport tickets after development
	CreatedAt          time.Time          `bson:"created_at"`
	UpdatedAt          time.Time          `bson:"updated_at"`
	VectorClock        int64              `bson:"vector_clock"` // For optimistic concurrency control
}

// Copy creates a deep copy of the MineConfig
func (mc *MineConfig) Copy() *MineConfig {
	if mc == nil {
		return nil
	}
	return &MineConfig{
		ID:                 mc.ID,
		Level:              mc.Level,
		MinTransportAmount: mc.MinTransportAmount,
		MaxTransportAmount: mc.MaxTransportAmount,
		TransportTime:      mc.TransportTime,
		MaxParticipants:    mc.MaxParticipants,
		RequiredPoints:     mc.RequiredPoints,
		TransportTicketMax: mc.TransportTicketMax,
		CreatedAt:          mc.CreatedAt,
		UpdatedAt:          mc.UpdatedAt,
		VectorClock:        mc.VectorClock,
	}
}
