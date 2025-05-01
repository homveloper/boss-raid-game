package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"nodestorage/v2"
	"os"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"nodestorage/v2/cache"
	"tictactoe/transport"
)

func main() {
	// 1. Flag 파싱
	mongoURI := flag.String("mongo-uri", "mongodb://localhost:27017", "MongoDB connection URI")
	dbName := flag.String("db-name", "goldmine_db", "Database name")
	envFile := flag.String("env", ".env", "Path to .env file")
	flag.Parse()

	// .env 파일 로드 (있는 경우)
	if _, err := os.Stat(*envFile); err == nil {
		if err := godotenv.Load(*envFile); err != nil {
			log.Printf("Warning: Error loading .env file: %v", err)
		}
	}

	// 환경 변수로 설정 덮어쓰기
	if uri := os.Getenv("MONGO_URI"); uri != "" {
		*mongoURI = uri
	}
	if name := os.Getenv("DB_NAME"); name != "" {
		*dbName = name
	}

	// 2. 컨텍스트 생성 (취소 가능)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 3. MongoDB 연결
	log.Printf("Connecting to MongoDB at %s...", *mongoURI)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(*mongoURI))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)

	// 연결 확인
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("Failed to ping MongoDB: %v", err)
	}
	log.Printf("Connected to MongoDB successfully")

	// 4. 컬렉션 생성
	mineCollection := client.Database(*dbName).Collection("mines")
	mineConfigCollection := client.Database(*dbName).Collection("mine_configs")
	generalCollection := client.Database(*dbName).Collection("generals")
	ticketCollection := client.Database(*dbName).Collection("tickets")

	// 5. 캐시 생성
	mineCache := cache.NewMemoryCache[*transport.Mine](nil)
	mineConfigCache := cache.NewMemoryCache[*transport.MineConfig](nil)
	generalCache := cache.NewMemoryCache[*transport.General](nil)
	ticketCache := cache.NewMemoryCache[*transport.TransportTicket](nil)

	// 6. 스토리지 옵션 생성
	storageOptions := &nodestorage.Options{
		VersionField: "VectorClock", // 구조체 필드 이름과 일치해야 함
		CacheTTL:     time.Hour,
	}

	// 7. 스토리지 생성
	mineStorage, err := nodestorage.NewStorage[*transport.Mine](ctx, client, mineCollection, mineCache, storageOptions)
	if err != nil {
		log.Fatalf("Failed to create mine storage: %v", err)
	}
	defer mineStorage.Close()

	mineConfigStorage, err := nodestorage.NewStorage[*transport.MineConfig](ctx, client, mineConfigCollection, mineConfigCache, storageOptions)
	if err != nil {
		log.Fatalf("Failed to create mine config storage: %v", err)
	}
	defer mineConfigStorage.Close()

	generalStorage, err := nodestorage.NewStorage[*transport.General](ctx, client, generalCollection, generalCache, storageOptions)
	if err != nil {
		log.Fatalf("Failed to create general storage: %v", err)
	}
	defer generalStorage.Close()

	ticketStorage, err := nodestorage.NewStorage[*transport.TransportTicket](ctx, client, ticketCollection, ticketCache, storageOptions)
	if err != nil {
		log.Fatalf("Failed to create ticket storage: %v", err)
	}
	defer ticketStorage.Close()

	// 8. 서비스 생성
	ticketService := transport.NewTicketService(ticketStorage)
	generalService := transport.NewGeneralService(generalStorage)
	mineService := transport.NewMineService(mineStorage, mineConfigStorage, generalService, ticketService)

	// 9. 메인 메뉴 실행
	fmt.Println("\n=== 광산 개발 시뮬레이션 ===")
	fmt.Println("광산 개발 시뮬레이션이 시작되었습니다.")

	// 시뮬레이션 실행
	runGoldMineSimulation(ctx, mineService, generalService, ticketService)

	log.Printf("광산 개발 시뮬레이션이 종료되었습니다.")
}

