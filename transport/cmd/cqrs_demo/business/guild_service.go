package business

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// GuildService는 길드 관련 서비스를 제공합니다.
type GuildService struct {
	collection *mongo.Collection
}

// NewGuildService는 새로운 GuildService를 생성합니다.
func NewGuildService(collection *mongo.Collection) *GuildService {
	return &GuildService{
		collection: collection,
	}
}

// IsGuildMember는 플레이어가 길드의 멤버인지 확인합니다.
func (s *GuildService) IsGuildMember(ctx context.Context, playerID, allianceID string) (bool, error) {
	// 길드 멤버십 조회
	count, err := s.collection.CountDocuments(ctx, bson.M{
		"alliance_id": allianceID,
		"members": bson.M{
			"$elemMatch": bson.M{
				"player_id": playerID,
				"status":    "ACTIVE", // 활성 상태인 멤버만 확인
			},
		},
	})
	if err != nil {
		return false, err
	}

	return count > 0, nil
}
