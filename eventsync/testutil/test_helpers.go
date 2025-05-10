package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TestDocument는 테스트에 사용되는 문서 구조체입니다.
type TestDocument struct {
	ID      primitive.ObjectID `bson:"_id" json:"id"`
	Name    string             `bson:"name" json:"name"`
	Value   int                `bson:"value" json:"value"`
	Tags    []string           `bson:"tags,omitempty" json:"tags,omitempty"`
	Created time.Time          `bson:"created" json:"created"`
	Updated time.Time          `bson:"updated" json:"updated"`
	Version int64              `bson:"version" json:"version"`
}

// Copy는 TestDocument의 복사본을 반환합니다.
func (d *TestDocument) Copy() *TestDocument {
	if d == nil {
		return nil
	}

	copy := &TestDocument{
		ID:      d.ID,
		Name:    d.Name,
		Value:   d.Value,
		Created: d.Created,
		Updated: d.Updated,
		Version: d.Version,
	}

	if d.Tags != nil {
		copy.Tags = make([]string, len(d.Tags))
		for i, tag := range d.Tags {
			copy.Tags[i] = tag
		}
	}

	return copy
}

// SetupTestDB는 테스트용 MongoDB 데이터베이스를 설정합니다.
// 반환값:
// - MongoDB 클라이언트
// - 데이터베이스 인스턴스
// - 정리 함수 (테스트 종료 후 호출해야 함)
func SetupTestDB(t *testing.T) (*mongo.Client, *mongo.Database, func()) {
	// 테스트용 MongoDB 연결
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	require.NoError(t, err)

	// 테스트용 데이터베이스 이름 (고유한 이름 생성)
	dbName := fmt.Sprintf("eventsync_test_%d", time.Now().UnixNano())
	db := client.Database(dbName)

	// 정리 함수 반환
	cleanup := func() {
		err := db.Drop(ctx)
		assert.NoError(t, err)
		err = client.Disconnect(ctx)
		assert.NoError(t, err)
	}

	return client, db, cleanup
}

// CreateTestDocument는 테스트용 문서를 생성합니다.
func CreateTestDocument() *TestDocument {
	return &TestDocument{
		ID:      primitive.NewObjectID(),
		Name:    "Test Document",
		Value:   100,
		Tags:    []string{"test", "sample"},
		Created: time.Now(),
		Updated: time.Now(),
		Version: 1,
	}
}

// CreateTestDocuments는 지정된 수의 테스트용 문서를 생성합니다.
func CreateTestDocuments(count int) []*TestDocument {
	docs := make([]*TestDocument, count)
	for i := 0; i < count; i++ {
		docs[i] = &TestDocument{
			ID:      primitive.NewObjectID(),
			Name:    fmt.Sprintf("Test Document %d", i+1),
			Value:   100 + i,
			Tags:    []string{"test", fmt.Sprintf("doc-%d", i+1)},
			Created: time.Now(),
			Updated: time.Now(),
			Version: 1,
		}
	}
	return docs
}

// WaitForCondition은 지정된 조건이 충족될 때까지 대기합니다.
func WaitForCondition(t *testing.T, condition func() bool, timeout time.Duration, message string) {
	start := time.Now()
	for {
		if condition() {
			return
		}

		if time.Since(start) > timeout {
			t.Fatalf("Timeout waiting for condition: %s", message)
			return
		}

		time.Sleep(50 * time.Millisecond)
	}
}