// runGoldMineSimulation 함수는 광산 개발 시뮬레이션을 실행합니다.
func runGoldMineSimulation(ctx context.Context, mineService *transport.MineService, generalService *transport.GeneralService, ticketService *transport.TicketService) {
	// 1. 연합 생성
	allianceID := primitive.NewObjectID()
	fmt.Printf("연합이 생성되었습니다. (ID: %s)\n", allianceID.Hex())

	// 2. 광산 설정 생성
	createMineConfigs(ctx, mineService)

	// 3. 플레이어 생성
	player1ID := primitive.NewObjectID()
	player1Name := "플레이어1"
	player2ID := primitive.NewObjectID()
	player2Name := "플레이어2"
	player3ID := primitive.NewObjectID()
	player3Name := "플레이어3"

	fmt.Printf("플레이어가 생성되었습니다: %s, %s, %s\n", player1Name, player2Name, player3Name)

	// 4. 이송권 생성
	createTransportTickets(ctx, ticketService, allianceID, player1ID, player2ID, player3ID, player1Name, player2Name, player3Name)

	// 5. 장수 생성
	generals := createGenerals(ctx, generalService, player1ID, player2ID, player3ID, player1Name, player2Name, player3Name)

	// 6. 광산 생성 (개발 전 상태)
	mine := createMine(ctx, mineService, allianceID, "금광산 알파", transport.MineLevel2)

	// 7. 광산에 장수 배치
	assignGeneralsToMine(ctx, mineService, mine, generals, player1Name, player2Name, player3Name)

	// 8. 개발 진행도 시뮬레이션
	simulateDevelopment(ctx, mineService, mine, generalService)

	// 9. 광산 활성화
	activateMine(ctx, mineService, mine)

	// 10. 채광 시도
	mineGold(ctx, mineService, mine)

	// 11. 이송권 확인
	checkTransportTickets(ctx, ticketService, player1ID, player2ID, player3ID, player1Name, player2Name, player3Name)
}

// createMineConfigs 함수는 광산 설정을 생성합니다.
func createMineConfigs(ctx context.Context, mineService *transport.MineService) {
	fmt.Println("\n=== 광산 설정 생성 ===")

	for level := transport.MineLevel(1); level <= 3; level++ {
		minTransport := 100 * int(level)
		maxTransport := 500 * int(level)
		transportTime := 30 + (10 * int(level)) // 40-60분
		maxParticipants := 3 + int(level)       // 4-6명

		var requiredPoints float64
		var transportTicketMax int

		switch level {
		case transport.MineLevel1:
			requiredPoints = 10000 // 약 1일
			transportTicketMax = 5
		case transport.MineLevel2:
			requiredPoints = 30000 // 약 3일
			transportTicketMax = 10
		case transport.MineLevel3:
			requiredPoints = 70000 // 약 7일
			transportTicketMax = 15
		}

		config, err := mineService.CreateOrUpdateMineConfigWithDevelopment(
			ctx, level, minTransport, maxTransport, transportTime, maxParticipants,
			requiredPoints, transportTicketMax)
		if err != nil {
			log.Fatalf("광산 설정 생성 실패: %v", err)
		}

		fmt.Printf("레벨 %d 광산 설정 생성: 필요 개발 포인트=%.0f, 이송권 최대=%d\n",
			config.Level, config.RequiredPoints, config.TransportTicketMax)
	}
}

