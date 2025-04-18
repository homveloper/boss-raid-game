package main

import (
	"encoding/json"
	"fmt"
	"log"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
	"tictactoe/luvjson/crdtpatch"
)

func main() {
	// Create a new document
	doc := crdt.NewDocument(common.NewSessionID())

	// Create a new patch
	patchID := common.LogicalTimestamp{SID: 1, Counter: 1}
	p := crdtpatch.NewPatch(patchID)

	// Add metadata to the patch
	p.SetMetadata(map[string]interface{}{
		"author": "John Doe",
	})

	// Create a new object node
	newObjOp := &crdtpatch.NewOperation{
		ID:       patchID,
		NodeType: common.NodeTypeObj,
	}
	p.AddOperation(newObjOp)

	// Create a new string node
	newStrOp := &crdtpatch.NewOperation{
		ID:       common.LogicalTimestamp{SID: 1, Counter: 2},
		NodeType: common.NodeTypeStr,
	}
	p.AddOperation(newStrOp)

	// Apply the patch to the document
	if err := p.Apply(doc); err != nil {
		log.Fatalf("Failed to apply patch: %v", err)
	}

	// Convert the patch to JSON
	patchJSON, err := json.Marshal(p)
	if err != nil {
		log.Fatalf("Failed to convert patch to JSON: %v", err)
	}

	fmt.Println("Patch JSON:")
	fmt.Println(string(patchJSON))

	// Convert the document to JSON
	docJSON, err := json.Marshal(doc)
	if err != nil {
		log.Fatalf("Failed to convert document to JSON: %v", err)
	}

	fmt.Println("\nDocument JSON:")
	fmt.Println(string(docJSON))

	// Get the document view
	view, err := doc.View()
	if err != nil {
		log.Fatalf("Failed to get document view: %v", err)
	}

	viewJSON, err := json.Marshal(view)
	if err != nil {
		log.Fatalf("Failed to convert view to JSON: %v", err)
	}

	fmt.Println("\nDocument View:")
	fmt.Println(string(viewJSON))
}
