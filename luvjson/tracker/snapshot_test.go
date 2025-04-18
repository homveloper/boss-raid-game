package tracker

import (
	"testing"
	"time"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
)

// TestSnapshot tests the snapshot functionality.
func TestSnapshot(t *testing.T) {
	// Create a new document
	sid := common.NewSessionID()
	doc := crdt.NewDocument(sid)

	// Create a test person
	person := &TestPerson{
		Name:    "John Doe",
		Age:     30,
		Email:   "john@example.com",
		Created: time.Now(),
		Tags:    []string{"tag1", "tag2"},
	}
	person.Address.Street = "123 Main St"
	person.Address.City = "New York"
	person.Address.Country = "USA"

	// Create a tracker and initialize the document
	tracker := NewTracker(doc, sid)
	if err := tracker.InitializeDocument(person); err != nil {
		t.Fatalf("Failed to initialize document: %v", err)
	}

	// Create a snapshot
	snapshot, err := tracker.CreateSnapshot("snapshot1")
	if err != nil {
		t.Fatalf("Failed to create snapshot: %v", err)
	}

	// Verify the snapshot
	if snapshot.ID != "snapshot1" {
		t.Errorf("Expected snapshot ID 'snapshot1', got '%s'", snapshot.ID)
	}

	// Update the person
	person.Name = "Jane Doe"
	person.Age = 31
	person.Email = "jane@example.com"
	person.Address.Street = "456 Main St"
	person.Tags = append(person.Tags, "tag3")

	// Update the tracker
	patch, err := tracker.Update(person)
	if err != nil {
		t.Fatalf("Failed to update tracker: %v", err)
	}

	// Apply the patch
	if err := tracker.ApplyPatch(patch); err != nil {
		t.Fatalf("Failed to apply patch: %v", err)
	}

	// Create another snapshot
	snapshot2, err := tracker.CreateSnapshot("snapshot2")
	if err != nil {
		t.Fatalf("Failed to create snapshot: %v", err)
	}

	// Verify the snapshot
	if snapshot2.ID != "snapshot2" {
		t.Errorf("Expected snapshot ID 'snapshot2', got '%s'", snapshot2.ID)
	}

	// List snapshots
	snapshots := tracker.ListSnapshots()
	if len(snapshots) != 2 {
		t.Errorf("Expected 2 snapshots, got %d", len(snapshots))
	}

	// Get a snapshot
	retrievedSnapshot, err := tracker.GetSnapshot("snapshot1")
	if err != nil {
		t.Fatalf("Failed to get snapshot: %v", err)
	}
	if retrievedSnapshot.ID != "snapshot1" {
		t.Errorf("Expected snapshot ID 'snapshot1', got '%s'", retrievedSnapshot.ID)
	}

	// Time travel to snapshot1
	travelDoc, err := tracker.TimeTravel("snapshot1")
	if err != nil {
		t.Fatalf("Failed to time travel: %v", err)
	}

	// Verify the time traveled document
	var travelPerson TestPerson
	sid2 := common.NewSessionID()
	travelTracker := NewTracker(travelDoc, sid2)
	if err := travelTracker.ToStruct(&travelPerson); err != nil {
		t.Fatalf("Failed to convert document to struct: %v", err)
	}

	// Verify that travelPerson has the original data
	if travelPerson.Name != "John Doe" {
		t.Errorf("Expected name 'John Doe', got '%s'", travelPerson.Name)
	}
	if travelPerson.Age != 30 {
		t.Errorf("Expected age 30, got %d", travelPerson.Age)
	}
	if travelPerson.Email != "john@example.com" {
		t.Errorf("Expected email 'john@example.com', got '%s'", travelPerson.Email)
	}
	if travelPerson.Address.Street != "123 Main St" {
		t.Errorf("Expected street '123 Main St', got '%s'", travelPerson.Address.Street)
	}
	if len(travelPerson.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(travelPerson.Tags))
	}

	// Revert to snapshot1
	if err := tracker.RevertToSnapshot("snapshot1"); err != nil {
		t.Fatalf("Failed to revert to snapshot: %v", err)
	}

	// Verify that the document has been reverted
	var revertedPerson TestPerson
	if err := tracker.ToStruct(&revertedPerson); err != nil {
		t.Fatalf("Failed to convert document to struct: %v", err)
	}

	// Verify that revertedPerson has the original data
	if revertedPerson.Name != "John Doe" {
		t.Errorf("Expected name 'John Doe', got '%s'", revertedPerson.Name)
	}
	if revertedPerson.Age != 30 {
		t.Errorf("Expected age 30, got %d", revertedPerson.Age)
	}
	if revertedPerson.Email != "john@example.com" {
		t.Errorf("Expected email 'john@example.com', got '%s'", revertedPerson.Email)
	}
	if revertedPerson.Address.Street != "123 Main St" {
		t.Errorf("Expected street '123 Main St', got '%s'", revertedPerson.Address.Street)
	}
	if len(revertedPerson.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(revertedPerson.Tags))
	}

	// Delete a snapshot
	if err := tracker.DeleteSnapshot("snapshot1"); err != nil {
		t.Fatalf("Failed to delete snapshot: %v", err)
	}

	// Verify that the snapshot has been deleted
	snapshots = tracker.ListSnapshots()
	if len(snapshots) != 1 {
		t.Errorf("Expected 1 snapshot, got %d", len(snapshots))
	}
	if snapshots[0].ID != "snapshot2" {
		t.Errorf("Expected snapshot ID 'snapshot2', got '%s'", snapshots[0].ID)
	}
}