// createTransportTickets 함수는 이송권을 생성합니다.
func createTransportTickets(ctx context.Context, ticketService *transport.TicketService,
	allianceID, player1ID, player2ID, player3ID primitive.ObjectID,
	player1Name, player2Name, player3Name string) {

	fmt.Println("\n=== 이송권 생성 ===")

	ticket1, err := ticketService.GetOrCreateTickets(ctx, player1ID, allianceID, 5)
	if err != nil {
		log.Fatalf("이송권 생성 실패: %v", err)
	}
	fmt.Printf("%s의 이송권: %d/%d\n", player1Name, ticket1.CurrentTickets, ticket1.MaxTickets)

	ticket2, err := ticketService.GetOrCreateTickets(ctx, player2ID, allianceID, 5)
	if err != nil {
		log.Fatalf("이송권 생성 실패: %v", err)
	}
	fmt.Printf("%s의 이송권: %d/%d\n", player2Name, ticket2.CurrentTickets, ticket2.MaxTickets)

	ticket3, err := ticketService.GetOrCreateTickets(ctx, player3ID, allianceID, 5)
	if err != nil {
		log.Fatalf("이송권 생성 실패: %v", err)
	}
	fmt.Printf("%s의 이송권: %d/%d\n", player3Name, ticket3.CurrentTickets, ticket3.MaxTickets)
}

// createGenerals 함수는 장수를 생성합니다.
func createGenerals(ctx context.Context, generalService *transport.GeneralService,
	player1ID, player2ID, player3ID primitive.ObjectID,
	player1Name, player2Name, player3Name string) []*transport.General {

	fmt.Println("\n=== 장수 생성 ===")

	general1, err := generalService.CreateGeneral(ctx, player1ID, "장수 장", 50, 10, transport.GeneralRarityLegendary)
	if err != nil {
		log.Fatalf("장수 생성 실패: %v", err)
	}
	fmt.Printf("%s의 장수 생성: %s (레벨: %d, 성급: %d, 희귀도: %s)\n",
		player1Name, general1.Name, general1.Level, general1.Stars, general1.Rarity)

	general2, err := generalService.CreateGeneral(ctx, player2ID, "장수 이", 30, 5, transport.GeneralRarityEpic)
	if err != nil {
		log.Fatalf("장수 생성 실패: %v", err)
	}
	fmt.Printf("%s의 장수 생성: %s (레벨: %d, 성급: %d, 희귀도: %s)\n",
		player2Name, general2.Name, general2.Level, general2.Stars, general2.Rarity)

	general3, err := generalService.CreateGeneral(ctx, player3ID, "장수 왕", 20, 3, transport.GeneralRarityRare)
	if err != nil {
		log.Fatalf("장수 생성 실패: %v", err)
	}
	fmt.Printf("%s의 장수 생성: %s (레벨: %d, 성급: %d, 희귀도: %s)\n",
		player3Name, general3.Name, general3.Level, general3.Stars, general3.Rarity)

	return []*transport.General{general1, general2, general3}
}

// createMine 함수는 광산을 생성합니다.
func createMine(ctx context.Context, mineService *transport.MineService, allianceID primitive.ObjectID, name string, level transport.MineLevel) *transport.Mine {
	fmt.Println("\n=== 광산 생성 ===")

	mine, err := mineService.CreateMine(ctx, allianceID, name, level)
	if err != nil {
		log.Fatalf("광산 생성 실패: %v", err)
	}

	fmt.Printf("광산 생성: %s (ID: %s)\n", mine.Name, mine.ID.Hex())
	fmt.Printf("광산 상태: %s\n", mine.Status)
	fmt.Printf("필요 개발 포인트: %.0f\n", mine.RequiredPoints)

	return mine
}

