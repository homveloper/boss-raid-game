package main

// // JoinTransportRequest는 이송 참가 요청 구조체입니다.
// type JoinTransportRequest struct {
// 	PlayerID   string `json:"player_id"`
// 	PlayerName string `json:"player_name"`
// 	GoldAmount int    `json:"gold_amount"`
// }

// // JoinTransportHandler는 이송 참가 요청을 처리합니다.
// func (s *TransportService) JoinTransportHandler(w http.ResponseWriter, r *http.Request) {
// 	vars := mux.Vars(r)
// 	transportID := vars["id"]

// 	var req JoinTransportRequest
// 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 		http.Error(w, err.Error(), http.StatusBadRequest)
// 		return
// 	}

// 	// 비즈니스 규칙 검증
// 	if req.GoldAmount <= 0 {
// 		http.Error(w, "gold amount must be positive", http.StatusBadRequest)
// 		return
// 	}

// 	// 이송 정보 조회
// 	var transport application.TransportReadModel
// 	if err := s.queryStore.FindOne(r.Context(), bson.M{"_id": transportID}).Decode(&transport); err != nil {
// 		if err == mongo.ErrNoDocuments {
// 			http.Error(w, "transport not found", http.StatusNotFound)
// 		} else {
// 			http.Error(w, err.Error(), http.StatusInternalServerError)
// 		}
// 		return
// 	}

// 	// 이송 상태 확인
// 	if transport.Status != "PREPARING" {
// 		http.Error(w, "transport is not in preparation phase", http.StatusBadRequest)
// 		return
// 	}

// 	// 참가자 수 확인
// 	if len(transport.Participants) >= transport.MaxParticipants {
// 		http.Error(w, "transport is full", http.StatusBadRequest)
// 		return
// 	}

// 	// 이미 참가 중인지 확인
// 	for _, p := range transport.Participants {
// 		if p.PlayerID == req.PlayerID {
// 			http.Error(w, "player is already participating in this transport", http.StatusBadRequest)
// 			return
// 		}
// 	}

// 	// 티켓 사용 (실제 구현에서는 티켓 서비스 호출)
// 	if err := s.useTicket(r.Context(), req.PlayerID); err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	// 커맨드 생성
// 	cmd := domain.NewAddParticipantCommand(
// 		transportID,
// 		req.PlayerID,
// 		req.PlayerName,
// 		req.GoldAmount,
// 		time.Now(),
// 	)

// 	// 커맨드 전송
// 	if err := s.commandBus.Dispatch(r.Context(), cmd); err != nil {
// 		// 실패 시 티켓 환불 (실제 구현에서는 보상 트랜잭션)
// 		if refundErr := s.refundTicket(r.Context(), req.PlayerID); refundErr != nil {
// 			log.Printf("Failed to refund ticket: %v", refundErr)
// 		}
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	// 참가 후 이송 정보 다시 조회
// 	if err := s.queryStore.FindOne(r.Context(), bson.M{"_id": transportID}).Decode(&transport); err != nil {
// 		// 이미 커맨드는 성공했으므로 에러만 로깅
// 		log.Printf("Failed to fetch updated transport: %v", err)
// 	} else {
// 		// 참가자 수가 최대에 도달했는지 확인
// 		if len(transport.Participants) >= transport.MaxParticipants {
// 			// 이송 시작 커맨드 생성
// 			now := time.Now()
// 			estimatedArrivalTime := now.Add(time.Duration(transport.TransportTime) * time.Minute)
// 			startCmd := domain.NewStartTransportCommand(
// 				transportID,
// 				now,
// 				estimatedArrivalTime,
// 			)

// 			// 커맨드 전송 (비동기로 처리)
// 			go func() {
// 				ctx := context.Background()
// 				if err := s.commandBus.Dispatch(ctx, startCmd); err != nil {
// 					log.Printf("Failed to start transport automatically: %v", err)
// 				} else {
// 					log.Printf("Transport %s started automatically after reaching max participants", transportID)
// 				}
// 			}()
// 		}
// 	}