// TestTimeTravelToTime tests the time travel to time functionality.
func TestTimeTravelToTime(t *testing.T) {
	// Create a new document
	sid := common.NewSessionID()
	doc := crdt.NewDocument(sid)

	// Create a test person
	person := &TestPerson{
		Name:    "John Doe",
		Age:     30,
		Email:   "john@example.com",
		Created: time.Now(),
		Tags:    []string{"tag1", "tag2"},
	}
	person.Address.Street = "123 Main St"
	person.Address.City = "New York"
	person.Address.Country = "USA"

	// Create a tracker and initialize the document
	tracker := NewTracker(doc, sid)
	if err := tracker.InitializeDocument(person); err != nil {
		t.Fatalf("Failed to initialize document: %v", err)
	}

	// Create a snapshot
	snapshot1Time := time.Now()
	_, err := tracker.CreateSnapshot("snapshot1")
	if err != nil {
		t.Fatalf("Failed to create snapshot: %v", err)
	}

	// Wait a bit to ensure different timestamps
	time.Sleep(100 * time.Millisecond)

	// Update the person
	person.Name = "Jane Doe"
	person.Age = 31
	person.Email = "jane@example.com"
	person.Address.Street = "456 Main St"
	person.Tags = append(person.Tags, "tag3")

	// Update the tracker
	patch, err := tracker.Update(person)
	if err != nil {
		t.Fatalf("Failed to update tracker: %v", err)
	}

	// Apply the patch
	if err := tracker.ApplyPatch(patch); err != nil {
		t.Fatalf("Failed to apply patch: %v", err)
	}

	// Create another snapshot
	snapshot2Time := time.Now()
	_, err = tracker.CreateSnapshot("snapshot2")
	if err != nil {
		t.Fatalf("Failed to create snapshot: %v", err)
	}

	// Wait a bit to ensure different timestamps
	time.Sleep(100 * time.Millisecond)

	// Update the person again
	person.Name = "Bob Smith"
	person.Age = 40
	person.Email = "bob@example.com"
	person.Address.Street = "789 Main St"
	person.Tags = append(person.Tags, "tag4")

	// Update the tracker
	patch, err = tracker.Update(person)
	if err != nil {
		t.Fatalf("Failed to update tracker: %v", err)
	}

	// Apply the patch
	if err := tracker.ApplyPatch(patch); err != nil {
		t.Fatalf("Failed to apply patch: %v", err)
	}

	// Create another snapshot
	_, err = tracker.CreateSnapshot("snapshot3")
	if err != nil {
		t.Fatalf("Failed to create snapshot: %v", err)
	}

	// Time travel to a time close to snapshot1
	travelDoc, err := tracker.TimeTravelToTime(snapshot1Time.Add(50 * time.Millisecond))
	if err != nil {
		t.Fatalf("Failed to time travel: %v", err)
	}

	// Verify the time traveled document
	var travelPerson TestPerson
	sid2 := common.NewSessionID()
	travelTracker := NewTracker(travelDoc, sid2)
	if err := travelTracker.ToStruct(&travelPerson); err != nil {
		t.Fatalf("Failed to convert document to struct: %v", err)
	}

	// Verify that travelPerson has the original data
	if travelPerson.Name != "John Doe" {
		t.Errorf("Expected name 'John Doe', got '%s'", travelPerson.Name)
	}
	if travelPerson.Age != 30 {
		t.Errorf("Expected age 30, got %d", travelPerson.Age)
	}

	// Time travel to a time close to snapshot2
	travelDoc, err = tracker.TimeTravelToTime(snapshot2Time.Add(50 * time.Millisecond))
	if err != nil {
		t.Fatalf("Failed to time travel: %v", err)
	}

	// Verify the time traveled document
	sid3 := common.NewSessionID()
	travelTracker = NewTracker(travelDoc, sid3)
	if err := travelTracker.ToStruct(&travelPerson); err != nil {
		t.Fatalf("Failed to convert document to struct: %v", err)
	}

	// Verify that travelPerson has the updated data
	if travelPerson.Name != "Jane Doe" {
		t.Errorf("Expected name 'Jane Doe', got '%s'", travelPerson.Name)
	}
	if travelPerson.Age != 31 {
		t.Errorf("Expected age 31, got %d", travelPerson.Age)
	}

	// Revert to a time close to snapshot1
	if err := tracker.RevertToTime(snapshot1Time.Add(50 * time.Millisecond)); err != nil {
		t.Fatalf("Failed to revert to time: %v", err)
	}

	// Verify that the document has been reverted
	var revertedPerson TestPerson
	if err := tracker.ToStruct(&revertedPerson); err != nil {
		t.Fatalf("Failed to convert document to struct: %v", err)
	}

	// Verify that revertedPerson has the original data
	if revertedPerson.Name != "John Doe" {
		t.Errorf("Expected name 'John Doe', got '%s'", revertedPerson.Name)
	}
	if revertedPerson.Age != 30 {
		t.Errorf("Expected age 30, got %d", revertedPerson.Age)
	}
}
