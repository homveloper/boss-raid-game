package crdtsync

import (
	"context"
	"encoding/json"
	"fmt"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdtpatch"
	"tictactoe/luvjson/crdtpubsub"
)

// PubSubBroadcaster는 PubSub을 사용하는 브로드캐스터 구현입니다.
type PubSubBroadcaster struct {
	// pubsub은 기본 PubSub 인스턴스입니다.
	pubsub crdtpubsub.PubSub

	// topic은 패치를 브로드캐스트하는 데 사용되는 토픽입니다.
	topic string

	// format은 패치 인코딩 형식입니다.
	format crdtpubsub.EncodingFormat

	// localSessionID는 로컬 세션 ID입니다.
	localSessionID common.SessionID
}

// NewPubSubBroadcaster는 새 PubSub 브로드캐스터를 생성합니다.
func NewPubSubBroadcaster(pubsub crdtpubsub.PubSub, topic string, format crdtpubsub.EncodingFormat, sessionID common.SessionID) *PubSubBroadcaster {
	return &PubSubBroadcaster{
		pubsub:         pubsub,
		topic:          topic,
		format:         format,
		localSessionID: sessionID,
	}
}

// Broadcast는 패치를 다른 노드들에게 브로드캐스트합니다.
func (b *PubSubBroadcaster) Broadcast(ctx context.Context, patch *crdtpatch.Patch) error {
	// 패치 브로드캐스트
	return b.pubsub.Publish(ctx, b.topic, patch, b.format)
}

// Next는 다음 브로드캐스트된 패치를 수신합니다.
func (b *PubSubBroadcaster) Next(ctx context.Context) (*crdtpatch.Patch, error) {
	// 채널 생성
	patchCh := make(chan *crdtpatch.Patch, 1)
	errCh := make(chan error, 1)

	// 구독 ID 생성
	subscriberID := fmt.Sprintf("broadcaster-%s", b.localSessionID.String())

	// 구독 함수
	subscriberFunc := func(ctx context.Context, topic string, data []byte, format crdtpubsub.EncodingFormat) error {
		// 패치 디코딩
		decoder, err := crdtpubsub.GetEncoderDecoder(format)
		if err != nil {
			errCh <- fmt.Errorf("failed to get decoder: %w", err)
			return err
		}

		patch, err := decoder.Decode(data)
		if err != nil {
			errCh <- fmt.Errorf("failed to decode patch: %w", err)
			return err
		}

		// 자신이 보낸 패치는 무시
		if patch.ID().SID.Compare(b.localSessionID) == 0 {
			return nil
		}

		// 패치 전송
		select {
		case patchCh <- patch:
		case <-ctx.Done():
			return ctx.Err()
		}

		return nil
	}

	// 구독
	if err := b.pubsub.Subscribe(ctx, b.topic, subscriberID, subscriberFunc); err != nil {
		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}
	defer b.pubsub.Unsubscribe(ctx, b.topic, subscriberID)

	// 패치 또는 에러 대기
	select {
	case patch := <-patchCh:
		return patch, nil
	case err := <-errCh:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Close는 브로드캐스터를 종료합니다.
func (b *PubSubBroadcaster) Close() error {
	return nil
}

// SyncMessage는 동기화 메시지를 나타냅니다.
type SyncMessage struct {
	// Type은 메시지 유형입니다.
	Type string `json:"type"`

	// PeerID는 메시지를 보낸 피어의 ID입니다.
	PeerID string `json:"peer_id"`

	// StateVector는 피어의 상태 벡터입니다.
	StateVector map[string]uint64 `json:"state_vector,omitempty"`

	// RequestedPatches는 요청된 패치 ID 목록입니다.
	RequestedPatches []common.LogicalTimestamp `json:"requested_patches,omitempty"`

	// Patches는 요청된 패치 목록입니다.
	Patches []*crdtpatch.Patch `json:"patches,omitempty"`
}

// PubSubSyncer는 PubSub을 사용하는 싱커 구현입니다.
type PubSubSyncer struct {
	// pubsub은 기본 PubSub 인스턴스입니다.
	pubsub crdtpubsub.PubSub

	// syncTopic은 동기화 메시지에 사용되는 토픽입니다.
	syncTopic string

	// peerID는 이 노드의 피어 ID입니다.
	peerID string

	// stateVector는 이 노드의 상태 벡터입니다.
	stateVector *StateVector

	// patchStore는 패치 저장소입니다.
	patchStore PatchStore

	// format은 메시지 인코딩 형식입니다.
	format crdtpubsub.EncodingFormat
}

// NewPubSubSyncer는 새 PubSub 싱커를 생성합니다.
func NewPubSubSyncer(pubsub crdtpubsub.PubSub, syncTopic string, peerID string, stateVector *StateVector, patchStore PatchStore, format crdtpubsub.EncodingFormat) *PubSubSyncer {
	return &PubSubSyncer{
		pubsub:       pubsub,
		syncTopic:    syncTopic,
		peerID:       peerID,
		stateVector:  stateVector,
		patchStore:   patchStore,
		format:       format,
	}
}

// Sync는 다른 노드와 상태를 동기화합니다.
func (s *PubSubSyncer) Sync(ctx context.Context, peerID string) error {
	// 상태 벡터 메시지 생성
	stateVectorMsg := SyncMessage{
		Type:        "state_vector",
		PeerID:      s.peerID,
		StateVector: s.stateVector.Get(),
	}

	// 메시지 마샬링
	msgData, err := json.Marshal(stateVectorMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal state vector message: %w", err)
	}

	// 개인 동기화 토픽 생성
	syncTopic := fmt.Sprintf("%s-%s-%s", s.syncTopic, s.peerID, peerID)

	// 상태 벡터 메시지 발행
	if err := s.pubsub.PublishRaw(ctx, syncTopic, msgData, s.format); err != nil {
		return fmt.Errorf("failed to publish state vector message: %w", err)
	}

	// 응답 대기
	// TODO: 응답 처리 구현

	return nil
}

// GetStateVector는 현재 노드의 상태 벡터를 반환합니다.
func (s *PubSubSyncer) GetStateVector() map[string]uint64 {
	return s.stateVector.Get()
}

// GetMissingPatches는 주어진 상태 벡터에 따라 누락된 패치를 반환합니다.
func (s *PubSubSyncer) GetMissingPatches(ctx context.Context, stateVector map[string]uint64) ([]*crdtpatch.Patch, error) {
	return s.patchStore.GetPatches(stateVector)
}

// Close는 싱커를 종료합니다.
func (s *PubSubSyncer) Close() error {
	return nil
}
