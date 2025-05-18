package domain

import (
	"time"
)

// RaidCreatedEvent는 약탈 생성 이벤트입니다.
type RaidCreatedEvent struct {
	BaseEvent
	AllianceID       string    `json:"alliance_id" bson:"alliance_id"`
	TargetAllianceID string    `json:"target_alliance_id" bson:"target_alliance_id"`
	PlayerID         string    `json:"player_id" bson:"player_id"`
	PlayerName       string    `json:"player_name" bson:"player_name"`
	TransportID      string    `json:"transport_id" bson:"transport_id"`
	GeneralID        string    `json:"general_id" bson:"general_id"`
	GoldAmount       int       `json:"gold_amount" bson:"gold_amount"`
	CreatedAt        time.Time `json:"created_at" bson:"created_at"`
}

// RaidStartedEvent는 약탈 시작 이벤트입니다.
type RaidStartedEvent struct {
	BaseEvent
	StartTime            time.Time `json:"start_time" bson:"start_time"`
	EstimatedInterceptTime time.Time `json:"estimated_intercept_time" bson:"estimated_intercept_time"`
}

// RaidSucceededEvent는 약탈 성공 이벤트입니다.
type RaidSucceededEvent struct {
	BaseEvent
	SucceededAt time.Time `json:"succeeded_at" bson:"succeeded_at"`
	GoldAmount  int       `json:"gold_amount" bson:"gold_amount"`
}

// RaidFailedEvent는 약탈 실패 이벤트입니다.
type RaidFailedEvent struct {
	BaseEvent
	FailedAt     time.Time `json:"failed_at" bson:"failed_at"`
	DefenseID    string    `json:"defense_id" bson:"defense_id"`
	DefenderID   string    `json:"defender_id" bson:"defender_id"`
	DefenderName string    `json:"defender_name" bson:"defender_name"`
}

// RaidCanceledEvent는 약탈 취소 이벤트입니다.
type RaidCanceledEvent struct {
	BaseEvent
	CanceledAt time.Time `json:"canceled_at" bson:"canceled_at"`
	Reason     string    `json:"reason" bson:"reason"`
}
