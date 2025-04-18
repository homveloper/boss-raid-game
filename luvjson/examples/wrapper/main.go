package main

import (
	"fmt"
	"log"

	"tictactoe/luvjson/wrapper"
)

// User is a user-defined struct.
type User struct {
	Name    string   `json:"name" crdt:"name"`
	Age     int      `json:"age" crdt:"age"`
	Email   string   `json:"email" crdt:"email"`
	Tags    []string `json:"tags" crdt:"tags"`
	Street  string   `json:"street" crdt:"street"`
	City    string   `json:"city" crdt:"city"`
	Country string   `json:"country" crdt:"country"`
	Secret  string   `json:"secret" crdt:"secret,ignore"` // Ignored in CRDT
}

func main() {
	// Create a new CRDT document
	doc := wrapper.NewCRDTDocument(1)

	// Create a user
	user := User{
		Name:    "John Doe",
		Age:     30,
		Email:   "john@example.com",
		Tags:    []string{"developer", "golang"},
		Street:  "123 Main St",
		City:    "New York",
		Country: "USA",
		Secret:  "password123",
	}

	// Initialize the document from the user struct
	if err := doc.FromStruct(&user); err != nil {
		log.Fatalf("Failed to initialize document: %v", err)
	}

	// Get the user from the document
	var retrievedUser User
	if err := doc.ToStruct(&retrievedUser); err != nil {
		log.Fatalf("Failed to get user from document: %v", err)
	}

	fmt.Println("Retrieved User:")
	fmt.Printf("Name: %s\n", retrievedUser.Name)
	fmt.Printf("Age: %d\n", retrievedUser.Age)
	fmt.Printf("Email: %s\n", retrievedUser.Email)
	fmt.Printf("Tags: %v\n", retrievedUser.Tags)
	fmt.Printf("Address: %s, %s, %s\n", retrievedUser.Street, retrievedUser.City, retrievedUser.Country)
	fmt.Printf("Secret: %s\n", retrievedUser.Secret) // Should be empty

	// Update a field
	if err := doc.UpdateField("age", 31); err != nil {
		log.Fatalf("Failed to update field: %v", err)
	}

	// Get the updated user
	var updatedUser User
	if err := doc.ToStruct(&updatedUser); err != nil {
		log.Fatalf("Failed to get updated user: %v", err)
	}

	fmt.Println("\nUpdated User:")
	fmt.Printf("Name: %s\n", updatedUser.Name)
	fmt.Printf("Age: %d\n", updatedUser.Age) // Should be 31

	// Update the user struct
	user.Email = "john.doe@example.com"

	// Update the document from the user struct
	_, err := doc.UpdateStruct(&user)
	if err != nil {
		log.Fatalf("Failed to update document: %v", err)
	}

	// Get the updated user again
	var updatedUser2 User
	if err := doc.ToStruct(&updatedUser2); err != nil {
		log.Fatalf("Failed to get updated user: %v", err)
	}

	fmt.Println("\nUpdated User (after struct update):")
	fmt.Printf("Name: %s\n", updatedUser2.Name)
	fmt.Printf("Age: %d\n", updatedUser2.Age)     // Should still be 31
	fmt.Printf("Email: %s\n", updatedUser2.Email) // Should be john.doe@example.com

	// Create a second CRDT document
	doc2 := wrapper.NewCRDTDocument(2)

	// Initialize the second document from a different user
	user2 := User{
		Name:    "Jane Smith",
		Age:     28,
		Email:   "jane@example.com",
		Tags:    []string{"designer", "ui"},
		Street:  "456 Oak St",
		City:    "San Francisco",
		Country: "USA",
	}

	if err := doc2.FromStruct(&user2); err != nil {
		log.Fatalf("Failed to initialize second document: %v", err)
	}

	// Get the user from the second document
	var user2FromDoc User
	if err := doc2.ToStruct(&user2FromDoc); err != nil {
		log.Fatalf("Failed to get user from second document: %v", err)
	}

	// Update the first document with the user from the second document
	_, err = doc.UpdateStruct(&user2FromDoc)
	if err != nil {
		log.Fatalf("Failed to update document: %v", err)
	}

	// Get the merged user
	var mergedUser User
	if err := doc.ToStruct(&mergedUser); err != nil {
		log.Fatalf("Failed to get merged user: %v", err)
	}

	fmt.Println("\nMerged User:")
	fmt.Printf("Name: %s\n", mergedUser.Name)
	fmt.Printf("Age: %d\n", mergedUser.Age)
	fmt.Printf("Email: %s\n", mergedUser.Email)
}
