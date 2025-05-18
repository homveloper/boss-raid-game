package storage

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Storage는 기본 저장소 인터페이스입니다.
type Storage struct {
	client       *mongo.Client
	db           *mongo.Database
	versionField string
}

// StorageOptions는 Storage 생성 옵션입니다.
type StorageOptions struct {
	VersionField string
}

// NewStorage는 새로운 Storage를 생성합니다.
func NewStorage(ctx context.Context, client *mongo.Client, dbName string, opts *StorageOptions) (*Storage, error) {
	if client == nil {
		return nil, errors.New("mongo client is required")
	}

	if dbName == "" {
		return nil, errors.New("database name is required")
	}

	versionField := "version"
	if opts != nil && opts.VersionField != "" {
		versionField = opts.VersionField
	}

	return &Storage{
		client:       client,
		db:           client.Database(dbName),
		versionField: versionField,
	}, nil
}

// Diff는 문서 변경 사항을 나타냅니다.
type Diff struct {
	ID         string      `json:"id" bson:"id"`
	Collection string      `json:"collection" bson:"collection"`
	IsNew      bool        `json:"is_new" bson:"is_new"`
	HasChanges bool        `json:"has_changes" bson:"has_changes"`
	Version    int         `json:"version" bson:"version"`
	MergePatch interface{} `json:"merge_patch" bson:"merge_patch"`
	BsonPatch  interface{} `json:"bson_patch" bson:"bson_patch"`
}

// EditFunc는 문서를 편집하는 함수 타입입니다.
type EditFunc func(doc interface{}) (interface{}, error)

// Update는 문서를 업데이트합니다.
func (s *Storage) Update(ctx context.Context, collection string, id string, editFunc EditFunc) (*Diff, error) {
	if collection == "" {
		return nil, errors.New("collection name is required")
	}

	if id == "" {
		return nil, errors.New("document id is required")
	}

	if editFunc == nil {
		return nil, errors.New("edit function is required")
	}

	// 문서 조회
	var doc interface{}
	err := s.FindOne(ctx, collection, bson.M{"_id": id}).Decode(&doc)
	isNew := false

	if err != nil {
		if err != mongo.ErrNoDocuments {
			return nil, fmt.Errorf("failed to find document: %w", err)
		}
		// 문서가 없는 경우 새 문서 생성
		isNew = true
		doc = make(map[string]interface{})
	}

	// 현재 버전 가져오기
	currentVersion := 0
	if !isNew {
		currentVersion, err = s.GetVersion(ctx, collection, id)
		if err != nil {
			return nil, fmt.Errorf("failed to get document version: %w", err)
		}
	}

	// 문서 편집
	updatedDoc, err := editFunc(doc)
	if err != nil {
		return nil, fmt.Errorf("edit function failed: %w", err)
	}

	if updatedDoc == nil {
		return nil, errors.New("edit function returned nil document")
	}

	// 버전 증가
	newVersion := currentVersion + 1
	if err := s.setVersion(updatedDoc, newVersion); err != nil {
		return nil, fmt.Errorf("failed to set version: %w", err)
	}

	// 문서 저장
	var result *mongo.UpdateResult
	if isNew {
		result, err = s.db.Collection(collection).UpdateOne(
			ctx,
			bson.M{"_id": id},
			bson.M{"$set": updatedDoc},
			options.Update().SetUpsert(true),
		)
	} else {
		result, err = s.db.Collection(collection).UpdateOne(
			ctx,
			bson.M{
				"_id":           id,
				s.versionField:  currentVersion,
			},
			bson.M{"$set": updatedDoc},
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to update document: %w", err)
	}

	if !isNew && result.MatchedCount == 0 {
		return nil, errors.New("optimistic concurrency control failed: document was modified")
	}

	// Diff 생성
	diff := &Diff{
		ID:         id,
		Collection: collection,
		IsNew:      isNew,
		HasChanges: true,
		Version:    newVersion,
		// MergePatch와 BsonPatch는 실제 구현에서 계산
	}

	return diff, nil
}

// FindOne은 단일 문서를 조회합니다.
func (s *Storage) FindOne(ctx context.Context, collection string, filter interface{}, opts ...*options.FindOneOptions) *mongo.SingleResult {
	return s.db.Collection(collection).FindOne(ctx, filter, opts...)
}

// FindOneAndUpdate는 문서를 찾아 업데이트합니다.
func (s *Storage) FindOneAndUpdate(ctx context.Context, collection string, filter interface{}, update interface{}, opts ...*options.FindOneAndUpdateOptions) *mongo.SingleResult {
	return s.db.Collection(collection).FindOneAndUpdate(ctx, filter, update, opts...)
}

// FindOneAndUpsert는 문서를 찾아 업서트합니다.
func (s *Storage) FindOneAndUpsert(ctx context.Context, collection string, filter interface{}, update interface{}, opts ...*options.FindOneAndUpdateOptions) *mongo.SingleResult {
	upsertOpts := options.FindOneAndUpdate().SetUpsert(true)
	if len(opts) > 0 {
		// 기존 옵션 복사
		for _, opt := range opts {
			if opt.Upsert != nil {
				upsertOpts.SetUpsert(*opt.Upsert)
			}
			if opt.ReturnDocument != nil {
				upsertOpts.SetReturnDocument(*opt.ReturnDocument)
			}
			// 기타 옵션 복사...
		}
	}
	return s.db.Collection(collection).FindOneAndUpdate(ctx, filter, update, upsertOpts)
}

// GetVersion은 문서의 현재 버전을 조회합니다.
func (s *Storage) GetVersion(ctx context.Context, collection string, id string) (int, error) {
	var result struct {
		Version int `bson:"version"`
	}

	err := s.db.Collection(collection).FindOne(
		ctx,
		bson.M{"_id": id},
		options.FindOne().SetProjection(bson.M{s.versionField: 1}),
	).Decode(&result)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return 0, nil
		}
		return 0, err
	}

	return result.Version, nil
}

// VersionField는 버전 필드 이름을 반환합니다.
func (s *Storage) VersionField() string {
	return s.versionField
}

// setVersion은 문서의 버전을 설정합니다.
func (s *Storage) setVersion(doc interface{}, version int) error {
	val := reflect.ValueOf(doc)

	// 포인터인 경우 역참조
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	switch val.Kind() {
	case reflect.Map:
		// map[string]interface{} 타입인 경우
		if m, ok := doc.(map[string]interface{}); ok {
			m[s.versionField] = version
			return nil
		}

		// bson.M 타입인 경우
		if m, ok := doc.(bson.M); ok {
			m[s.versionField] = version
			return nil
		}

		return fmt.Errorf("unsupported map type: %T", doc)

	case reflect.Struct:
		// 구조체인 경우 필드 찾기
		field := val.FieldByName(s.versionField)
		if !field.IsValid() {
			// 소문자로 시작하는 필드명 처리
			field = val.FieldByName("Version")
			if !field.IsValid() {
				return fmt.Errorf("version field '%s' not found in struct", s.versionField)
			}
		}

		if field.CanSet() {
			field.SetInt(int64(version))
			return nil
		}

		return fmt.Errorf("version field '%s' cannot be set", s.versionField)

	default:
		return fmt.Errorf("unsupported document type: %T", doc)
	}
}

// Close는 저장소 연결을 닫습니다.
func (s *Storage) Close(ctx context.Context) error {
	return nil
}
