package domain

import (
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TransportStatus는 이송의 상태를 나타냅니다.
type TransportStatus string

const (
	TransportStatusPreparing  TransportStatus = "PREPARING"
	TransportStatusInTransit  TransportStatus = "IN_TRANSIT"
	TransportStatusCompleted  TransportStatus = "COMPLETED"
	TransportStatusRaided     TransportStatus = "RAIDED"
)

// TransportAggregate는 이송 애그리게이트입니다.
type TransportAggregate struct {
	BaseAggregateRoot
	AllianceID          string
	PlayerID            string
	MineID              string
	GeneralID           string
	GoldAmount          int
	Status              TransportStatus
	StartTime           time.Time
	EstimatedArrivalTime time.Time
	ActualArrivalTime   *time.Time
	Participants        []TransportParticipant
	RaidInfo            *RaidInfo
}

// TransportParticipant는 이송 참여자 정보입니다.
type TransportParticipant struct {
	PlayerID      string
	PlayerName    string
	GoldAmount    int
	JoinedAt      time.Time
}

// RaidInfo는 약탈 정보입니다.
type RaidInfo struct {
	RaidID             string
	RaiderID           string
	RaiderName         string
	RaidStartTime      time.Time
	DefenseEndTime     time.Time
	IsDefended         bool
	DefenseSuccessful  bool
	DefenderID         string
	DefenderName       string
	DefenseCompletedAt *time.Time
	GoldAmountLost     int
}

// NewTransportAggregate는 새로운 TransportAggregate를 생성합니다.
func NewTransportAggregate(id string) *TransportAggregate {
	aggregate := &TransportAggregate{
		BaseAggregateRoot: BaseAggregateRoot{
			ID:      id,
			Type:    "Transport",
			version: 0,
			changes: []Event{},
		},
		Status:       TransportStatusPreparing,
		Participants: []TransportParticipant{},
	}
	return aggregate
}

// Create는 이송을 생성합니다.
func (t *TransportAggregate) Create(cmd *CreateTransportCommand) error {
	if t.Version() > 0 {
		return fmt.Errorf("transport already exists")
	}

	// 이벤트 생성 및 적용
	event := &TransportCreatedEvent{
		BaseEvent: NewBaseEvent(
			primitive.NewObjectID().Hex(),
			"TransportCreated",
			t.AggregateID(),
			t.AggregateType(),
		),
		AllianceID:          cmd.AllianceID,
		PlayerID:            cmd.PlayerID,
		MineID:              cmd.MineID,
		GeneralID:           cmd.GeneralID,
		GoldAmount:          cmd.GoldAmount,
		CreatedAt:           time.Now(),
	}

	t.ApplyChange(event)
	return nil
}

// Start는 이송을 시작합니다.
func (t *TransportAggregate) Start(cmd *StartTransportCommand) error {
	if t.Status != TransportStatusPreparing {
		return fmt.Errorf("transport cannot be started in its current state: %s", t.Status)
	}

	// 이벤트 생성 및 적용
	event := &TransportStartedEvent{
		BaseEvent: NewBaseEvent(
			primitive.NewObjectID().Hex(),
			"TransportStarted",
			t.AggregateID(),
			t.AggregateType(),
		),
		StartTime:           cmd.StartTime,
		EstimatedArrivalTime: cmd.EstimatedArrivalTime,
	}

	t.ApplyChange(event)
	return nil
}

// Complete는 이송을 완료합니다.
func (t *TransportAggregate) Complete(cmd *CompleteTransportCommand) error {
	if t.Status != TransportStatusInTransit {
		return fmt.Errorf("transport cannot be completed in its current state: %s", t.Status)
	}

	// 이벤트 생성 및 적용
	event := &TransportCompletedEvent{
		BaseEvent: NewBaseEvent(
			primitive.NewObjectID().Hex(),
			"TransportCompleted",
			t.AggregateID(),
			t.AggregateType(),
		),
		CompletedAt: cmd.CompletedAt,
	}

	t.ApplyChange(event)
	return nil
}

// Raid는 이송을 약탈합니다.
func (t *TransportAggregate) Raid(cmd *RaidTransportCommand) error {
	if t.Status != TransportStatusInTransit {
		return fmt.Errorf("transport cannot be raided in its current state: %s", t.Status)
	}

	if t.RaidInfo != nil {
		return fmt.Errorf("transport is already being raided")
	}

	// 이벤트 생성 및 적용
	event := &TransportRaidedEvent{
		BaseEvent: NewBaseEvent(
			primitive.NewObjectID().Hex(),
			"TransportRaided",
			t.AggregateID(),
			t.AggregateType(),
		),
		RaidID:         cmd.RaidID,
		RaiderID:       cmd.RaiderID,
		RaiderName:     cmd.RaiderName,
		RaidStartTime:  cmd.RaidStartTime,
		DefenseEndTime: cmd.DefenseEndTime,
	}

	t.ApplyChange(event)
	return nil
}

// Defend는 이송을 방어합니다.
func (t *TransportAggregate) Defend(cmd *DefendTransportCommand) error {
	if t.RaidInfo == nil {
		return fmt.Errorf("transport is not being raided")
	}

	if t.RaidInfo.IsDefended {
		return fmt.Errorf("transport is already defended")
	}

	// 이벤트 생성 및 적용
	event := &TransportDefendedEvent{
		BaseEvent: NewBaseEvent(
			primitive.NewObjectID().Hex(),
			"TransportDefended",
			t.AggregateID(),
			t.AggregateType(),
		),
		DefenderID:        cmd.DefenderID,
		DefenderName:      cmd.DefenderName,
		DefenseSuccessful: cmd.Successful,
		DefendedAt:        cmd.DefendedAt,
		GoldAmountLost:    cmd.GoldAmountLost,
	}

	t.ApplyChange(event)
	return nil
}

// ApplyTransportCreatedEvent는 TransportCreatedEvent를 적용합니다.
func (t *TransportAggregate) ApplyTransportCreatedEvent(event *TransportCreatedEvent) {
	t.AllianceID = event.AllianceID
	t.PlayerID = event.PlayerID
	t.MineID = event.MineID
	t.GeneralID = event.GeneralID
	t.GoldAmount = event.GoldAmount
	t.Status = TransportStatusPreparing
	t.Participants = append(t.Participants, TransportParticipant{
		PlayerID:   event.PlayerID,
		PlayerName: event.PlayerName,
		GoldAmount: event.GoldAmount,
		JoinedAt:   event.CreatedAt,
	})
}

// ApplyTransportStartedEvent는 TransportStartedEvent를 적용합니다.
func (t *TransportAggregate) ApplyTransportStartedEvent(event *TransportStartedEvent) {
	t.Status = TransportStatusInTransit
	t.StartTime = event.StartTime
	t.EstimatedArrivalTime = event.EstimatedArrivalTime
}

// ApplyTransportCompletedEvent는 TransportCompletedEvent를 적용합니다.
func (t *TransportAggregate) ApplyTransportCompletedEvent(event *TransportCompletedEvent) {
	t.Status = TransportStatusCompleted
	t.ActualArrivalTime = &event.CompletedAt
}

// ApplyTransportRaidedEvent는 TransportRaidedEvent를 적용합니다.
func (t *TransportAggregate) ApplyTransportRaidedEvent(event *TransportRaidedEvent) {
	t.RaidInfo = &RaidInfo{
		RaidID:         event.RaidID,
		RaiderID:       event.RaiderID,
		RaiderName:     event.RaiderName,
		RaidStartTime:  event.RaidStartTime,
		DefenseEndTime: event.DefenseEndTime,
		IsDefended:     false,
	}
}

// ApplyTransportDefendedEvent는 TransportDefendedEvent를 적용합니다.
func (t *TransportAggregate) ApplyTransportDefendedEvent(event *TransportDefendedEvent) {
	if t.RaidInfo != nil {
		t.RaidInfo.IsDefended = true
		t.RaidInfo.DefenseSuccessful = event.DefenseSuccessful
		t.RaidInfo.DefenderID = event.DefenderID
		t.RaidInfo.DefenderName = event.DefenderName
		t.RaidInfo.DefenseCompletedAt = &event.DefendedAt
		t.RaidInfo.GoldAmountLost = event.GoldAmountLost

		// 방어 실패로 인한 약탈 성공 시 상태 변경
		if !event.DefenseSuccessful && event.GoldAmountLost >= t.GoldAmount {
			t.Status = TransportStatusRaided
			t.GoldAmount = 0
		} else if !event.DefenseSuccessful {
			t.GoldAmount -= event.GoldAmountLost
		}
	}
}
