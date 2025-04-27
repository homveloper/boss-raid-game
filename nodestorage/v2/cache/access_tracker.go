package cache

import (
	"container/heap"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AccessRecord represents a record of document access
type AccessRecord struct {
	ID            primitive.ObjectID
	AccessCount   int64
	LastAccessed  time.Time
	FirstAccessed time.Time
	Score         float64 // Combined score based on recency and frequency
}

// AccessTracker tracks document access patterns to identify hot data
type AccessTracker struct {
	records     map[primitive.ObjectID]*AccessRecord
	hotItems    *AccessHeap
	mu          sync.RWMutex
	maxHotItems int
	decayFactor float64 // Factor to decay old access counts (0-1)
}

// NewAccessTracker creates a new access tracker
func NewAccessTracker(maxHotItems int, decayFactor float64) *AccessTracker {
	h := &AccessHeap{}
	heap.Init(h)

	return &AccessTracker{
		records:     make(map[primitive.ObjectID]*AccessRecord),
		hotItems:    h,
		maxHotItems: maxHotItems,
		decayFactor: decayFactor,
	}
}

// RecordAccess records an access to a document
func (t *AccessTracker) RecordAccess(id primitive.ObjectID) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()

	record, exists := t.records[id]
	if !exists {
		// Create new record
		record = &AccessRecord{
			ID:            id,
			AccessCount:   0,
			FirstAccessed: now,
			LastAccessed:  now,
		}
		t.records[id] = record
	}

	// Update access count and time
	record.AccessCount++
	record.LastAccessed = now

	// Calculate score: combination of recency and frequency
	// This formula gives more weight to recent accesses while still considering frequency
	timeSinceFirstAccess := now.Sub(record.FirstAccessed).Seconds()
	if timeSinceFirstAccess < 1 {
		timeSinceFirstAccess = 1 // Avoid division by zero
	}

	// Score formula: (access count / time since first access) * recency factor
	recencyFactor := 1.0 / (1.0 + now.Sub(record.LastAccessed).Seconds()/3600) // Decay over hours
	record.Score = (float64(record.AccessCount) / timeSinceFirstAccess) * recencyFactor

	// Update heap
	t.updateHeap(record)
}

// GetHotItems returns the current hot items
func (t *AccessTracker) GetHotItems() []primitive.ObjectID {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]primitive.ObjectID, 0, t.hotItems.Len())

	// Create a copy of the heap to avoid modifying it
	heapCopy := make(AccessHeap, t.hotItems.Len())
	copy(heapCopy, *t.hotItems)

	// Extract items in order
	for heapCopy.Len() > 0 {
		item := heap.Pop(&heapCopy).(*AccessRecord)
		result = append(result, item.ID)
	}

	return result
}

// IsHotItem checks if an item is in the hot set
func (t *AccessTracker) IsHotItem(id primitive.ObjectID) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	_, exists := t.records[id]
	if !exists {
		return false
	}

	// Check if this record is in the hot items heap
	for _, item := range *t.hotItems {
		if item.ID == id {
			return true
		}
	}

	return false
}

// DecayScores periodically decays scores to give preference to more recent access patterns
func (t *AccessTracker) DecayScores() {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, record := range t.records {
		// Apply decay factor to score
		record.Score *= t.decayFactor

		// Update heap
		t.updateHeapNoLock(record)
	}

	// Remove records with very low scores
	var toRemove []primitive.ObjectID
	for id, record := range t.records {
		if record.Score < 0.01 {
			toRemove = append(toRemove, id)
		}
	}

	for _, id := range toRemove {
		delete(t.records, id)
	}

	// Rebuild heap
	t.rebuildHeapNoLock()
}

// updateHeap updates the heap with the given record
func (t *AccessTracker) updateHeap(record *AccessRecord) {
	// Check if record is already in heap
	for i, item := range *t.hotItems {
		if item.ID == record.ID {
			// Update score and fix heap
			(*t.hotItems)[i] = record
			heap.Fix(t.hotItems, i)
			return
		}
	}

	// If heap is not full, add the record
	if t.hotItems.Len() < t.maxHotItems {
		heap.Push(t.hotItems, record)
		return
	}

	// If heap is full, check if this record should replace the lowest score
	if t.hotItems.Len() > 0 {
		lowestScore := (*t.hotItems)[0]
		if record.Score > lowestScore.Score {
			// Replace lowest score with this record
			heap.Pop(t.hotItems)
			heap.Push(t.hotItems, record)
		}
	}
}

// updateHeapNoLock updates the heap without locking (for internal use)
func (t *AccessTracker) updateHeapNoLock(record *AccessRecord) {
	// Same as updateHeap but without locking
	for i, item := range *t.hotItems {
		if item.ID == record.ID {
			(*t.hotItems)[i] = record
			heap.Fix(t.hotItems, i)
			return
		}
	}

	if t.hotItems.Len() < t.maxHotItems {
		heap.Push(t.hotItems, record)
		return
	}

	if t.hotItems.Len() > 0 {
		lowestScore := (*t.hotItems)[0]
		if record.Score > lowestScore.Score {
			heap.Pop(t.hotItems)
			heap.Push(t.hotItems, record)
		}
	}
}

// rebuildHeapNoLock rebuilds the heap from scratch (for internal use)
func (t *AccessTracker) rebuildHeapNoLock() {
	// Create a new heap
	newHeap := &AccessHeap{}
	heap.Init(newHeap)

	// Add all records to a slice
	var allRecords []*AccessRecord
	for _, record := range t.records {
		allRecords = append(allRecords, record)
	}

	// Sort by score (descending)
	for i := 0; i < len(allRecords) && i < t.maxHotItems; i++ {
		heap.Push(newHeap, allRecords[i])
	}

	// Replace the old heap
	t.hotItems = newHeap
}

// AccessHeap is a min-heap of AccessRecords ordered by score
type AccessHeap []*AccessRecord

func (h AccessHeap) Len() int           { return len(h) }
func (h AccessHeap) Less(i, j int) bool { return h[i].Score < h[j].Score } // Min-heap by score
func (h AccessHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *AccessHeap) Push(x interface{}) {
	*h = append(*h, x.(*AccessRecord))
}

func (h *AccessHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
