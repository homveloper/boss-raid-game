package crdt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"tictactoe/internal/domain"
	"time"

	ds "github.com/ipfs/go-datastore"
	dsquery "github.com/ipfs/go-datastore/query"
)

// RoomRepository is a CRDT-based implementation of domain.RoomRepository
type RoomRepository struct {
	crdtStore ds.Datastore
	prefix    string
	mu        sync.RWMutex
	ctx       context.Context
}

// NewRoomRepository creates a new CRDT-based room repository
func NewRoomRepository(ctx context.Context, crdtStore ds.Datastore, prefix string) *RoomRepository {
	return &RoomRepository{
		crdtStore: crdtStore,
		prefix:    prefix,
		ctx:       ctx,
	}
}

// Create creates a new room
func (r *RoomRepository) Create(room *domain.Room) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 각 작업에 대해 타임아웃이 있는 컨텍스트 생성 (10초)
	ctx, cancel := context.WithTimeout(r.ctx, 10*time.Second)
	defer cancel()

	// 디버깅 로그 추가
	fmt.Printf("Creating room: %+v\n", room)

	// Check if room already exists
	key := ds.NewKey(r.prefix + "/rooms/" + room.ID)
	fmt.Printf("Room key: %s\n", key)

	exists, err := r.crdtStore.Has(ctx, key)
	if err != nil {
		fmt.Printf("Error checking if room exists: %v\n", err)
		return fmt.Errorf("failed to check if room exists: %w", err)
	}
	if exists {
		fmt.Printf("Room already exists: %s\n", room.ID)
		return errors.New("room already exists")
	}

	// Serialize room to JSON
	data, err := json.Marshal(room)
	if err != nil {
		fmt.Printf("Error marshaling room: %v\n", err)
		return fmt.Errorf("failed to marshal room: %w", err)
	}

	// Store room in CRDT datastore
	fmt.Printf("Storing room in CRDT datastore: %s\n", key)
	err = r.crdtStore.Put(ctx, key, data)
	if err != nil {
		fmt.Printf("Error storing room: %v\n", err)
		return fmt.Errorf("failed to store room: %w", err)
	}

	fmt.Printf("Room created successfully: %s\n", room.ID)
	return nil
}

// Get retrieves a room by ID
func (r *RoomRepository) Get(id string) (*domain.Room, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Get room from CRDT datastore
	key := ds.NewKey(r.prefix + "/rooms/" + id)
	data, err := r.crdtStore.Get(r.ctx, key)
	if err != nil {
		if err == ds.ErrNotFound {
			return nil, errors.New("room not found")
		}
		return nil, fmt.Errorf("failed to get room: %w", err)
	}

	// Deserialize room from JSON
	var room domain.Room
	err = json.Unmarshal(data, &room)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal room: %w", err)
	}

	return &room, nil
}

// Update updates a room
func (r *RoomRepository) Update(room *domain.Room) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if room exists
	key := ds.NewKey(r.prefix + "/rooms/" + room.ID)
	exists, err := r.crdtStore.Has(r.ctx, key)
	if err != nil {
		return fmt.Errorf("failed to check if room exists: %w", err)
	}
	if !exists {
		return errors.New("room not found")
	}

	// Serialize room to JSON
	data, err := json.Marshal(room)
	if err != nil {
		return fmt.Errorf("failed to marshal room: %w", err)
	}

	// Store room in CRDT datastore
	err = r.crdtStore.Put(r.ctx, key, data)
	if err != nil {
		return fmt.Errorf("failed to store room: %w", err)
	}

	return nil
}

// Delete deletes a room
func (r *RoomRepository) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if room exists
	key := ds.NewKey(r.prefix + "/rooms/" + id)
	exists, err := r.crdtStore.Has(r.ctx, key)
	if err != nil {
		return fmt.Errorf("failed to check if room exists: %w", err)
	}
	if !exists {
		return errors.New("room not found")
	}

	// Delete room from CRDT datastore
	err = r.crdtStore.Delete(r.ctx, key)
	if err != nil {
		return fmt.Errorf("failed to delete room: %w", err)
	}

	return nil
}

// List returns all rooms
func (r *RoomRepository) List() ([]*domain.Room, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Query all rooms from CRDT datastore
	query := dsquery.Query{Prefix: r.prefix + "/rooms/"}
	results, err := r.crdtStore.Query(r.ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query rooms: %w", err)
	}
	defer results.Close()

	// Deserialize rooms from JSON
	var rooms []*domain.Room
	for {
		result, ok := results.NextSync()
		if !ok {
			break
		}

		var room domain.Room
		err = json.Unmarshal(result.Value, &room)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal room: %w", err)
		}

		rooms = append(rooms, &room)
	}

	return rooms, nil
}
