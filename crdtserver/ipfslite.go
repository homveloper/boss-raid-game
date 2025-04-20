package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	format "github.com/ipfs/go-ipld-format"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/multiformats/go-multiaddr"

	ipfslite "github.com/hsanjuan/ipfs-lite"
)

// IPFSLiteDAGSyncer는 IPFS-Lite를 사용하는 DAGSyncer 구현체입니다.
type IPFSLiteDAGSyncer struct {
	ctx    context.Context
	cancel context.CancelFunc
	host   host.Host
	dht    routing.Routing
	ipfs   *ipfslite.Peer
}

// NewIPFSLiteDAGSyncer는 새로운 IPFS-Lite 기반 DAGSyncer를 생성합니다.
func NewIPFSLiteDAGSyncer(ctx context.Context, bstore blockstore.Blockstore) (*IPFSLiteDAGSyncer, error) {
	// 컨텍스트 생성
	ipfsCtx, cancel := context.WithCancel(ctx)

	// libp2p 호스트 생성
	h, dht, err := setupLibp2p(ipfsCtx)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to setup libp2p: %w", err)
	}

	// IPFS-Lite 설정
	cfg := &ipfslite.Config{
		Offline:           false,
		ReprovideInterval: time.Hour * 12,
	}

	// IPFS-Lite 피어 생성
	ipfs, err := ipfslite.New(ipfsCtx, nil, bstore, h, dht, cfg)
	if err != nil {
		h.Close()
		cancel()
		return nil, fmt.Errorf("failed to create IPFS-Lite peer: %w", err)
	}

	// 부트스트랩 피어 연결
	ipfs.Bootstrap(ipfslite.DefaultBootstrapPeers())

	return &IPFSLiteDAGSyncer{
		ctx:    ipfsCtx,
		cancel: cancel,
		host:   h,
		dht:    dht,
		ipfs:   ipfs,
	}, nil
}

// setupLibp2p는 libp2p 호스트와 DHT를 설정합니다.
func setupLibp2p(ctx context.Context) (host.Host, routing.Routing, error) {
	// 연결 관리자 설정
	connManager, err := connmgr.NewConnManager(100, 400, connmgr.WithGracePeriod(time.Minute))
	if err != nil {
		return nil, nil, err
	}

	// 호스트 옵션 설정
	opts := []libp2p.Option{
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
		libp2p.NATPortMap(),
		libp2p.ConnectionManager(connManager),
		libp2p.EnableNATService(),
	}

	hostkey, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	// 호스트 생성
	return ipfslite.SetupLibp2p(
		ctx,
		hostkey,
		nil, // PSK (Private Shared Key)
		[]multiaddr.Multiaddr{},
		nil, // Datastore
		opts...,
	)
}

// GetLinks는 노드의 링크를 가져옵니다.
func (s *IPFSLiteDAGSyncer) GetLinks(ctx context.Context, c cid.Cid) ([]*format.Link, error) {
	node, err := s.ipfs.Get(ctx, c)
	if err != nil {
		return nil, err
	}
	return node.Links(), nil
}

// Get은 CID로 노드를 가져옵니다.
func (s *IPFSLiteDAGSyncer) Get(ctx context.Context, c cid.Cid) (format.Node, error) {
	return s.ipfs.Get(ctx, c)
}

// GetMany는 여러 CID로 노드들을 가져옵니다.
func (s *IPFSLiteDAGSyncer) GetMany(ctx context.Context, cids []cid.Cid) <-chan *format.NodeOption {
	return s.ipfs.GetMany(ctx, cids)
}

// Add는 노드를 추가합니다.
func (s *IPFSLiteDAGSyncer) Add(ctx context.Context, node format.Node) error {
	return s.ipfs.Add(ctx, node)
}

// AddMany는 여러 노드를 추가합니다.
func (s *IPFSLiteDAGSyncer) AddMany(ctx context.Context, nodes []format.Node) error {
	return s.ipfs.AddMany(ctx, nodes)
}

// Remove는 노드를 제거합니다.
func (s *IPFSLiteDAGSyncer) Remove(ctx context.Context, c cid.Cid) error {
	return s.ipfs.Remove(ctx, c)
}

// RemoveMany는 여러 노드를 제거합니다.
func (s *IPFSLiteDAGSyncer) RemoveMany(ctx context.Context, cids []cid.Cid) error {
	return s.ipfs.RemoveMany(ctx, cids)
}

// Session은 세션 기반 NodeGetter를 반환합니다.
func (s *IPFSLiteDAGSyncer) Session(ctx context.Context) format.NodeGetter {
	return s.ipfs.Session(ctx)
}

// Host는 libp2p 호스트를 반환합니다.
func (s *IPFSLiteDAGSyncer) Host() host.Host {
	return s.host
}

// Bootstrap은 지정된 피어에 연결하고 DHT를 부트스트랩합니다.
func (s *IPFSLiteDAGSyncer) Bootstrap(peers []peer.AddrInfo) {
	s.ipfs.Bootstrap(peers)
}

// Close는 리소스를 정리합니다.
func (s *IPFSLiteDAGSyncer) Close() error {
	s.host.Close()
	s.cancel()
	return nil
}
