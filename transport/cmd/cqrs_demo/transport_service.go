package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"tictactoe/transport/cqrs/application"
	"tictactoe/transport/cqrs/domain"
	"tictactoe/transport/cqrs/infrastructure"
)

// TransportService는 이송 관련 서비스를 제공합니다.
type TransportService struct {
	commandBus infrastructure.CommandBus
	queryStore *mongo.Collection
}

// NewTransportService는 새로운 TransportService를 생성합니다.
func NewTransportService(
	commandBus infrastructure.CommandBus,
	queryStore *mongo.Collection,
) *TransportService {
	return &TransportService{
		commandBus: commandBus,
		queryStore: queryStore,
	}
}

// CreateTransportRequest는 이송 생성 요청 구조체입니다.
type CreateTransportRequest struct {
	AllianceID      string `json:"alliance_id"`
	PlayerID        string `json:"player_id"`
	PlayerName      string `json:"player_name"`
	MineID          string `json:"mine_id"`
	MineName        string `json:"mine_name"`
	MineLevel       int    `json:"mine_level"`
	GeneralID       string `json:"general_id"`
	GoldAmount      int    `json:"gold_amount"`
	MaxParticipants int    `json:"max_participants"`
	PrepTime        int    `json:"prep_time"`
	TransportTime   int    `json:"transport_time"`
}

// CreateTransportHandler는 이송 생성 요청을 처리합니다.
func (s *TransportService) CreateTransportHandler(w http.ResponseWriter, r *http.Request) {
	var req CreateTransportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 비즈니스 규칙 검증 (실제 구현에서는 더 복잡할 수 있음)
	if req.GoldAmount <= 0 {
		http.Error(w, "gold amount must be positive", http.StatusBadRequest)
		return
	}

	// 새 ID 생성
	id := generateID()

	// 커맨드 생성
	cmd := domain.NewCreateTransportCommand(
		id,
		req.AllianceID,
		req.PlayerID,
		req.PlayerName,
		req.MineID,
		req.MineName,
		req.MineLevel,
		req.GeneralID,
		req.GoldAmount,
		req.MaxParticipants,
		req.PrepTime,
		req.TransportTime,
	)

	// 커맨드 전송
	if err := s.commandBus.Dispatch(r.Context(), cmd); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 응답 반환
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

// JoinTransportRequest는 이송 참가 요청 구조체입니다.
type JoinTransportRequest struct {
	PlayerID   string `json:"player_id"`
	PlayerName string `json:"player_name"`
	GoldAmount int    `json:"gold_amount"`
}

// JoinTransportHandler는 이송 참가 요청을 처리합니다.
func (s *TransportService) JoinTransportHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	transportID := vars["id"]

	var req JoinTransportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 비즈니스 규칙 검증
	if req.GoldAmount <= 0 {
		http.Error(w, "gold amount must be positive", http.StatusBadRequest)
		return
	}

	// 이송 정보 조회
	var transport application.TransportReadModel
	if err := s.queryStore.FindOne(r.Context(), bson.M{"_id": transportID}).Decode(&transport); err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "transport not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// 이송 상태 확인
	if transport.Status != "PREPARING" {
		http.Error(w, "transport is not in preparation phase", http.StatusBadRequest)
		return
	}

	// 참가자 수 확인
	if len(transport.Participants) >= transport.MaxParticipants {
		http.Error(w, "transport is full", http.StatusBadRequest)
		return
	}

	// 이미 참가 중인지 확인
	for _, p := range transport.Participants {
		if p.PlayerID == req.PlayerID {
			http.Error(w, "player is already participating in this transport", http.StatusBadRequest)
			return
		}
	}

	// 티켓 사용 (실제 구현에서는 티켓 서비스 호출)
	if err := s.useTicket(r.Context(), req.PlayerID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 커맨드 생성
	cmd := domain.NewAddParticipantCommand(
		transportID,
		req.PlayerID,
		req.PlayerName,
		req.GoldAmount,
		time.Now(),
	)

	// 커맨드 전송
	if err := s.commandBus.Dispatch(r.Context(), cmd); err != nil {
		// 실패 시 티켓 환불 (실제 구현에서는 보상 트랜잭션)
		if refundErr := s.refundTicket(r.Context(), req.PlayerID); refundErr != nil {
			log.Printf("Failed to refund ticket: %v", refundErr)
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 참가 후 이송 정보 다시 조회
	if err := s.queryStore.FindOne(r.Context(), bson.M{"_id": transportID}).Decode(&transport); err != nil {
		// 이미 커맨드는 성공했으므로 에러만 로깅
		log.Printf("Failed to fetch updated transport: %v", err)
	} else {
		// 참가자 수가 최대에 도달했는지 확인
		if len(transport.Participants) >= transport.MaxParticipants {
			// 이송 시작 커맨드 생성
			now := time.Now()
			estimatedArrivalTime := now.Add(time.Duration(60) * time.Minute) // 기본값 60분
			startCmd := domain.NewStartTransportCommand(
				transportID,
				now,
				estimatedArrivalTime,
			)

			// 커맨드 전송 (비동기로 처리)
			go func() {
				ctx := context.Background()
				if err := s.commandBus.Dispatch(ctx, startCmd); err != nil {
					log.Printf("Failed to start transport automatically: %v", err)
				} else {
					log.Printf("Transport %s started automatically after reaching max participants", transportID)
				}
			}()
		}
	}

	// 응답 반환
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "joined",
		"id":     transportID,
	})
}

// StartTransportHandler는 이송 시작 요청을 처리합니다.
func (s *TransportService) StartTransportHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// 현재 시간
	now := time.Now()

	// 이송 정보 조회
	var transport application.TransportReadModel
	if err := s.queryStore.FindOne(r.Context(), map[string]interface{}{"_id": id}).Decode(&transport); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// 예상 도착 시간 계산 (실제 구현에서는 더 복잡할 수 있음)
	estimatedArrivalTime := now.Add(time.Duration(60) * time.Minute) // 기본값 60분

	// 커맨드 생성
	cmd := domain.NewStartTransportCommand(
		id,
		now,
		estimatedArrivalTime,
	)

	// 커맨드 전송
	if err := s.commandBus.Dispatch(r.Context(), cmd); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 응답 반환
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

// GetTransportHandler는 이송 조회 요청을 처리합니다.
func (s *TransportService) GetTransportHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// 이송 정보 조회
	var transport application.TransportReadModel
	if err := s.queryStore.FindOne(r.Context(), map[string]interface{}{"_id": id}).Decode(&transport); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// 응답 반환
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(transport)
}

