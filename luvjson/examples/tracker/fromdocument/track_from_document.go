package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
	"tictactoe/luvjson/crdtpatch"
	"tictactoe/luvjson/tracker"
)

// Person represents a person.
type Person struct {
	Name    string    `json:"name"`
	Age     int       `json:"age"`
	Email   string    `json:"email"`
	Created time.Time `json:"created"`
	Address struct {
		Street  string `json:"street"`
		City    string `json:"city"`
		Country string `json:"country"`
	} `json:"address"`
	Tags []string `json:"tags"`
}

// printPerson prints a person.
func printPerson(person *Person) {
	fmt.Printf("Name: %s\n", person.Name)
	fmt.Printf("Age: %d\n", person.Age)
	fmt.Printf("Email: %s\n", person.Email)
	fmt.Printf("Created: %s\n", person.Created.Format(time.RFC3339))
	fmt.Printf("Address: %s, %s, %s\n", person.Address.Street, person.Address.City, person.Address.Country)
	fmt.Printf("Tags: %v\n", person.Tags)
}

// printPatch prints a patch.
func printPatch(patch *crdtpatch.Patch) {
	fmt.Printf("Patch ID: %v\n", patch.ID())
	fmt.Printf("Operations: %d\n", len(patch.Operations()))
	for i, op := range patch.Operations() {
		data, err := json.Marshal(op)
		if err != nil {
			log.Fatalf("Failed to marshal operation to JSON: %v", err)
		}
		fmt.Printf("Operation %d: %s\n", i+1, string(data))
	}
}

// printJSON prints an object as formatted JSON.
func printJSON(obj interface{}) {
	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal to JSON: %v", err)
	}
	fmt.Println(string(data))
}

func main() {
	// Create a CRDT document
	doc := crdt.NewDocument(common.NewSessionID())

	// Create a person
	originalPerson := &Person{
		Name:    "John Doe",
		Age:     30,
		Email:   "john@example.com",
		Created: time.Now(),
		Tags:    []string{"tag1", "tag2"},
	}
	originalPerson.Address.Street = "123 Main St"
	originalPerson.Address.City = "New York"
	originalPerson.Address.Country = "USA"

	fmt.Println("Original person:")
	printPerson(originalPerson)
	fmt.Println()

	// Create a tracker and initialize the document with the original person
	tracker1 := tracker.NewTracker(doc, 1)
	if err := tracker1.InitializeDocument(originalPerson); err != nil {
		log.Fatalf("Failed to initialize document: %v", err)
	}

	fmt.Println("Document initialized with original person")
	fmt.Println()

	// Scenario 1: Using TrackFromDocument
	fmt.Println("Scenario 1: Using TrackFromDocument")
	fmt.Println("-----------------------------------")

	// Create a new tracker from the same document
	tracker2 := tracker.NewTracker(doc, 2)

	// Create an empty person to fill with document data
	person1 := &Person{}

	// Track from document
	if err := tracker2.TrackFromDocument(person1); err != nil {
		log.Fatalf("Failed to track from document: %v", err)
	}

	fmt.Println("Person loaded from document:")
	printPerson(person1)
	fmt.Println()

	// Update the person
	fmt.Println("Updating person...")
	person1.Name = "Jane Doe"
	person1.Age = 31
	person1.Email = "jane@example.com"
	person1.Address.Street = "456 Main St"
	person1.Tags = append(person1.Tags, "tag3")

	// Generate a patch
	patch1, err := tracker2.Update(person1)
	if err != nil {
		log.Fatalf("Failed to update tracker: %v", err)
	}

	// Apply the patch to the document
	if err := tracker2.ApplyPatch(patch1); err != nil {
		log.Fatalf("Failed to apply patch: %v", err)
	}

	fmt.Println("Updated person:")
	printPerson(person1)
	fmt.Println()

	fmt.Println("Generated patch:")
	printPatch(patch1)
	fmt.Println()

	// Scenario 2: Using NewTrackerFromDocument
	fmt.Println("Scenario 2: Using NewTrackerFromDocument")
	fmt.Println("---------------------------------------")

	// Create an empty person to fill with document data
	person2 := &Person{}

	// Create a new tracker from the document
	tracker3, err := tracker.NewTrackerFromDocument(doc, 3, person2)
	if err != nil {
		log.Fatalf("Failed to create tracker from document: %v", err)
	}

	fmt.Println("Person loaded from document:")
	printPerson(person2)
	fmt.Println()

	// Update the person
	fmt.Println("Updating person...")
	person2.Name = "Bob Smith"
	person2.Age = 40
	person2.Email = "bob@example.com"
	person2.Address.Street = "789 Main St"
	person2.Tags = []string{"tag1", "tag2", "tag3", "tag4"}

	// Generate a patch
	patch2, err := tracker3.Update(person2)
	if err != nil {
		log.Fatalf("Failed to update tracker: %v", err)
	}

	// Apply the patch to the document
	if err := tracker3.ApplyPatch(patch2); err != nil {
		log.Fatalf("Failed to apply patch: %v", err)
	}

	fmt.Println("Updated person:")
	printPerson(person2)
	fmt.Println()

	fmt.Println("Generated patch:")
	printPatch(patch2)
	fmt.Println()

	// Get the final document view
	fmt.Println("Final document view:")
	view, err := doc.View()
	if err != nil {
		log.Fatalf("Failed to get document view: %v", err)
	}
	printJSON(view)
}
