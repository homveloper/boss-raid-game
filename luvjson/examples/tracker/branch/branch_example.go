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
	printStep("CRDT Document Branching Example")

	// Create a CRDT document for the main branch
	mainDoc := crdt.NewDocument(common.NewSessionID())

	// Create a tracker for the main branch
	mainTracker := tracker.NewTracker(mainDoc, 1)

	// Create initial document (version 1)
	document := &Document{
		Title:     "Hello World",
		Content:   "This is the initial content.",
		Author:    "John Doe",
		UpdatedAt: time.Now(),
		Version:   1,
	}

	printStep("Main Branch - Version 1")
	printJSON(document)

	// Initialize the CRDT document with the document
	if err := mainTracker.InitializeDocument(document); err != nil {
		log.Fatalf("Failed to initialize document: %v", err)
	}

	// Create a snapshot for version 1
	snapshot1, err := mainTracker.CreateSnapshot("v1")
	if err != nil {
		log.Fatalf("Failed to create snapshot: %v", err)
	}

	printStep("Main Branch - Snapshot 1")
	printJSON(map[string]interface{}{
		"id":        snapshot1.ID,
		"timestamp": snapshot1.Timestamp.Format(time.RFC3339),
	})

	// Wait a bit to ensure different timestamps
	time.Sleep(100 * time.Millisecond)

	// Update to version 2 on the main branch
	document.Title = "Updated Title"
	document.Content = "This content has been updated."
	document.Author = "Jane Smith"
	document.UpdatedAt = time.Now()
	document.Version = 2

	printStep("Main Branch - Version 2")
	printJSON(document)

	// Update the tracker
	patch, err := mainTracker.Update(document)
	if err != nil {
		log.Fatalf("Failed to update tracker: %v", err)
	}

	// Apply the patch
	if err := mainTracker.ApplyPatch(patch); err != nil {
		log.Fatalf("Failed to apply patch: %v", err)
	}

	// Create a snapshot for version 2
	snapshot2, err := mainTracker.CreateSnapshot("v2")
	if err != nil {
		log.Fatalf("Failed to create snapshot: %v", err)
	}

	printStep("Main Branch - Snapshot 2")
	printJSON(map[string]interface{}{
		"id":        snapshot2.ID,
		"timestamp": snapshot2.Timestamp.Format(time.RFC3339),
	})

	// Create a feature branch from version 1
	printStep("Creating Feature Branch from Version 1")

	// Get the document from version 1
	v1Doc, err := mainTracker.TimeTravel("v1")
	if err != nil {
		log.Fatalf("Failed to time travel to version 1: %v", err)
	}

	// Create a new document for the feature branch
	featureDoc := crdt.NewDocument(2)
	featureTracker := tracker.NewTracker(featureDoc, 2)

	// Get the document from version 1
	v1Document := &Document{}
	v1Tracker := tracker.NewTracker(v1Doc, 3)
	if err := v1Tracker.ToStruct(v1Document); err != nil {
		log.Fatalf("Failed to get document from version 1: %v", err)
	}

	printStep("Feature Branch - Starting Point (Version 1)")
	printJSON(v1Document)

	// Initialize the feature branch with version 1
	if err := featureTracker.InitializeDocument(v1Document); err != nil {
		log.Fatalf("Failed to initialize feature branch: %v", err)
	}

	// Update the document on the feature branch
	featureDocument := &Document{}
	if err := featureTracker.ToStruct(featureDocument); err != nil {
		log.Fatalf("Failed to get document from feature branch: %v", err)
	}

	featureDocument.Title = "Feature Branch Title"
	featureDocument.Content = "This is a feature branch with different content."
	featureDocument.Author = "Alice Williams"
	featureDocument.UpdatedAt = time.Now()
	featureDocument.Version = 3

	printStep("Feature Branch - Updated Document")
	printJSON(featureDocument)

	// Update the feature branch tracker
	featurePatch, err := featureTracker.Update(featureDocument)
	if err != nil {
		log.Fatalf("Failed to update feature branch tracker: %v", err)
	}

	// Apply the patch to the feature branch
	if err := featureTracker.ApplyPatch(featurePatch); err != nil {
		log.Fatalf("Failed to apply patch to feature branch: %v", err)
	}

	// Create a snapshot for the feature branch
	featureSnapshot, err := featureTracker.CreateSnapshot("feature")
	if err != nil {
		log.Fatalf("Failed to create snapshot for feature branch: %v", err)
	}

	printStep("Feature Branch - Snapshot")
	printJSON(map[string]interface{}{
		"id":        featureSnapshot.ID,
		"timestamp": featureSnapshot.Timestamp.Format(time.RFC3339),
	})

	// Verify the feature branch document
	updatedFeatureDocument := &Document{}
	if err := featureTracker.ToStruct(updatedFeatureDocument); err != nil {
		log.Fatalf("Failed to get updated feature branch document: %v", err)
	}

	printStep("Feature Branch - Final Document")
	printJSON(updatedFeatureDocument)

	// Meanwhile, update the main branch to version 3
	document.Title = "Main Branch Final Title"
	document.Content = "This is the final content on the main branch."
	document.Author = "Bob Johnson"
	document.UpdatedAt = time.Now()
	document.Version = 3

	printStep("Main Branch - Version 3")
	printJSON(document)

	// Update the main branch tracker
	mainPatch, err := mainTracker.Update(document)
	if err != nil {
		log.Fatalf("Failed to update main branch tracker: %v", err)
	}

	// Apply the patch to the main branch
	if err := mainTracker.ApplyPatch(mainPatch); err != nil {
		log.Fatalf("Failed to apply patch to main branch: %v", err)
	}

	// Create a snapshot for version 3
	snapshot3, err := mainTracker.CreateSnapshot("v3")
	if err != nil {
		log.Fatalf("Failed to create snapshot for version 3: %v", err)
	}

	printStep("Main Branch - Snapshot 3")
	printJSON(map[string]interface{}{
		"id":        snapshot3.ID,
		"timestamp": snapshot3.Timestamp.Format(time.RFC3339),
	})

	// Show the final state of both branches
	printStep("Final State - Main Branch")
	mainFinalDocument := &Document{}
	if err := mainTracker.ToStruct(mainFinalDocument); err != nil {
		log.Fatalf("Failed to get final main branch document: %v", err)
	}
	printJSON(mainFinalDocument)

	printStep("Final State - Feature Branch")
	featureFinalDocument := &Document{}
	if err := featureTracker.ToStruct(featureFinalDocument); err != nil {
		log.Fatalf("Failed to get final feature branch document: %v", err)
	}
	printJSON(featureFinalDocument)
}
