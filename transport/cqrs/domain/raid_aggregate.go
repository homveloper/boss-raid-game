package domain

import (
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RaidStatus는 약탈의 상태를 나타냅니다.
type RaidStatus string

const (
	RaidStatusPreparing  RaidStatus = "PREPARING"
	RaidStatusInProgress RaidStatus = "IN_PROGRESS"
	RaidStatusSuccessful RaidStatus = "SUCCESSFUL"
	RaidStatusFailed     RaidStatus = "FAILED"
	RaidStatusCanceled   RaidStatus = "CANCELED"
)

// RaidAggregate는 약탈 애그리게이트입니다.
type RaidAggregate struct {
	BaseAggregateRoot
	AllianceID           string
	TargetAllianceID     string
	PlayerID             string
	TransportID          string
	GeneralID            string
	Status               RaidStatus
	GoldAmount           int
	StartTime            time.Time
	EstimatedInterceptTime time.Time
	ActualInterceptTime  *time.Time
	DefenseInfo          *DefenseInfo
}

// DefenseInfo는 방어 정보입니다.
type DefenseInfo struct {
	DefenseID          string
	DefenderID         string
	DefenderName       string
	DefenseStartTime   time.Time
	DefenseEndTime     *time.Time
	IsSuccessful       bool
	GoldAmountRecovered int
}

// NewRaidAggregate는 새로운 RaidAggregate를 생성합니다.
func NewRaidAggregate(id string) *RaidAggregate {
	aggregate := &RaidAggregate{
		BaseAggregateRoot: BaseAggregateRoot{
			ID:      id,
			Type:    "Raid",
			version: 0,
			changes: []Event{},
		},
		Status: RaidStatusPreparing,
	}
	return aggregate
}

// Create는 약탈을 생성합니다.
func (r *RaidAggregate) Create(cmd *CreateRaidCommand) error {
	if r.Version() > 0 {
		return fmt.Errorf("raid already exists")
	}

	// 이벤트 생성 및 적용
	event := &RaidCreatedEvent{
		BaseEvent: NewBaseEvent(
			primitive.NewObjectID().Hex(),
			"RaidCreated",
			r.AggregateID(),
			r.AggregateType(),
		),
		AllianceID:       cmd.AllianceID,
		TargetAllianceID: cmd.TargetAllianceID,
		PlayerID:         cmd.PlayerID,
		TransportID:      cmd.TransportID,
		GeneralID:        cmd.GeneralID,
		GoldAmount:       cmd.GoldAmount,
		CreatedAt:        time.Now(),
	}

	r.ApplyChange(event)
	return nil
}

// Start는 약탈을 시작합니다.
func (r *RaidAggregate) Start(cmd *StartRaidCommand) error {
	if r.Status != RaidStatusPreparing {
		return fmt.Errorf("raid cannot be started in its current state: %s", r.Status)
	}

	// 이벤트 생성 및 적용
	event := &RaidStartedEvent{
		BaseEvent: NewBaseEvent(
			primitive.NewObjectID().Hex(),
			"RaidStarted",
			r.AggregateID(),
			r.AggregateType(),
		),
		StartTime:            cmd.StartTime,
		EstimatedInterceptTime: cmd.EstimatedInterceptTime,
	}

	r.ApplyChange(event)
	return nil
}

// Succeed는 약탈 성공을 처리합니다.
func (r *RaidAggregate) Succeed(cmd *RaidSucceedCommand) error {
	if r.Status != RaidStatusInProgress {
		return fmt.Errorf("raid cannot succeed in its current state: %s", r.Status)
	}

	// 이벤트 생성 및 적용
	event := &RaidSucceededEvent{
		BaseEvent: NewBaseEvent(
			primitive.NewObjectID().Hex(),
			"RaidSucceeded",
			r.AggregateID(),
			r.AggregateType(),
		),
		SucceededAt:  cmd.SucceededAt,
		GoldAmount:   cmd.GoldAmount,
	}

	r.ApplyChange(event)
	return nil
}

// Fail는 약탈 실패를 처리합니다.
func (r *RaidAggregate) Fail(cmd *RaidFailCommand) error {
	if r.Status != RaidStatusInProgress {
		return fmt.Errorf("raid cannot fail in its current state: %s", r.Status)
	}

	// 이벤트 생성 및 적용
	event := &RaidFailedEvent{
		BaseEvent: NewBaseEvent(
			primitive.NewObjectID().Hex(),
			"RaidFailed",
			r.AggregateID(),
			r.AggregateType(),
		),
		FailedAt:    cmd.FailedAt,
		DefenseID:   cmd.DefenseID,
		DefenderID:  cmd.DefenderID,
		DefenderName: cmd.DefenderName,
	}

	r.ApplyChange(event)
	return nil
}

// Cancel는 약탈을 취소합니다.
func (r *RaidAggregate) Cancel(cmd *CancelRaidCommand) error {
	if r.Status != RaidStatusPreparing && r.Status != RaidStatusInProgress {
		return fmt.Errorf("raid cannot be canceled in its current state: %s", r.Status)
	}

	// 이벤트 생성 및 적용
	event := &RaidCanceledEvent{
		BaseEvent: NewBaseEvent(
			primitive.NewObjectID().Hex(),
			"RaidCanceled",
			r.AggregateID(),
			r.AggregateType(),
		),
		CanceledAt: cmd.CanceledAt,
		Reason:     cmd.Reason,
	}

	r.ApplyChange(event)
	return nil
}

// ApplyRaidCreatedEvent는 RaidCreatedEvent를 적용합니다.
func (r *RaidAggregate) ApplyRaidCreatedEvent(event *RaidCreatedEvent) {
	r.AllianceID = event.AllianceID
	r.TargetAllianceID = event.TargetAllianceID
	r.PlayerID = event.PlayerID
	r.TransportID = event.TransportID
	r.GeneralID = event.GeneralID
	r.GoldAmount = event.GoldAmount
	r.Status = RaidStatusPreparing
}

// ApplyRaidStartedEvent는 RaidStartedEvent를 적용합니다.
func (r *RaidAggregate) ApplyRaidStartedEvent(event *RaidStartedEvent) {
	r.Status = RaidStatusInProgress
	r.StartTime = event.StartTime
	r.EstimatedInterceptTime = event.EstimatedInterceptTime
}

// ApplyRaidSucceededEvent는 RaidSucceededEvent를 적용합니다.
func (r *RaidAggregate) ApplyRaidSucceededEvent(event *RaidSucceededEvent) {
	r.Status = RaidStatusSuccessful
	r.ActualInterceptTime = &event.SucceededAt
	r.GoldAmount = event.GoldAmount
}

// ApplyRaidFailedEvent는 RaidFailedEvent를 적용합니다.
func (r *RaidAggregate) ApplyRaidFailedEvent(event *RaidFailedEvent) {
	r.Status = RaidStatusFailed
	r.ActualInterceptTime = &event.FailedAt
	
	if r.DefenseInfo == nil {
		r.DefenseInfo = &DefenseInfo{
			DefenseID:    event.DefenseID,
			DefenderID:   event.DefenderID,
			DefenderName: event.DefenderName,
			IsSuccessful: true,
			DefenseEndTime: &event.FailedAt,
		}
	} else {
		r.DefenseInfo.IsSuccessful = true
		r.DefenseInfo.DefenseEndTime = &event.FailedAt
	}
}

// ApplyRaidCanceledEvent는 RaidCanceledEvent를 적용합니다.
func (r *RaidAggregate) ApplyRaidCanceledEvent(event *RaidCanceledEvent) {
	r.Status = RaidStatusCanceled
}