// GetActiveTransportsHandler는 활성 이송 목록 조회 요청을 처리합니다.
func (s *TransportService) GetActiveTransportsHandler(w http.ResponseWriter, r *http.Request) {
	allianceID := r.URL.Query().Get("alliance_id")
	if allianceID == "" {
		http.Error(w, "alliance_id is required", http.StatusBadRequest)
		return
	}

	// 활성 상태 이송 조회
	cursor, err := s.queryStore.Find(r.Context(), map[string]interface{}{
		"alliance_id": allianceID,
		"status": map[string]interface{}{
			"$in": []string{"PREPARING", "IN_TRANSIT"},
		},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(r.Context())

	// 결과 변환
	var transports []application.TransportReadModel
	if err := cursor.All(r.Context(), &transports); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 응답 반환
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(transports)
}

// RaidTransportRequest는 이송 약탈 요청 구조체입니다.
type RaidTransportRequest struct {
	RaiderID   string `json:"raider_id"`
	RaiderName string `json:"raider_name"`
}

// RaidTransportHandler는 이송 약탈 요청을 처리합니다.
func (s *TransportService) RaidTransportHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var req RaidTransportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 새 약탈 ID 생성
	raidID := generateID()

	// 현재 시간
	now := time.Now()

	// 방어 종료 시간 (30분 후)
	defenseEndTime := now.Add(30 * time.Minute)

	// 커맨드 생성
	cmd := domain.NewRaidTransportCommand(
		id,
		raidID,
		req.RaiderID,
		req.RaiderName,
		now,
		defenseEndTime,
	)

	// 커맨드 전송
	if err := s.commandBus.Dispatch(r.Context(), cmd); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 응답 반환
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "raiding", "raid_id": raidID})
}

// DefendTransportRequest는 이송 방어 요청 구조체입니다.
type DefendTransportRequest struct {
	DefenderID   string `json:"defender_id"`
	DefenderName string `json:"defender_name"`
	Successful   bool   `json:"successful"`
}

// DefendTransportHandler는 이송 방어 요청을 처리합니다.
func (s *TransportService) DefendTransportHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var req DefendTransportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 현재 시간
	now := time.Now()

	// 이송 정보 조회
	var transport application.TransportReadModel
	if err := s.queryStore.FindOne(r.Context(), map[string]interface{}{"_id": id}).Decode(&transport); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// 골드 손실량 계산 (방어 실패 시 30%)
	goldAmountLost := 0
	if !req.Successful {
		goldAmountLost = int(float64(transport.GoldAmount) * 0.3)
	}

	// 커맨드 생성
	cmd := domain.NewDefendTransportCommand(
		id,
		req.DefenderID,
		req.DefenderName,
		req.Successful,
		now,
		goldAmountLost,
	)

	// 커맨드 전송
	if err := s.commandBus.Dispatch(r.Context(), cmd); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 응답 반환
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "defended"})
}

// useTicket은 플레이어의 티켓을 사용합니다.
func (s *TransportService) useTicket(ctx context.Context, playerID string) error {
	// 실제 구현에서는 티켓 서비스 호출
	// 여기서는 간단히 성공으로 처리
	return nil
}

// refundTicket은 플레이어에게 티켓을 환불합니다.
func (s *TransportService) refundTicket(ctx context.Context, playerID string) error {
	// 실제 구현에서는 티켓 서비스 호출
	// 여기서는 간단히 성공으로 처리
	return nil
}

// generateID는 고유 ID를 생성합니다.
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