// 	// 응답 반환
// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(map[string]string{
// 		"status": "joined",
// 		"id":     transportID,
// 	})
// }

// // useTicket은 플레이어의 티켓을 사용합니다.
// func (s *TransportService) useTicket(ctx context.Context, playerID string) error {
// 	// 실제 구현에서는 티켓 서비스 호출
// 	// 여기서는 간단히 성공으로 처리
// 	return nil
// }

// // refundTicket은 플레이어에게 티켓을 환불합니다.
// func (s *TransportService) refundTicket(ctx context.Context, playerID string) error {
// 	// 실제 구현에서는 티켓 서비스 호출
// 	// 여기서는 간단히 성공으로 처리
// 	return nil
// }

// // EnhancedJoinTransportHandler는 비즈니스 규칙이 적용된 이송 참가 핸들러입니다.
// func (s *EnhancedTransportService) JoinTransportHandler(w http.ResponseWriter, r *http.Request) {
// 	vars := mux.Vars(r)
// 	transportID := vars["id"]

// 	var req JoinTransportRequest
// 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 		http.Error(w, err.Error(), http.StatusBadRequest)
// 		return
// 	}

// 	// 비즈니스 규칙 검증
// 	if err := s.validateJoinTransport(r.Context(), transportID, req.PlayerID, req.GoldAmount); err != nil {
// 		// 에러 타입에 따른 HTTP 상태 코드 설정
// 		switch {
// 		case err == mongo.ErrNoDocuments:
// 			http.Error(w, "transport not found", http.StatusNotFound)
// 		case fmt.Sprintf("%v", err) == "transport is not in preparation phase":
// 			http.Error(w, err.Error(), http.StatusBadRequest)
// 		case fmt.Sprintf("%v", err) == "transport is full":
// 			http.Error(w, err.Error(), http.StatusBadRequest)
// 		case fmt.Sprintf("%v", err) == "player is already participating in this transport":
// 			http.Error(w, err.Error(), http.StatusBadRequest)
// 		case fmt.Sprintf("%v", err) == "player does not have available transport tickets":
// 			http.Error(w, err.Error(), http.StatusBadRequest)
// 		default:
// 			http.Error(w, err.Error(), http.StatusInternalServerError)
// 		}
// 		return
// 	}

// 	// 티켓 사용
// 	if err := s.ticketService.UseTicket(r.Context(), req.PlayerID); err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	// 이송 참가 (기존 핸들러 호출)
// 	s.TransportService.JoinTransportHandler(w, r)
// }

// // validateJoinTransport는 이송 참가 규칙을 검증합니다.
// func (s *EnhancedTransportService) validateJoinTransport(
// 	ctx context.Context,
// 	transportID, playerID string,
// 	goldAmount int,
// ) error {
// 	// 이송 정보 조회
// 	var transport application.TransportReadModel
// 	if err := s.queryStore.FindOne(ctx, bson.M{"_id": transportID}).Decode(&transport); err != nil {
// 		return err
// 	}

// 	// 이송 상태 확인
// 	if transport.Status != "PREPARING" {
// 		return fmt.Errorf("transport is not in preparation phase")
// 	}

// 	// 참가자 수 확인
// 	if len(transport.Participants) >= transport.MaxParticipants {
// 		return fmt.Errorf("transport is full")
// 	}

// 	// 이미 참가 중인지 확인
// 	for _, p := range transport.Participants {
// 		if p.PlayerID == playerID {
// 			return fmt.Errorf("player is already participating in this transport")
// 		}
// 	}

// 	// 티켓 보유 여부 확인
// 	hasTicket, err := s.ticketService.HasAvailableTicket(ctx, playerID)
// 	if err != nil {
// 		return fmt.Errorf("failed to check ticket availability: %w", err)
// 	}
// 	if !hasTicket {
// 		return fmt.Errorf("player does not have available transport tickets")
// 	}

// 	// 골드 양 검증
// 	if goldAmount <= 0 {
// 		return fmt.Errorf("gold amount must be positive")
// 	}

// 	return nil
// }
