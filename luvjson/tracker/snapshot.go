package tracker

import (
	"encoding/json"
	"fmt"
	"time"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
)

// Snapshot represents a snapshot of a CRDT document at a specific point in time.
type Snapshot struct {
	// ID is the unique identifier for this snapshot.
	ID string

	// Timestamp is when the snapshot was created.
	Timestamp time.Time

	// DocumentData is the serialized CRDT document.
	DocumentData []byte

	// Metadata is optional custom metadata.
	Metadata map[string]interface{}
}

// NewSnapshot creates a new snapshot of the given document.
func NewSnapshot(doc *crdt.Document, id string) (*Snapshot, error) {
	// Serialize the document
	docData, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize document: %w", err)
	}

	return &Snapshot{
		ID:           id,
		Timestamp:    time.Now(),
		DocumentData: docData,
		Metadata:     make(map[string]interface{}),
	}, nil
}

// RestoreDocument creates a new document from the snapshot.
func (s *Snapshot) RestoreDocument() (*crdt.Document, error) {
	// Create a new document
	zeroSID := common.SessionID{}
	doc := crdt.NewDocument(zeroSID)

	// Deserialize the document data
	if err := json.Unmarshal(s.DocumentData, doc); err != nil {
		return nil, fmt.Errorf("failed to deserialize document: %w", err)
	}

	return doc, nil
}

// SnapshotManager manages snapshots of CRDT documents.
type SnapshotManager struct {
	// snapshots is a map of snapshot IDs to snapshots.
	snapshots map[string]*Snapshot

	// timestamps is a sorted list of snapshot timestamps.
	timestamps []time.Time

	// timestampToID maps timestamps to snapshot IDs.
	timestampToID map[time.Time]string
}

// NewSnapshotManager creates a new snapshot manager.
func NewSnapshotManager() *SnapshotManager {
	return &SnapshotManager{
		snapshots:     make(map[string]*Snapshot),
		timestamps:    make([]time.Time, 0),
		timestampToID: make(map[time.Time]string),
	}
}

// CreateSnapshot creates a new snapshot of the given document.
func (m *SnapshotManager) CreateSnapshot(doc *crdt.Document, id string) (*Snapshot, error) {
	// Create a new snapshot
	snapshot, err := NewSnapshot(doc, id)
	if err != nil {
		return nil, err
	}

	// Add the snapshot to the manager
	m.snapshots[id] = snapshot
	m.timestamps = append(m.timestamps, snapshot.Timestamp)
	m.timestampToID[snapshot.Timestamp] = id

	// Sort the timestamps
	sortTimestamps(m.timestamps)

	return snapshot, nil
}

// GetSnapshot returns the snapshot with the given ID.
func (m *SnapshotManager) GetSnapshot(id string) (*Snapshot, error) {
	snapshot, ok := m.snapshots[id]
	if !ok {
		return nil, fmt.Errorf("snapshot not found: %s", id)
	}
	return snapshot, nil
}

// GetSnapshotByTime returns the snapshot closest to the given time.
func (m *SnapshotManager) GetSnapshotByTime(t time.Time) (*Snapshot, error) {
	if len(m.timestamps) == 0 {
		return nil, fmt.Errorf("no snapshots available")
	}

	// Find the closest timestamp
	closestTime := m.timestamps[0]
	closestDiff := absDuration(t.Sub(closestTime))

	for _, timestamp := range m.timestamps {
		diff := absDuration(t.Sub(timestamp))
		if diff < closestDiff {
			closestTime = timestamp
			closestDiff = diff
		}
	}

	// Get the snapshot ID for the closest timestamp
	id, ok := m.timestampToID[closestTime]
	if !ok {
		return nil, fmt.Errorf("snapshot not found for timestamp: %v", closestTime)
	}

	return m.GetSnapshot(id)
}

// absDuration returns the absolute value of a duration.
func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// ListSnapshots returns a list of all snapshots.
func (m *SnapshotManager) ListSnapshots() []*Snapshot {
	snapshots := make([]*Snapshot, 0, len(m.snapshots))
	for _, snapshot := range m.snapshots {
		snapshots = append(snapshots, snapshot)
	}
	return snapshots
}

// DeleteSnapshot deletes the snapshot with the given ID.
func (m *SnapshotManager) DeleteSnapshot(id string) error {
	snapshot, ok := m.snapshots[id]
	if !ok {
		return fmt.Errorf("snapshot not found: %s", id)
	}

	// Remove the snapshot from the manager
	delete(m.snapshots, id)
	delete(m.timestampToID, snapshot.Timestamp)

	// Remove the timestamp from the list
	for i, t := range m.timestamps {
		if t == snapshot.Timestamp {
			m.timestamps = append(m.timestamps[:i], m.timestamps[i+1:]...)
			break
		}
	}

	return nil
}

// sortTimestamps sorts the timestamps in ascending order.
func sortTimestamps(timestamps []time.Time) {
	for i := 0; i < len(timestamps); i++ {
		for j := i + 1; j < len(timestamps); j++ {
			if timestamps[j].Before(timestamps[i]) {
				timestamps[i], timestamps[j] = timestamps[j], timestamps[i]
			}
		}
	}
}

// TimeTravel creates a new document from the snapshot with the given ID.
func (m *SnapshotManager) TimeTravel(id string) (*crdt.Document, error) {
	snapshot, err := m.GetSnapshot(id)
	if err != nil {
		return nil, err
	}

	return snapshot.RestoreDocument()
}

// TimeTravelToTime creates a new document from the snapshot closest to the given time.
func (m *SnapshotManager) TimeTravelToTime(t time.Time) (*crdt.Document, error) {
	snapshot, err := m.GetSnapshotByTime(t)
	if err != nil {
		return nil, err
	}

	return snapshot.RestoreDocument()
}
