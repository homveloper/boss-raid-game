package crdtpubsub

import (
	"fmt"
	"sync"

	"tictactoe/luvjson/crdt"
	"tictactoe/luvjson/crdtpatch"
)

// Tracker is a helper for tracking CRDT patches and applying them to a document.
type Tracker struct {
	// doc is the CRDT document being tracked.
	doc *crdt.Document

	// appliedPatches is a map of patch IDs to patches that have been applied.
	appliedPatches map[string]bool

	// mutex is used to protect access to the appliedPatches map.
	mutex sync.RWMutex
}

// NewTracker creates a new Tracker for the given document.
func NewTracker(doc *crdt.Document) *Tracker {
	return &Tracker{
		doc:           doc,
		appliedPatches: make(map[string]bool),
		mutex:         sync.RWMutex{},
	}
}

// ApplyPatch applies a patch to the document if it hasn't been applied already.
func (t *Tracker) ApplyPatch(patch *crdtpatch.Patch) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	// Check if the patch has already been applied
	patchID := patch.ID().String()
	if t.appliedPatches[patchID] {
		// Patch has already been applied, skip it
		return nil
	}

	// Apply the patch
	if err := patch.Apply(t.doc); err != nil {
		return fmt.Errorf("failed to apply patch: %w", err)
	}

	// Mark the patch as applied
	t.appliedPatches[patchID] = true

	return nil
}

// GetDocument returns the tracked document.
func (t *Tracker) GetDocument() *crdt.Document {
	return t.doc
}

// Reset clears the applied patches map.
func (t *Tracker) Reset() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.appliedPatches = make(map[string]bool)
}

// HasAppliedPatch checks if a patch has been applied.
func (t *Tracker) HasAppliedPatch(patchID string) bool {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	return t.appliedPatches[patchID]
}

// GetAppliedPatchCount returns the number of applied patches.
func (t *Tracker) GetAppliedPatchCount() int {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	return len(t.appliedPatches)
}
