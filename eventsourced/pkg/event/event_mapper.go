package event

import (
	"strings"
	"time"

	"github.com/yourusername/eventsourced/pkg/storage"
)

// EventMapper는 문서 변경을 이벤트로 매핑하는 인터페이스입니다.
type EventMapper interface {
	// MapToEvents는 문서 변경을 이벤트로 매핑합니다.
	MapToEvents(collection string, id string, diff *storage.Diff) []Event
}

// CollectionEventTypes는 컬렉션별 이벤트 타입 정의입니다.
type CollectionEventTypes struct {
	Created string
	Updated string
	Deleted string
	// 필요에 따라 추가 이벤트 타입
}

// DefaultEventMapper는 기본 이벤트 매퍼 구현입니다.
type DefaultEventMapper struct {
	// 컬렉션별 이벤트 타입 매핑
	collectionEventTypes map[string]CollectionEventTypes
}

// NewDefaultEventMapper는 새로운 DefaultEventMapper를 생성합니다.
func NewDefaultEventMapper() *DefaultEventMapper {
	return &DefaultEventMapper{
		collectionEventTypes: map[string]CollectionEventTypes{
			"users": {
				Created: "UserCreated",
				Updated: "UserUpdated",
				Deleted: "UserDeleted",
			},
			"transports": {
				Created: "TransportCreated",
				Updated: "TransportUpdated",
				Deleted: "TransportDeleted",
			},
			// 기타 컬렉션별 이벤트 타입 정의
		},
	}
}

// RegisterCollectionEventTypes는 컬렉션별 이벤트 타입을 등록합니다.
func (m *DefaultEventMapper) RegisterCollectionEventTypes(collection string, eventTypes CollectionEventTypes) {
	m.collectionEventTypes[collection] = eventTypes
}

// MapToEvents는 문서 변경을 이벤트로 매핑합니다.
func (m *DefaultEventMapper) MapToEvents(collection string, id string, diff *storage.Diff) []Event {
	events := make([]Event, 0, 1)
	
	// 컬렉션별 이벤트 타입 조회
	eventTypes, ok := m.collectionEventTypes[collection]
	if !ok {
		// 등록되지 않은 컬렉션은 기본 이벤트 타입 사용
		eventTypes = CollectionEventTypes{
			Created: "EntityCreated",
			Updated: "EntityUpdated",
			Deleted: "EntityDeleted",
		}
	}
	
	// 이벤트 타입 결정
	var eventType string
	if diff.IsNew {
		eventType = eventTypes.Created
	} else if diff.HasChanges {
		eventType = eventTypes.Updated
	} else {
		eventType = eventTypes.Deleted
	}
	
	// 애그리게이트 타입 결정 (컬렉션 이름 기반)
	aggregateType := collection
	if len(collection) > 0 {
		// 복수형을 단수형으로 변환 (간단한 규칙)
		if strings.HasSuffix(collection, "s") {
			aggregateType = collection[:len(collection)-1]
		}
		// 첫 글자를 대문자로 변환
		if len(aggregateType) > 0 {
			aggregateType = strings.ToUpper(aggregateType[:1]) + aggregateType[1:]
		}
	}
	
	// 기본 이벤트 생성
	event := NewDefaultEvent(
		eventType,
		id,
		aggregateType,
		diff.Version,
		time.Now(),
		map[string]interface{}{
			"diff":        diff,
			"merge_patch": diff.MergePatch,
			"is_new":      diff.IsNew,
			"has_changes": diff.HasChanges,
		},
	)
	
	events = append(events, event)
	
	return events
}
