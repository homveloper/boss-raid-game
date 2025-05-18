package domain

import (
	"time"
)

// TransportCreatedEvent는 이송 생성 이벤트입니다.
type TransportCreatedEvent struct {
	BaseEvent
	AllianceID          string    `json:"alliance_id" bson:"alliance_id"`
	PlayerID            string    `json:"player_id" bson:"player_id"`
	PlayerName          string    `json:"player_name" bson:"player_name"`
	MineID              string    `json:"mine_id" bson:"mine_id"`
	MineName            string    `json:"mine_name" bson:"mine_name"`
	MineLevel           int       `json:"mine_level" bson:"mine_level"`
	GeneralID           string    `json:"general_id" bson:"general_id"`
	GoldAmount          int       `json:"gold_amount" bson:"gold_amount"`
	MaxParticipants     int       `json:"max_participants" bson:"max_participants"`
	PrepTime            int       `json:"prep_time" bson:"prep_time"`
	TransportTime       int       `json:"transport_time" bson:"transport_time"`
	CreatedAt           time.Time `json:"created_at" bson:"created_at"`
}

// TransportStartedEvent는 이송 시작 이벤트입니다.
type TransportStartedEvent struct {
	BaseEvent
	StartTime           time.Time `json:"start_time" bson:"start_time"`
	EstimatedArrivalTime time.Time `json:"estimated_arrival_time" bson:"estimated_arrival_time"`
}

// TransportCompletedEvent는 이송 완료 이벤트입니다.
type TransportCompletedEvent struct {
	BaseEvent
	CompletedAt time.Time `json:"completed_at" bson:"completed_at"`
}

// TransportRaidedEvent는 이송 약탈 이벤트입니다.
type TransportRaidedEvent struct {
	BaseEvent
	RaidID         string    `json:"raid_id" bson:"raid_id"`
	RaiderID       string    `json:"raider_id" bson:"raider_id"`
	RaiderName     string    `json:"raider_name" bson:"raider_name"`
	RaidStartTime  time.Time `json:"raid_start_time" bson:"raid_start_time"`
	DefenseEndTime time.Time `json:"defense_end_time" bson:"defense_end_time"`
}

// TransportDefendedEvent는 이송 방어 이벤트입니다.
type TransportDefendedEvent struct {
	BaseEvent
	DefenderID        string    `json:"defender_id" bson:"defender_id"`
	DefenderName      string    `json:"defender_name" bson:"defender_name"`
	DefenseSuccessful bool      `json:"defense_successful" bson:"defense_successful"`
	DefendedAt        time.Time `json:"defended_at" bson:"defended_at"`
	GoldAmountLost    int       `json:"gold_amount_lost" bson:"gold_amount_lost"`
}

// TransportParticipantAddedEvent는 이송 참여자 추가 이벤트입니다.
type TransportParticipantAddedEvent struct {
	BaseEvent
	PlayerID      string    `json:"player_id" bson:"player_id"`
	PlayerName    string    `json:"player_name" bson:"player_name"`
	GoldAmount    int       `json:"gold_amount" bson:"gold_amount"`
	JoinedAt      time.Time `json:"joined_at" bson:"joined_at"`
}

// TransportRaidCompletedEvent는 이송 약탈 완료 이벤트입니다.
type TransportRaidCompletedEvent struct {
	BaseEvent
	RaidID         string    `json:"raid_id" bson:"raid_id"`
	Successful     bool      `json:"successful" bson:"successful"`
	GoldAmountLost int       `json:"gold_amount_lost" bson:"gold_amount_lost"`
	CompletedAt    time.Time `json:"completed_at" bson:"completed_at"`
}