// assignGeneralsToMine 함수는 광산에 장수를 배치합니다.
func assignGeneralsToMine(ctx context.Context, mineService *transport.MineService,
	mine *transport.Mine, generals []*transport.General,
	player1Name, player2Name, player3Name string) {

	fmt.Println("\n=== 장수 배치 ===")

	// 첫 번째 장수 배치
	mine, err := mineService.AssignGeneralToMine(ctx, mine.ID, generals[0].PlayerID, player1Name, generals[0].ID)
	if err != nil {
		log.Fatalf("장수 배치 실패: %v", err)
	}
	fmt.Printf("%s의 장수 %s를 광산 %s에 배치했습니다.\n", player1Name, generals[0].Name, mine.Name)

	// 두 번째 장수 배치
	mine, err = mineService.AssignGeneralToMine(ctx, mine.ID, generals[1].PlayerID, player2Name, generals[1].ID)
	if err != nil {
		log.Fatalf("장수 배치 실패: %v", err)
	}
	fmt.Printf("%s의 장수 %s를 광산 %s에 배치했습니다.\n", player2Name, generals[1].Name, mine.Name)

	// 세 번째 장수 배치
	mine, err = mineService.AssignGeneralToMine(ctx, mine.ID, generals[2].PlayerID, player3Name, generals[2].ID)
	if err != nil {
		log.Fatalf("장수 배치 실패: %v", err)
	}
	fmt.Printf("%s의 장수 %s를 광산 %s에 배치했습니다.\n", player3Name, generals[2].Name, mine.Name)

	fmt.Printf("광산 %s 상태: %s, 배치된 장수: %d명\n", mine.Name, mine.Status, len(mine.AssignedGenerals))

	// 각 장수의 시간당 기여도 출력
	fmt.Println("\n=== 장수별 시간당 기여도 ===")
	for _, ag := range mine.AssignedGenerals {
		fmt.Printf("%s의 장수 %s: 시간당 %.4f 포인트\n", ag.PlayerName, ag.GeneralName, ag.ContributionRate)
	}

	// 총 시간당 기여도 계산
	var totalContributionRate float64
	for _, ag := range mine.AssignedGenerals {
		totalContributionRate += ag.ContributionRate
	}
	fmt.Printf("총 시간당 기여도: %.4f 포인트\n", totalContributionRate)

	// 예상 완료 시간 계산
	estimatedHours := mine.RequiredPoints / totalContributionRate
	estimatedDays := estimatedHours / 24
	fmt.Printf("예상 완료 시간: %.1f 시간 (약 %.1f 일)\n", estimatedHours, estimatedDays)
}

