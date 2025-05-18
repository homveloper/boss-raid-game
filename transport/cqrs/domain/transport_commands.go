package domain

import (
	"time"
)

// CreateTransportCommand는 이송 생성 명령입니다.
type CreateTransportCommand struct {
	BaseCommand
	AllianceID      string `json:"alliance_id" bson:"alliance_id"`
	PlayerID        string `json:"player_id" bson:"player_id"`
	PlayerName      string `json:"player_name" bson:"player_name"`
	MineID          string `json:"mine_id" bson:"mine_id"`
	MineName        string `json:"mine_name" bson:"mine_name"`
	MineLevel       int    `json:"mine_level" bson:"mine_level"`
	GeneralID       string `json:"general_id" bson:"general_id"`
	GoldAmount      int    `json:"gold_amount" bson:"gold_amount"`
	MaxParticipants int    `json:"max_participants" bson:"max_participants"`
	PrepTime        int    `json:"prep_time" bson:"prep_time"`
	TransportTime   int    `json:"transport_time" bson:"transport_time"`
}

// NewCreateTransportCommand는 새로운 CreateTransportCommand를 생성합니다.
func NewCreateTransportCommand(
	id, allianceID, playerID, playerName, mineID, mineName string,
	mineLevel int, generalID string, goldAmount, maxParticipants, prepTime, transportTime int,
) *CreateTransportCommand {
	return &CreateTransportCommand{
		BaseCommand: NewBaseCommand(
			id,
			"CreateTransport",
			id,
			"Transport",
		),
		AllianceID:      allianceID,
		PlayerID:        playerID,
		PlayerName:      playerName,
		MineID:          mineID,
		MineName:        mineName,
		MineLevel:       mineLevel,
		GeneralID:       generalID,
		GoldAmount:      goldAmount,
		MaxParticipants: maxParticipants,
		PrepTime:        prepTime,
		TransportTime:   transportTime,
	}
}

// StartTransportCommand는 이송 시작 명령입니다.
type StartTransportCommand struct {
	BaseCommand
	StartTime           time.Time `json:"start_time" bson:"start_time"`
	EstimatedArrivalTime time.Time `json:"estimated_arrival_time" bson:"estimated_arrival_time"`
}

// NewStartTransportCommand는 새로운 StartTransportCommand를 생성합니다.
func NewStartTransportCommand(
	id string,
	startTime, estimatedArrivalTime time.Time,
) *StartTransportCommand {
	return &StartTransportCommand{
		BaseCommand: NewBaseCommand(
			id,
			"StartTransport",
			id,
			"Transport",
		),
		StartTime:           startTime,
		EstimatedArrivalTime: estimatedArrivalTime,
	}
}

// CompleteTransportCommand는 이송 완료 명령입니다.
type CompleteTransportCommand struct {
	BaseCommand
	CompletedAt time.Time `json:"completed_at" bson:"completed_at"`
}

// NewCompleteTransportCommand는 새로운 CompleteTransportCommand를 생성합니다.
func NewCompleteTransportCommand(
	id string,
	completedAt time.Time,
) *CompleteTransportCommand {
	return &CompleteTransportCommand{
		BaseCommand: NewBaseCommand(
			id,
			"CompleteTransport",
			id,
			"Transport",
		),
		CompletedAt: completedAt,
	}
}

// RaidTransportCommand는 이송 약탈 명령입니다.
type RaidTransportCommand struct {
	BaseCommand
	RaidID         string    `json:"raid_id" bson:"raid_id"`
	RaiderID       string    `json:"raider_id" bson:"raider_id"`
	RaiderName     string    `json:"raider_name" bson:"raider_name"`
	RaidStartTime  time.Time `json:"raid_start_time" bson:"raid_start_time"`
	DefenseEndTime time.Time `json:"defense_end_time" bson:"defense_end_time"`
}

// NewRaidTransportCommand는 새로운 RaidTransportCommand를 생성합니다.
func NewRaidTransportCommand(
	id, raidID, raiderID, raiderName string,
	raidStartTime, defenseEndTime time.Time,
) *RaidTransportCommand {
	return &RaidTransportCommand{
		BaseCommand: NewBaseCommand(
			id,
			"RaidTransport",
			id,
			"Transport",
		),
		RaidID:         raidID,
		RaiderID:       raiderID,
		RaiderName:     raiderName,
		RaidStartTime:  raidStartTime,
		DefenseEndTime: defenseEndTime,
	}
}

// DefendTransportCommand는 이송 방어 명령입니다.
type DefendTransportCommand struct {
	BaseCommand
	DefenderID        string    `json:"defender_id" bson:"defender_id"`
	DefenderName      string    `json:"defender_name" bson:"defender_name"`
	Successful        bool      `json:"successful" bson:"successful"`
	DefendedAt        time.Time `json:"defended_at" bson:"defended_at"`
	GoldAmountLost    int       `json:"gold_amount_lost" bson:"gold_amount_lost"`
}

// NewDefendTransportCommand는 새로운 DefendTransportCommand를 생성합니다.
func NewDefendTransportCommand(
	id, defenderID, defenderName string,
	successful bool,
	defendedAt time.Time,
	goldAmountLost int,
) *DefendTransportCommand {
	return &DefendTransportCommand{
		BaseCommand: NewBaseCommand(
			id,
			"DefendTransport",
			id,
			"Transport",
		),
		DefenderID:     defenderID,
		DefenderName:   defenderName,
		Successful:     successful,
		DefendedAt:     defendedAt,
		GoldAmountLost: goldAmountLost,
	}
}

// AddParticipantCommand는 이송 참여자 추가 명령입니다.
type AddParticipantCommand struct {
	BaseCommand
	PlayerID      string    `json:"player_id" bson:"player_id"`
	PlayerName    string    `json:"player_name" bson:"player_name"`
	GoldAmount    int       `json:"gold_amount" bson:"gold_amount"`
	JoinedAt      time.Time `json:"joined_at" bson:"joined_at"`
}

// NewAddParticipantCommand는 새로운 AddParticipantCommand를 생성합니다.
func NewAddParticipantCommand(
	id, playerID, playerName string,
	goldAmount int,
	joinedAt time.Time,
) *AddParticipantCommand {
	return &AddParticipantCommand{
		BaseCommand: NewBaseCommand(
			id,
			"AddParticipant",
			id,
			"Transport",
		),
		PlayerID:   playerID,
		PlayerName: playerName,
		GoldAmount: goldAmount,
		JoinedAt:   joinedAt,
	}
}
