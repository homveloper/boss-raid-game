package main

import (
	"fmt"

	"github.com/google/uuid"
)

func main() {
	fmt.Printf("uuid.Nil: %v\n", uuid.Nil)
	
	// Create a new UUID v7
	id, err := uuid.NewV7()
	if err != nil {
		fmt.Printf("Error creating UUID v7: %v\n", err)
		return
	}
	
	fmt.Printf("UUID v7: %v\n", id)
}
