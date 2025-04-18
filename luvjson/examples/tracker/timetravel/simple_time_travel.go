package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
	"tictactoe/luvjson/tracker"
)

// Document represents a simple document with version history.
type Document struct {
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Author    string    `json:"author"`
	UpdatedAt time.Time `json:"updatedAt"`
	Version   int       `json:"version"`
}

// printJSON prints an object as formatted JSON.
func printJSON(obj interface{}) {
	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal to JSON: %v", err)
	}
	fmt.Println(string(data))
}

// printStep prints a step header.
func printStep(step string) {
	fmt.Println("\n===", step, "===")
}

func main() {
	printStep("CRDT Document Time Travel Example")

	// Create a CRDT document
	crdtDoc := crdt.NewDocument(common.NewSessionID())

	// Create a tracker
	t := tracker.NewTracker(crdtDoc, 1)

	// Create initial document (version 1)
	document := &Document{
		Title:     "Hello World",
		Content:   "This is the initial content.",
		Author:    "John Doe",
		UpdatedAt: time.Now(),
		Version:   1,
	}

	printStep("Version 1")
	printJSON(document)

	// Initialize the CRDT document with the document
	if err := t.InitializeDocument(document); err != nil {
		log.Fatalf("Failed to initialize document: %v", err)
	}

	// Create a snapshot for version 1
	snapshot1, err := t.CreateSnapshot("v1")
	if err != nil {
		log.Fatalf("Failed to create snapshot: %v", err)
	}

	printStep("Snapshot 1")
	printJSON(map[string]interface{}{
		"id":        snapshot1.ID,
		"timestamp": snapshot1.Timestamp.Format(time.RFC3339),
	})

	// Wait a bit to ensure different timestamps
	time.Sleep(100 * time.Millisecond)

	// Update to version 2
	document.Title = "Updated Title"
	document.Content = "This content has been updated."
	document.Author = "Jane Smith"
	document.UpdatedAt = time.Now()
	document.Version = 2

	printStep("Version 2")
	printJSON(document)

	// Update the tracker
	patch, err := t.Update(document)
	if err != nil {
		log.Fatalf("Failed to update tracker: %v", err)
	}

	// Apply the patch
	if err := t.ApplyPatch(patch); err != nil {
		log.Fatalf("Failed to apply patch: %v", err)
	}

	// Create a snapshot for version 2
	snapshot2, err := t.CreateSnapshot("v2")
	if err != nil {
		log.Fatalf("Failed to create snapshot: %v", err)
	}

	printStep("Snapshot 2")
	printJSON(map[string]interface{}{
		"id":        snapshot2.ID,
		"timestamp": snapshot2.Timestamp.Format(time.RFC3339),
	})

	// Wait a bit to ensure different timestamps
	time.Sleep(100 * time.Millisecond)

	// Update to version 3
	document.Title = "Final Title"
	document.Content = "This is the final content with more details and information."
	document.Author = "Bob Johnson"
	document.UpdatedAt = time.Now()
	document.Version = 3

	printStep("Version 3")
	printJSON(document)

	// Update the tracker
	patch, err = t.Update(document)
	if err != nil {
		log.Fatalf("Failed to update tracker: %v", err)
	}

	// Apply the patch
	if err := t.ApplyPatch(patch); err != nil {
		log.Fatalf("Failed to apply patch: %v", err)
	}

	// Create a snapshot for version 3
	snapshot3, err := t.CreateSnapshot("v3")
	if err != nil {
		log.Fatalf("Failed to create snapshot: %v", err)
	}

	printStep("Snapshot 3")
	printJSON(map[string]interface{}{
		"id":        snapshot3.ID,
		"timestamp": snapshot3.Timestamp.Format(time.RFC3339),
	})

	// List all snapshots
	printStep("All Snapshots")
	snapshots := t.ListSnapshots()
	snapshotList := make([]map[string]interface{}, 0)
	for _, snapshot := range snapshots {
		snapshotList = append(snapshotList, map[string]interface{}{
			"id":        snapshot.ID,
			"timestamp": snapshot.Timestamp.Format(time.RFC3339),
		})
	}
	printJSON(snapshotList)

	// Time Travel Example 1: Create a new document from version 1
	printStep("Time Travel to Version 1")
	travelDoc1, err := t.TimeTravel("v1")
	if err != nil {
		log.Fatalf("Failed to time travel: %v", err)
	}

	// Create a new tracker for the time traveled document
	travelTracker1 := tracker.NewTracker(travelDoc1, 2)

	// Get the document from the time traveled document
	travelDocument1 := &Document{}
	if err := travelTracker1.ToStruct(travelDocument1); err != nil {
		log.Fatalf("Failed to get document from time traveled document: %v", err)
	}

	printJSON(travelDocument1)

	// Time Travel Example 2: Create a new document from version 2
	printStep("Time Travel to Version 2")
	travelDoc2, err := t.TimeTravel("v2")
	if err != nil {
		log.Fatalf("Failed to time travel: %v", err)
	}

	// Create a new tracker for the time traveled document
	travelTracker2 := tracker.NewTracker(travelDoc2, 3)

	// Get the document from the time traveled document
	travelDocument2 := &Document{}
	if err := travelTracker2.ToStruct(travelDocument2); err != nil {
		log.Fatalf("Failed to get document from time traveled document: %v", err)
	}

	printJSON(travelDocument2)

	// Demonstrate creating a new branch from version 1
	printStep("Creating a New Branch from Version 1")

	// Create a new document from version 1
	branchDoc, err := t.TimeTravel("v1")
	if err != nil {
		log.Fatalf("Failed to time travel: %v", err)
	}

	// Create a new tracker for the branch document
	branchTracker := tracker.NewTracker(branchDoc, 4)

	// Get the document from the branch document
	branchDocument := &Document{}
	if err := branchTracker.ToStruct(branchDocument); err != nil {
		log.Fatalf("Failed to get document from branch document: %v", err)
	}

	// Update the branch document
	branchDocument.Title = "Branch Title"
	branchDocument.Content = "This is a branch from version 1 with different content."
	branchDocument.Author = "Alice Williams"
	branchDocument.UpdatedAt = time.Now()
	branchDocument.Version = 4

	printJSON(branchDocument)

	// Re-initialize the document with the updated branch document
	if err := branchTracker.InitializeDocument(branchDocument); err != nil {
		log.Fatalf("Failed to initialize branch document: %v", err)
	}

	// Verify the document was updated
	updatedBranchDoc := &Document{}
	if err := branchTracker.ToStruct(updatedBranchDoc); err != nil {
		log.Fatalf("Failed to get updated branch document: %v", err)
	}

	printStep("Branch Document After Initialization")
	printJSON(updatedBranchDoc)

	// Create a snapshot for the branch
	branchSnapshot, err := branchTracker.CreateSnapshot("branch")
	if err != nil {
		log.Fatalf("Failed to create snapshot for branch: %v", err)
	}

	printStep("Branch Snapshot")
	printJSON(map[string]interface{}{
		"id":        branchSnapshot.ID,
		"timestamp": branchSnapshot.Timestamp.Format(time.RFC3339),
	})

	// Get the updated branch document
	updatedBranchDocument := &Document{}
	if err := branchTracker.ToStruct(updatedBranchDocument); err != nil {
		log.Fatalf("Failed to get updated branch document: %v", err)
	}

	printStep("Updated Branch Document")
	printJSON(updatedBranchDocument)

	// Show that the original document is still at version 3
	printStep("Original Document (still at version 3)")
	currentDocument := &Document{}
	if err := t.ToStruct(currentDocument); err != nil {
		log.Fatalf("Failed to get current document: %v", err)
	}
	printJSON(currentDocument)
}
