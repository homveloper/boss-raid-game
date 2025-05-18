package main

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"tictactoe/transport/cqrs/application"
	"tictactoe/transport/cqrs/domain"
	"tictactoe/transport/cqrs/infrastructure"
)

// Scheduler는 시간 기반 이벤트를 처리하는 스케줄러입니다.
type Scheduler struct {
	commandBus infrastructure.CommandBus
	queryStore *mongo.Collection
	interval   time.Duration
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewScheduler는 새로운 Scheduler를 생성합니다.
func NewScheduler(
	commandBus infrastructure.CommandBus,
	queryStore *mongo.Collection,
	interval time.Duration,
) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		commandBus: commandBus,
		queryStore: queryStore,
		interval:   interval,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start는 스케줄러를 시작합니다.
func (s *Scheduler) Start() {
	log.Println("Starting scheduler...")
	go s.processCompletedTransports()
	go s.processExpiredDefenses()
	go s.processExpiredPreparations()
}

// Stop은 스케줄러를 중지합니다.
func (s *Scheduler) Stop() {
	log.Println("Stopping scheduler...")
	s.cancel()
}

// processCompletedTransports는 완료된 이송을 처리합니다.
func (s *Scheduler) processCompletedTransports() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.checkCompletedTransports()
		}
	}
}

// checkCompletedTransports는 완료된 이송을 확인하고 처리합니다.
func (s *Scheduler) checkCompletedTransports() {
	now := time.Now()

	// 완료 예정 시간이 지난 진행 중인 이송 조회
	filter := bson.M{
		"status":    "IN_TRANSIT",
		"end_time":  bson.M{"$lte": now},
		"raid_info": nil, // 약탈 중이 아닌 이송만
	}

	cursor, err := s.queryStore.Find(s.ctx, filter)
	if err != nil {
		log.Printf("Failed to query completed transports: %v", err)
		return
	}
	defer cursor.Close(s.ctx)

	var transports []application.TransportReadModel
	if err := cursor.All(s.ctx, &transports); err != nil {
		log.Printf("Failed to decode transports: %v", err)
		return
	}

	for _, transport := range transports {
		// 이송 완료 커맨드 생성
		cmd := domain.NewCompleteTransportCommand(
			transport.ID,
			now,
		)

		// 커맨드 전송
		if err := s.commandBus.Dispatch(s.ctx, cmd); err != nil {
			log.Printf("Failed to complete transport %s: %v", transport.ID, err)
			continue
		}

		log.Printf("Completed transport %s", transport.ID)
	}
}

// processExpiredDefenses는 만료된 방어 시간을 처리합니다.
func (s *Scheduler) processExpiredDefenses() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.checkExpiredDefenses()
		}
	}
}

// checkExpiredDefenses는 만료된 방어 시간을 확인하고 처리합니다.
func (s *Scheduler) checkExpiredDefenses() {
	now := time.Now()

	// 방어 시간이 만료된 약탈 중인 이송 조회
	filter := bson.M{
		"status": "IN_TRANSIT",
		"raid_info": bson.M{
			"$ne": nil,
		},
		"raid_info.is_defended":      false,
		"raid_info.defense_end_time": bson.M{"$lte": now},
	}

	cursor, err := s.queryStore.Find(s.ctx, filter)
	if err != nil {
		log.Printf("Failed to query expired defenses: %v", err)
		return
	}
	defer cursor.Close(s.ctx)

	var transports []application.TransportReadModel
	if err := cursor.All(s.ctx, &transports); err != nil {
		log.Printf("Failed to decode transports: %v", err)
		return
	}

	for _, transport := range transports {
		if transport.RaidInfo == nil {
			continue
		}

		// 약탈 성공 처리 (방어 시간 만료)
		// 골드 손실량 계산 (50%)
		goldAmountLost := int(float64(transport.GoldAmount) * 0.5)

		// 약탈 성공 커맨드 생성
		raidSucceedCmd := domain.NewRaidSucceedCommand(
			transport.RaidInfo.RaidID,
			now,
			goldAmountLost,
		)

		// 커맨드 전송
		if err := s.commandBus.Dispatch(s.ctx, raidSucceedCmd); err != nil {
			log.Printf("Failed to succeed raid %s: %v", transport.RaidInfo.RaidID, err)
			continue
		}

		// 이송 약탈 완료 처리
		transportRaidedCmd := domain.NewDefendTransportCommand(
			transport.ID,
			"",    // 방어자 없음
			"",    // 방어자 이름 없음
			false, // 방어 실패
			now,
			goldAmountLost,
		)

		// 커맨드 전송
		if err := s.commandBus.Dispatch(s.ctx, transportRaidedCmd); err != nil {
			log.Printf("Failed to raid transport %s: %v", transport.ID, err)
			continue
		}

		log.Printf("Processed expired defense for transport %s", transport.ID)
	}
}

// processExpiredPreparations는 준비 시간이 만료된 이송을 처리합니다.
func (s *Scheduler) processExpiredPreparations() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.checkExpiredPreparations()
		}
	}
}

// checkExpiredPreparations는 준비 시간이 만료된 이송을 확인하고 처리합니다.
func (s *Scheduler) checkExpiredPreparations() {
	now := time.Now()

	// 준비 시간이 만료된 이송 조회
	filter := bson.M{
		"status":        "PREPARING",
		"prep_end_time": bson.M{"$lte": now},
	}

	cursor, err := s.queryStore.Find(s.ctx, filter)
	if err != nil {
		log.Printf("Failed to query expired preparations: %v", err)
		return
	}
	defer cursor.Close(s.ctx)

	var transports []application.TransportReadModel
	if err := cursor.All(s.ctx, &transports); err != nil {
		log.Printf("Failed to decode transports: %v", err)
		return
	}

	for _, transport := range transports {
		// 이송 시작 커맨드 생성
		// 예상 도착 시간 계산 (기본값 60분)
		transportTime := 60 * time.Minute

		// 실제 구현에서는 이송 시간을 데이터베이스에서 조회하거나 설정에서 가져와야 함
		estimatedArrivalTime := now.Add(transportTime)
		cmd := domain.NewStartTransportCommand(
			transport.ID,
			now,
			estimatedArrivalTime,
		)

		// 커맨드 전송
		if err := s.commandBus.Dispatch(s.ctx, cmd); err != nil {
			log.Printf("Failed to start transport %s after prep time expired: %v", transport.ID, err)
			continue
		}

		log.Printf("Started transport %s after prep time expired", transport.ID)
	}
}