// simulateDevelopment 함수는 광산 개발 진행도를 지연 계산 방식으로 시뮬레이션합니다.
func simulateDevelopment(ctx context.Context, mineService *transport.MineService, mine *transport.Mine, generalService *transport.GeneralService) {
	fmt.Println("\n=== 지연 계산 방식의 개발 진행도 시뮬레이션 ===")

	// 초기 상태 확인
	mine, err := mineService.GetMine(ctx, mine.ID)
	if err != nil {
		log.Fatalf("광산 조회 실패: %v", err)
	}
	fmt.Printf("초기 개발 진행도: %.2f/%.0f (%.1f%%)\n",
		mine.DevelopmentPoints, mine.RequiredPoints,
		(mine.DevelopmentPoints/mine.RequiredPoints)*100)

	// 시뮬레이션 단계 (10% 단위로 진행)
	simulationSteps := 10
	totalHours := mine.RequiredPoints / getTotalContributionRate(mine)
	hoursPerStep := totalHours / float64(simulationSteps)

	// 마지막 업데이트 시간 저장 (시뮬레이션용)
	var simulatedTime time.Time = time.Now()

	// 장수 레벨/성급 업그레이드 시점 (30% 및 70% 진행 시)
	upgradePoints := []float64{0.3, 0.7}
	upgradeDone := make([]bool, len(upgradePoints))

	fmt.Println("\n=== 클라이언트 폴링 시뮬레이션 ===")
	fmt.Println("클라이언트가 주기적으로 서버에 광산 상태를 요청하는 상황을 시뮬레이션합니다.")

	// 개발 완료될 때까지 반복
	for i := 1; i <= simulationSteps*2; i++ {
		// 시간 경과 시뮬레이션 (클라이언트 폴링 간격)
		elapsedHours := hoursPerStep / 2 // 폴링 간격은 시뮬레이션 단계의 절반
		simulatedTime = simulatedTime.Add(time.Duration(elapsedHours * float64(time.Hour)))

		// 현재 진행률 계산
		currentProgress := mine.DevelopmentPoints / mine.RequiredPoints

		// 장수 업그레이드 체크 및 적용
		for idx, upgradePoint := range upgradePoints {
			if !upgradeDone[idx] && currentProgress >= upgradePoint {
				upgradeDone[idx] = true

				fmt.Printf("\n=== 장수 능력치 향상 (진행률 %.0f%%) ===\n", upgradePoint*100)

				// 첫 번째 장수의 레벨 향상
				if idx == 0 {
					general, err := generalService.GetGeneralByID(ctx, mine.AssignedGenerals[0].GeneralID)
					if err != nil {
						log.Fatalf("장수 조회 실패: %v", err)
					}

					oldLevel := general.Level
					oldContribution := generalService.CalculateContributionRate(general)

					// 레벨 향상
					general, err = upgradeGeneralLevel(ctx, generalService, general.ID, 10)
					if err != nil {
						log.Fatalf("장수 레벨 향상 실패: %v", err)
					}

					newContribution := generalService.CalculateContributionRate(general)
					fmt.Printf("%s의 장수 %s 레벨 향상: %d → %d\n",
						mine.AssignedGenerals[0].PlayerName,
						mine.AssignedGenerals[0].GeneralName,
						oldLevel, general.Level)
					fmt.Printf("시간당 기여도 향상: %.4f → %.4f (%.1f%% 증가)\n",
						oldContribution, newContribution,
						(newContribution/oldContribution-1)*100)

					// 광산의 장수 정보 업데이트
					mine, err = updateAssignedGeneralStats(ctx, mineService, mine.ID, general)
					if err != nil {
						log.Fatalf("광산 장수 정보 업데이트 실패: %v", err)
					}
				} else {
					// 두 번째 장수의 성급 향상
					general, err := generalService.GetGeneralByID(ctx, mine.AssignedGenerals[1].GeneralID)
					if err != nil {
						log.Fatalf("장수 조회 실패: %v", err)
					}

					oldStars := general.Stars
					oldContribution := generalService.CalculateContributionRate(general)

					// 성급 향상
					general, err = upgradeGeneralStars(ctx, generalService, general.ID, 3)
					if err != nil {
						log.Fatalf("장수 성급 향상 실패: %v", err)
					}

					newContribution := generalService.CalculateContributionRate(general)
					fmt.Printf("%s의 장수 %s 성급 향상: %d → %d\n",
						mine.AssignedGenerals[1].PlayerName,
						mine.AssignedGenerals[1].GeneralName,
						oldStars, general.Stars)
					fmt.Printf("시간당 기여도 향상: %.4f → %.4f (%.1f%% 증가)\n",
						oldContribution, newContribution,
						(newContribution/oldContribution-1)*100)

					// 광산의 장수 정보 업데이트
					mine, err = updateAssignedGeneralStats(ctx, mineService, mine.ID, general)
					if err != nil {
						log.Fatalf("광산 장수 정보 업데이트 실패: %v", err)
					}
				}

				// 총 기여도 재계산 및 출력
				totalRate := getTotalContributionRate(mine)
				remainingPoints := mine.RequiredPoints - mine.DevelopmentPoints
				remainingHours := remainingPoints / totalRate

				fmt.Printf("총 시간당 기여도: %.4f 포인트\n", totalRate)
				fmt.Printf("예상 남은 시간: %.1f 시간\n", remainingHours)
			}
		}

		// 지연 계산 방식으로 개발 진행도 업데이트
		// 실제 서버에서는 클라이언트 요청 시 이 부분이 실행됨
		mine, err = calculateDevelopmentProgress(ctx, mineService, mine, simulatedTime)
		if err != nil {
			log.Fatalf("개발 진행도 계산 실패: %v", err)
		}

		// 진행 상황 출력 (클라이언트 화면에 표시되는 정보)
		fmt.Printf("클라이언트 폴링 #%d (%.1f시간 경과): 개발 진행도 %.2f/%.0f (%.1f%%)\n",
			i, elapsedHours*float64(i),
			mine.DevelopmentPoints, mine.RequiredPoints,
			(mine.DevelopmentPoints/mine.RequiredPoints)*100)

		// 개발이 완료되면 종료
		if mine.Status == transport.MineStatusDeveloped {
			fmt.Printf("\n광산 개발이 완료되었습니다! 상태: %s\n", mine.Status)
			fmt.Println("클라이언트에 푸시 알림: '광산 개발이 완료되었습니다!'")
			break
		}
	}
}

