package main

import (
	"context"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// PubSubBroadcaster는 libp2p PubSub을 사용하는 CRDT 브로드캐스터
type PubSubBroadcaster struct {
	ctx          context.Context
	topic        *pubsub.Topic
	subscription *pubsub.Subscription
}

// NewPubSubBroadcaster는 새 PubSub 브로드캐스터를 생성
func NewPubSubBroadcaster(ctx context.Context, topic *pubsub.Topic, subscription *pubsub.Subscription) *PubSubBroadcaster {
	return &PubSubBroadcaster{
		ctx:          ctx,
		topic:        topic,
		subscription: subscription,
	}
}

// Broadcast는 데이터를 브로드캐스트
func (b *PubSubBroadcaster) Broadcast(ctx context.Context, data []byte) error {
	return b.topic.Publish(ctx, data)
}

// Next는 다음 브로드캐스트 메시지를 수신
func (b *PubSubBroadcaster) Next(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-b.ctx.Done():
		return nil, b.ctx.Err()
	default:
		msg, err := b.subscription.Next(ctx)
		if err != nil {
			return nil, err
		}

		// 자신이 보낸 메시지는 무시
		peers := b.topic.ListPeers()
		if len(peers) > 0 && msg.ReceivedFrom == peers[0] {
			return b.Next(ctx)
		}

		return msg.Data, nil
	}
}
