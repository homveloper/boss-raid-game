package main

import (
	"context"

	blockservice "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	format "github.com/ipfs/go-ipld-format"
	dag "github.com/ipfs/go-merkledag"
)

// SimpleDAGService는 간단한 DAG 서비스 구현
type SimpleDAGService struct {
	format.DAGService
}

// NewSimpleDAGService는 새 DAG 서비스를 생성
func NewSimpleDAGService(bs blockstore.Blockstore) format.DAGService {
	// 기본 DAG 서비스 생성
	dagService := dag.NewDAGService(blockservice.New(bs, nil))

	// 래퍼 생성
	return &SimpleDAGService{
		DAGService: dagService,
	}
}

// GetLinks는 노드의 링크를 가져옴
func (s *SimpleDAGService) GetLinks(ctx context.Context, c cid.Cid) ([]*format.Link, error) {
	node, err := s.Get(ctx, c)
	if err != nil {
		return nil, err
	}
	return node.Links(), nil
}

// GetMany는 여러 노드를 가져옴
func (s *SimpleDAGService) GetMany(ctx context.Context, cids []cid.Cid) <-chan *format.NodeOption {
	out := make(chan *format.NodeOption, len(cids))
	var count int

	for _, c := range cids {
		count++
		go func(c cid.Cid) {
			node, err := s.Get(ctx, c)

			select {
			case out <- &format.NodeOption{Node: node, Err: err}:
			case <-ctx.Done():
			}
		}(c)
	}

	go func() {
		defer close(out)
		<-ctx.Done()
	}()

	return out
}