// calculateDevelopmentProgress 함수는 지연 계산 방식으로 개발 진행도를 계산합니다.
// 실제 서버에서는 클라이언트 요청 시 이 함수가 호출됩니다.
func calculateDevelopmentProgress(ctx context.Context, mineService *transport.MineService, mine *transport.Mine, currentTime time.Time) (*transport.Mine, error) {
	// 마지막 업데이트 시간부터 현재까지 경과 시간 계산
	hoursSinceLastUpdate := currentTime.Sub(mine.LastUpdatedAt).Hours()

	if hoursSinceLastUpdate <= 0 {
		return mine, nil
	}

	// 각 장수의 기여도 계산
	var totalPointsAdded float64 = 0
	for _, ag := range mine.AssignedGenerals {
		pointsFromGeneral := ag.ContributionRate * hoursSinceLastUpdate
		totalPointsAdded += pointsFromGeneral
	}

	// 광산 업데이트
	updatedMine, err := mineService.UpdateMineWithFunction(ctx, mine.ID, func(m *transport.Mine) (*transport.Mine, error) {
		m.DevelopmentPoints += totalPointsAdded
		m.LastUpdatedAt = currentTime

		// 개발 완료 체크
		if m.DevelopmentPoints >= m.RequiredPoints {
			m.DevelopmentPoints = m.RequiredPoints
			m.Status = transport.MineStatusDeveloped

			// 장수 해제 처리는 실제 구현에서 수행
		}

		return m, nil
	})

	return updatedMine, err
}

// upgradeGeneralLevel 함수는 장수의 레벨을 향상시킵니다.
func upgradeGeneralLevel(ctx context.Context, generalService *transport.GeneralService, generalID primitive.ObjectID, levelIncrease int) (*transport.General, error) {
	return generalService.UpdateGeneralWithFunction(ctx, generalID, func(g *transport.General) (*transport.General, error) {
		g.Level += levelIncrease
		if g.Level > 80 {
			g.Level = 80 // 최대 레벨 제한
		}
		g.UpdatedAt = time.Now()
		return g, nil
	})
}

// upgradeGeneralStars 함수는 장수의 성급을 향상시킵니다.
func upgradeGeneralStars(ctx context.Context, generalService *transport.GeneralService, generalID primitive.ObjectID, starsIncrease int) (*transport.General, error) {
	return generalService.UpdateGeneralWithFunction(ctx, generalID, func(g *transport.General) (*transport.General, error) {
		g.Stars += starsIncrease
		if g.Stars > 15 {
			g.Stars = 15 // 최대 성급 제한
		}
		g.UpdatedAt = time.Now()
		return g, nil
	})
}

// updateAssignedGeneralStats 함수는 광산에 배치된 장수의 정보를 업데이트합니다.
func updateAssignedGeneralStats(ctx context.Context, mineService *transport.MineService, mineID primitive.ObjectID, general *transport.General) (*transport.Mine, error) {
	generalService := mineService.GetGeneralService()

	return mineService.UpdateMineWithFunction(ctx, mineID, func(m *transport.Mine) (*transport.Mine, error) {
		for i, ag := range m.AssignedGenerals {
			if ag.GeneralID == general.ID {
				// 장수 정보 업데이트
				m.AssignedGenerals[i].Level = general.Level
				m.AssignedGenerals[i].Stars = general.Stars
				m.AssignedGenerals[i].Rarity = general.Rarity

				// 기여도 재계산
				contributionRate := generalService.CalculateContributionRate(general)
				m.AssignedGenerals[i].ContributionRate = contributionRate
				break
			}
		}
		return m, nil
	})
}

