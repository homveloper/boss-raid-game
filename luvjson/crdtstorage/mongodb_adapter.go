package crdtstorage

import (
	"context"
	"fmt"
	"sync"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoDBAdapter는 MongoDB 기반 영구 저장소 어댑터입니다.
type MongoDBAdapter struct {
	// collection은 MongoDB 컬렉션입니다.
	collection *mongo.Collection

	// mutex는 MongoDB 작업에 대한 동시 접근을 보호합니다.
	mutex sync.RWMutex

	// serializer는 문서 직렬화/역직렬화를 담당합니다.
	serializer DocumentSerializer
}

// NewMongoDBAdapter는 새 MongoDB 어댑터를 생성합니다.
func NewMongoDBAdapter(collection *mongo.Collection) *MongoDBAdapter {
	return &MongoDBAdapter{
		collection: collection,
		serializer: NewDefaultDocumentSerializer(),
	}
}

// SaveDocument는 문서를 MongoDB에 저장합니다.
func (a *MongoDBAdapter) SaveDocument(ctx context.Context, doc *Document) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// 문서 내용 가져오기
	content, err := doc.CRDTDoc.View()
	if err != nil {
		return fmt.Errorf("failed to get document view: %w", err)
	}

	// MongoDB 문서 생성
	mongoDoc := bson.M{
		"_id":          doc.ID,
		"content":      content,
		"lastModified": doc.LastModified,
		"metadata":     doc.Metadata,
		"version":      doc.Version,
	}

	// MongoDB에 저장
	opts := options.Replace().SetUpsert(true)
	_, err = a.collection.ReplaceOne(ctx, bson.M{"_id": doc.ID}, mongoDoc, opts)
	if err != nil {
		return fmt.Errorf("failed to save document: %w", err)
	}

	return nil
}

// LoadDocument는 문서를 MongoDB에서 로드합니다.
func (a *MongoDBAdapter) LoadDocument(ctx context.Context, documentID string) ([]byte, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	// MongoDB에서 문서 가져오기
	var result bson.M
	err := a.collection.FindOne(ctx, bson.M{"_id": documentID}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("document not found: %s", documentID)
		}
		return nil, fmt.Errorf("failed to load document: %w", err)
	}

	// MongoDB 문서를 바이트 배열로 변환
	// 이 부분은 실제 구현에서는 더 복잡할 수 있음
	// 여기서는 간단히 BSON을 JSON으로 변환
	data, err := bson.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal document: %w", err)
	}

	return data, nil
}

// ListDocuments는 모든 문서 목록을 반환합니다.
func (a *MongoDBAdapter) ListDocuments(ctx context.Context) ([]string, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	// MongoDB에서 모든 문서 ID 가져오기
	cursor, err := a.collection.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"_id": 1}))
	if err != nil {
		return nil, fmt.Errorf("failed to find documents: %w", err)
	}
	defer cursor.Close(ctx)

	// 문서 ID 목록 생성
	var ids []string
	for cursor.Next(ctx) {
		var result bson.M
		if err := cursor.Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode document: %w", err)
		}
		if id, ok := result["_id"].(string); ok {
			ids = append(ids, id)
		}
	}

	// 오류 확인
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return ids, nil
}

// DeleteDocument는 문서를 MongoDB에서 삭제합니다.
func (a *MongoDBAdapter) DeleteDocument(ctx context.Context, documentID string) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// MongoDB에서 문서 삭제
	_, err := a.collection.DeleteOne(ctx, bson.M{"_id": documentID})
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	return nil
}

// Close는 MongoDB 어댑터를 닫습니다.
func (a *MongoDBAdapter) Close() error {
	// MongoDB 컬렉션은 외부에서 관리하므로 여기서 닫지 않음
	return nil
}
