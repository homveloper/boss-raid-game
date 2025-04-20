package main

import (
	"fmt"
	"log"

	"tictactoe/luvjson/api"
	"tictactoe/luvjson/common"
)

func main() {
	// Create a new model with a random session ID
	model := api.NewModel(common.NewSessionID())

	// Print the initial state
	printModelState(model, "Initial state")

	// Set the root value to an object
	model.GetApi().Root(map[string]interface{}{
		"counter": 0,
		"text":    "Hello",
	})

	// Print the state after setting the root
	printModelState(model, "After setting root")

	// Update the counter field
	model.GetApi().Obj([]interface{}{}).Set(map[string]interface{}{
		"counter": 25,
	})

	// Print the state after updating the counter
	printModelState(model, "After updating counter")

	// Get the text field and insert text
	textNode, err := api.ResolveNode(model.GetDocument(), api.Path{api.StringPathElement("text")})
	if err != nil {
		log.Fatalf("Failed to resolve text node: %v", err)
	}

	strApi := model.GetApi().Wrap(textNode).(*api.StrApi)
	strApi.Ins(5, " world!")

	// Print the state after inserting text
	printModelState(model, "After inserting text")

	// Flush the changes and get the patch
	patch := model.GetApi().Flush()
	fmt.Printf("Patch: %v\n", patch)
}

// printModelState prints the current state of the model.
func printModelState(model *api.Model, label string) {
	fmt.Printf("\n=== %s ===\n", label)

	// Print the document structure
	fmt.Printf("Document: %s\n", model.String())

	// Print the document view
	view, err := model.View()
	if err != nil {
		fmt.Printf("Error getting view: %v\n", err)
		return
	}
	fmt.Printf("View: %v\n", view)
}