// activateMine 함수는 광산을 활성화합니다.
func activateMine(ctx context.Context, mineService *transport.MineService, mine *transport.Mine) {
	fmt.Println("\n=== 광산 활성화 ===")

	// 광산 활성화
	mine, err := mineService.ActivateMine(ctx, mine.ID)
	if err != nil {
		log.Fatalf("광산 활성화 실패: %v", err)
	}

	fmt.Printf("광산 %s가 활성화되었습니다. 상태: %s\n", mine.Name, mine.Status)
}

// mineGold 함수는 광산에서 금을 채굴합니다.
func mineGold(ctx context.Context, mineService *transport.MineService, mine *transport.Mine) {
	fmt.Println("\n=== 채광 시도 ===")

	// 개발 전 광산 생성 (채광 실패 테스트용)
	undevelopedMine, err := mineService.CreateMine(ctx, mine.AllianceID, "미개발 광산", transport.MineLevel1)
	if err != nil {
		log.Fatalf("미개발 광산 생성 실패: %v", err)
	}

	// 미개발 광산에 채광 시도 (실패해야 함)
	_, err = mineService.AddGoldOre(ctx, undevelopedMine.ID, 1000)
	if err != nil {
		fmt.Printf("미개발 광산 채광 시도 결과: %v (예상된 실패)\n", err)
	} else {
		fmt.Println("미개발 광산 채광 성공 (예상치 못한 결과)")
	}

	// 개발 완료된 광산에 채광 시도 (성공해야 함)
	mine, err = mineService.AddGoldOre(ctx, mine.ID, 1000)
	if err != nil {
		fmt.Printf("개발 완료된 광산 채광 시도 결과: %v (예상치 못한 실패)\n", err)
	} else {
		fmt.Printf("개발 완료된 광산 채광 성공: 현재 금광석 %d\n", mine.GoldOre)
	}
}

// checkTransportTickets 함수는 이송권 상태를 확인합니다.
func checkTransportTickets(ctx context.Context, ticketService *transport.TicketService,
	player1ID, player2ID, player3ID primitive.ObjectID,
	player1Name, player2Name, player3Name string) {

	fmt.Println("\n=== 이송권 상태 확인 ===")

	ticket1, err := ticketService.GetOrCreateTickets(ctx, player1ID, primitive.NilObjectID, 5)
	if err != nil {
		log.Fatalf("이송권 조회 실패: %v", err)
	}
	fmt.Printf("%s의 이송권: %d/%d\n", player1Name, ticket1.CurrentTickets, ticket1.MaxTickets)

	ticket2, err := ticketService.GetOrCreateTickets(ctx, player2ID, primitive.NilObjectID, 5)
	if err != nil {
		log.Fatalf("이송권 조회 실패: %v", err)
	}
	fmt.Printf("%s의 이송권: %d/%d\n", player2Name, ticket2.CurrentTickets, ticket2.MaxTickets)

	ticket3, err := ticketService.GetOrCreateTickets(ctx, player3ID, primitive.NilObjectID, 5)
	if err != nil {
		log.Fatalf("이송권 조회 실패: %v", err)
	}
	fmt.Printf("%s의 이송권: %d/%d\n", player3Name, ticket3.CurrentTickets, ticket3.MaxTickets)
}

// getTotalContributionRate 함수는 광산에 배치된 모든 장수의 총 기여도를 계산합니다.
func getTotalContributionRate(mine *transport.Mine) float64 {
	var totalRate float64
	for _, ag := range mine.AssignedGenerals {
		totalRate += ag.ContributionRate
	}
	return totalRate
}
