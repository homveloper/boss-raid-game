package domain

import (
	"time"
)

// CreateRaidCommand는 약탈 생성 명령입니다.
type CreateRaidCommand struct {
	BaseCommand
	AllianceID       string `json:"alliance_id" bson:"alliance_id"`
	TargetAllianceID string `json:"target_alliance_id" bson:"target_alliance_id"`
	PlayerID         string `json:"player_id" bson:"player_id"`
	PlayerName       string `json:"player_name" bson:"player_name"`
	TransportID      string `json:"transport_id" bson:"transport_id"`
	GeneralID        string `json:"general_id" bson:"general_id"`
	GoldAmount       int    `json:"gold_amount" bson:"gold_amount"`
}

// NewCreateRaidCommand는 새로운 CreateRaidCommand를 생성합니다.
func NewCreateRaidCommand(
	id, allianceID, targetAllianceID, playerID, playerName, transportID, generalID string,
	goldAmount int,
) *CreateRaidCommand {
	return &CreateRaidCommand{
		BaseCommand: NewBaseCommand(
			id,
			"CreateRaid",
			id,
			"Raid",
		),
		AllianceID:       allianceID,
		TargetAllianceID: targetAllianceID,
		PlayerID:         playerID,
		PlayerName:       playerName,
		TransportID:      transportID,
		GeneralID:        generalID,
		GoldAmount:       goldAmount,
	}
}

// StartRaidCommand는 약탈 시작 명령입니다.
type StartRaidCommand struct {
	BaseCommand
	StartTime            time.Time `json:"start_time" bson:"start_time"`
	EstimatedInterceptTime time.Time `json:"estimated_intercept_time" bson:"estimated_intercept_time"`
}

// NewStartRaidCommand는 새로운 StartRaidCommand를 생성합니다.
func NewStartRaidCommand(
	id string,
	startTime, estimatedInterceptTime time.Time,
) *StartRaidCommand {
	return &StartRaidCommand{
		BaseCommand: NewBaseCommand(
			id,
			"StartRaid",
			id,
			"Raid",
		),
		StartTime:            startTime,
		EstimatedInterceptTime: estimatedInterceptTime,
	}
}

// RaidSucceedCommand는 약탈 성공 명령입니다.
type RaidSucceedCommand struct {
	BaseCommand
	SucceededAt time.Time `json:"succeeded_at" bson:"succeeded_at"`
	GoldAmount  int       `json:"gold_amount" bson:"gold_amount"`
}

// NewRaidSucceedCommand는 새로운 RaidSucceedCommand를 생성합니다.
func NewRaidSucceedCommand(
	id string,
	succeededAt time.Time,
	goldAmount int,
) *RaidSucceedCommand {
	return &RaidSucceedCommand{
		BaseCommand: NewBaseCommand(
			id,
			"RaidSucceed",
			id,
			"Raid",
		),
		SucceededAt: succeededAt,
		GoldAmount:  goldAmount,
	}
}

// RaidFailCommand는 약탈 실패 명령입니다.
type RaidFailCommand struct {
	BaseCommand
	FailedAt     time.Time `json:"failed_at" bson:"failed_at"`
	DefenseID    string    `json:"defense_id" bson:"defense_id"`
	DefenderID   string    `json:"defender_id" bson:"defender_id"`
	DefenderName string    `json:"defender_name" bson:"defender_name"`
}

// NewRaidFailCommand는 새로운 RaidFailCommand를 생성합니다.
func NewRaidFailCommand(
	id, defenseID, defenderID, defenderName string,
	failedAt time.Time,
) *RaidFailCommand {
	return &RaidFailCommand{
		BaseCommand: NewBaseCommand(
			id,
			"RaidFail",
			id,
			"Raid",
		),
		FailedAt:     failedAt,
		DefenseID:    defenseID,
		DefenderID:   defenderID,
		DefenderName: defenderName,
	}
}

// CancelRaidCommand는 약탈 취소 명령입니다.
type CancelRaidCommand struct {
	BaseCommand
	CanceledAt time.Time `json:"canceled_at" bson:"canceled_at"`
	Reason     string    `json:"reason" bson:"reason"`
}

// NewCancelRaidCommand는 새로운 CancelRaidCommand를 생성합니다.
func NewCancelRaidCommand(
	id, reason string,
	canceledAt time.Time,
) *CancelRaidCommand {
	return &CancelRaidCommand{
		BaseCommand: NewBaseCommand(
			id,
			"CancelRaid",
			id,
			"Raid",
		),
		CanceledAt: canceledAt,
		Reason:     reason,
	}
}
