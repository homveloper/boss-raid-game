package application

import (
	"context"
	"fmt"
	"tictactoe/transport/cqrs/domain"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TransportReadModel은 이송 읽기 모델입니다.
type TransportReadModel struct {
	ID                   string             `bson:"_id"`
	AllianceID           string             `bson:"alliance_id"`
	PlayerID             string             `bson:"player_id"`
	MineID               string             `bson:"mine_id"`
	MineName             string             `bson:"mine_name"`
	MineLevel            int                `bson:"mine_level"`
	GeneralID            string             `bson:"general_id"`
	Status               string             `bson:"status"`
	GoldAmount           int                `bson:"gold_amount"`
	MaxParticipants      int                `bson:"max_participants"`
	Participants         []ParticipantModel `bson:"participants"`
	PrepStartTime        time.Time          `bson:"prep_start_time"`
	PrepEndTime          time.Time          `bson:"prep_end_time"`
	StartTime            *time.Time         `bson:"start_time"`
	EstimatedArrivalTime *time.Time         `bson:"estimated_arrival_time"`
	ActualArrivalTime    *time.Time         `bson:"actual_arrival_time"`
	RaidInfo             *RaidInfoModel     `bson:"raid_info"`
	CreatedAt            time.Time          `bson:"created_at"`
	UpdatedAt            time.Time          `bson:"updated_at"`
	Version              int                `bson:"version"`
}

// ParticipantModel은 이송 참여자 모델입니다.
type ParticipantModel struct {
	PlayerID   string    `bson:"player_id"`
	PlayerName string    `bson:"player_name"`
	GoldAmount int       `bson:"gold_amount"`
	JoinedAt   time.Time `bson:"joined_at"`
}

// RaidInfoModel은 약탈 정보 모델입니다.
type RaidInfoModel struct {
	RaidID             string     `bson:"raid_id"`
	RaiderID           string     `bson:"raider_id"`
	RaiderName         string     `bson:"raider_name"`
	RaidStartTime      time.Time  `bson:"raid_start_time"`
	DefenseEndTime     time.Time  `bson:"defense_end_time"`
	IsDefended         bool       `bson:"is_defended"`
	DefenseSuccessful  bool       `bson:"defense_successful"`
	DefenderID         string     `bson:"defender_id"`
	DefenderName       string     `bson:"defender_name"`
	DefenseCompletedAt *time.Time `bson:"defense_completed_at"`
	GoldAmountLost     int        `bson:"gold_amount_lost"`
}

// TransportProjector는 이송 이벤트를 처리하여 읽기 모델을 업데이트하는 프로젝터입니다.
type TransportProjector struct {
	collection *mongo.Collection
	processed  *mongo.Collection
}

// NewTransportProjector는 새로운 TransportProjector를 생성합니다.
func NewTransportProjector(
	client *mongo.Client,
	database string,
	collection string,
	processedCollection string,
) *TransportProjector {
	return &TransportProjector{
		collection: client.Database(database).Collection(collection),
		processed:  client.Database(database).Collection(processedCollection),
	}
}

// HandleEvent는 이벤트를 처리합니다.
func (p *TransportProjector) HandleEvent(ctx context.Context, event domain.Event) error {
	// 이벤트 ID 생성
	eventID := event.AggregateID() + "-" + fmt.Sprintf("%d", event.Version())

	// 이미 처리된 이벤트인지 확인
	var processed struct {
		ID string `bson:"_id"`
	}
	err := p.processed.FindOne(ctx, bson.M{"_id": eventID}).Decode(&processed)
	if err == nil {
		// 이미 처리된 이벤트
		return nil
	} else if err != mongo.ErrNoDocuments {
		return fmt.Errorf("failed to check processed event: %w", err)
	}

	// 이벤트 타입에 따라 처리
	var handlerErr error
	switch e := event.(type) {
	case *domain.TransportCreatedEvent:
		handlerErr = p.handleTransportCreated(ctx, e)
	case *domain.TransportStartedEvent:
		handlerErr = p.handleTransportStarted(ctx, e)
	case *domain.TransportCompletedEvent:
		handlerErr = p.handleTransportCompleted(ctx, e)
	case *domain.TransportRaidedEvent:
		handlerErr = p.handleTransportRaided(ctx, e)
	case *domain.TransportDefendedEvent:
		handlerErr = p.handleTransportDefended(ctx, e)
	case *domain.TransportParticipantAddedEvent:
		handlerErr = p.handleTransportParticipantAdded(ctx, e)
	}

	if handlerErr != nil {
		return handlerErr
	}

	// 처리된 이벤트 기록
	_, err = p.processed.InsertOne(ctx, bson.M{
		"_id":       eventID,
		"timestamp": time.Now(),
	})

	return err
}

// handleTransportCreated는 TransportCreatedEvent를 처리합니다.
func (p *TransportProjector) handleTransportCreated(
	ctx context.Context,
	event *domain.TransportCreatedEvent,
) error {
	// 준비 종료 시간 계산
	prepEndTime := event.CreatedAt.Add(time.Duration(event.PrepTime) * time.Minute)

	// 읽기 모델 생성
	model := TransportReadModel{
		ID:              event.AggregateID(),
		AllianceID:      event.AllianceID,
		PlayerID:        event.PlayerID,
		MineID:          event.MineID,
		MineName:        event.MineName,
		MineLevel:       event.MineLevel,
		GeneralID:       event.GeneralID,
		Status:          string(domain.TransportStatusPreparing),
		GoldAmount:      event.GoldAmount,
		MaxParticipants: event.MaxParticipants,
		Participants: []ParticipantModel{
			{
				PlayerID:   event.PlayerID,
				PlayerName: event.PlayerName,
				GoldAmount: event.GoldAmount,
				JoinedAt:   event.CreatedAt,
			},
		},
		PrepStartTime: event.CreatedAt,
		PrepEndTime:   prepEndTime,
		CreatedAt:     event.CreatedAt,
		UpdatedAt:     event.CreatedAt,
		Version:       event.Version(),
	}

	// 낙관적 동시성 제어를 위한 옵션
	opts := options.FindOneAndReplace().SetUpsert(true)

	// 읽기 모델 저장
	result := p.collection.FindOneAndReplace(
		ctx,
		bson.M{"_id": model.ID, "version": bson.M{"$lt": model.Version}},
		model,
		opts,
	)

	if result.Err() != nil && result.Err() != mongo.ErrNoDocuments {
		return fmt.Errorf("failed to save transport read model: %w", result.Err())
	}

	return nil
}

// handleTransportStarted는 TransportStartedEvent를 처리합니다.
func (p *TransportProjector) handleTransportStarted(
	ctx context.Context,
	event *domain.TransportStartedEvent,
) error {
	// 낙관적 동시성 제어를 위한 옵션
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	// 읽기 모델 업데이트
	var model TransportReadModel
	err := p.collection.FindOneAndUpdate(
		ctx,
		bson.M{
			"_id":     event.AggregateID(),
			"version": bson.M{"$lt": event.Version()},
		},
		bson.M{
			"$set": bson.M{
				"status":                 string(domain.TransportStatusInTransit),
				"start_time":             event.StartTime,
				"estimated_arrival_time": event.EstimatedArrivalTime,
				"updated_at":             time.Now(),
				"version":                event.Version(),
			},
		},
		opts,
	).Decode(&model)

	if err != nil {
		return fmt.Errorf("failed to update transport read model: %w", err)
	}

	return nil
}

// handleTransportCompleted는 TransportCompletedEvent를 처리합니다.
func (p *TransportProjector) handleTransportCompleted(
	ctx context.Context,
	event *domain.TransportCompletedEvent,
) error {
	// 낙관적 동시성 제어를 위한 옵션
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	// 읽기 모델 업데이트
	var model TransportReadModel
	err := p.collection.FindOneAndUpdate(
		ctx,
		bson.M{
			"_id":     event.AggregateID(),
			"version": bson.M{"$lt": event.Version()},
		},
		bson.M{
			"$set": bson.M{
				"status":              string(domain.TransportStatusCompleted),
				"actual_arrival_time": event.CompletedAt,
				"updated_at":          time.Now(),
				"version":             event.Version(),
			},
		},
		opts,
	).Decode(&model)

	if err != nil {
		return fmt.Errorf("failed to update transport read model: %w", err)
	}

	return nil
}

// handleTransportRaided는 TransportRaidedEvent를 처리합니다.
func (p *TransportProjector) handleTransportRaided(
	ctx context.Context,
	event *domain.TransportRaidedEvent,
) error {
	// 낙관적 동시성 제어를 위한 옵션
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	// 읽기 모델 업데이트
	var model TransportReadModel
	err := p.collection.FindOneAndUpdate(
		ctx,
		bson.M{
			"_id":     event.AggregateID(),
			"version": bson.M{"$lt": event.Version()},
		},
		bson.M{
			"$set": bson.M{
				"raid_info": bson.M{
					"raid_id":          event.RaidID,
					"raider_id":        event.RaiderID,
					"raider_name":      event.RaiderName,
					"raid_start_time":  event.RaidStartTime,
					"defense_end_time": event.DefenseEndTime,
					"is_defended":      false,
				},
				"updated_at": time.Now(),
				"version":    event.Version(),
			},
		},
		opts,
	).Decode(&model)

	if err != nil {
		return fmt.Errorf("failed to update transport read model: %w", err)
	}

	return nil
}

// handleTransportDefended는 TransportDefendedEvent를 처리합니다.
func (p *TransportProjector) handleTransportDefended(
	ctx context.Context,
	event *domain.TransportDefendedEvent,
) error {
	// 낙관적 동시성 제어를 위한 옵션
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	// 읽기 모델 업데이트
	update := bson.M{
		"$set": bson.M{
			"raid_info.is_defended":          true,
			"raid_info.defense_successful":   event.DefenseSuccessful,
			"raid_info.defender_id":          event.DefenderID,
			"raid_info.defender_name":        event.DefenderName,
			"raid_info.defense_completed_at": event.DefendedAt,
			"raid_info.gold_amount_lost":     event.GoldAmountLost,
			"updated_at":                     time.Now(),
			"version":                        event.Version(),
		},
	}

	// 방어 실패로 인한 약탈 성공 시 상태 변경
	if !event.DefenseSuccessful {
		// 현재 모델 조회
		var currentModel TransportReadModel
		err := p.collection.FindOne(ctx, bson.M{"_id": event.AggregateID()}).Decode(&currentModel)
		if err != nil {
			return fmt.Errorf("failed to find transport read model: %w", err)
		}

		// 골드 감소
		newGoldAmount := currentModel.GoldAmount - event.GoldAmountLost
		update["$set"].(bson.M)["gold_amount"] = newGoldAmount

		// 모든 골드를 잃었으면 상태 변경
		if newGoldAmount <= 0 {
			update["$set"].(bson.M)["status"] = string(domain.TransportStatusRaided)
		}
	}

	// 읽기 모델 업데이트
	var model TransportReadModel
	err := p.collection.FindOneAndUpdate(
		ctx,
		bson.M{
			"_id":     event.AggregateID(),
			"version": bson.M{"$lt": event.Version()},
		},
		update,
		opts,
	).Decode(&model)

	if err != nil {
		return fmt.Errorf("failed to update transport read model: %w", err)
	}

	return nil
}

// handleTransportParticipantAdded는 TransportParticipantAddedEvent를 처리합니다.
func (p *TransportProjector) handleTransportParticipantAdded(
	ctx context.Context,
	event *domain.TransportParticipantAddedEvent,
) error {
	// 낙관적 동시성 제어를 위한 옵션
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	// 읽기 모델 업데이트
	var model TransportReadModel
	err := p.collection.FindOneAndUpdate(
		ctx,
		bson.M{
			"_id":     event.AggregateID(),
			"version": bson.M{"$lt": event.Version()},
		},
		bson.M{
			"$push": bson.M{
				"participants": bson.M{
					"player_id":   event.PlayerID,
					"player_name": event.PlayerName,
					"gold_amount": event.GoldAmount,
					"joined_at":   event.JoinedAt,
				},
			},
			"$inc": bson.M{
				"gold_amount": event.GoldAmount,
			},
			"$set": bson.M{
				"updated_at": time.Now(),
				"version":    event.Version(),
			},
		},
		opts,
	).Decode(&model)

	if err != nil {
		return fmt.Errorf("failed to update transport read model: %w", err)
	}

	return nil
}
