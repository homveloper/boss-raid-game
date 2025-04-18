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
	// Create a new document with session ID 1
	doc := crdt.NewDocument(common.NewSessionID())

	// Create a new patch to add an object
	patchID := doc.NextTimestamp()
	p1 := crdtpatch.NewPatch(patchID)

	// Create a new object node
	objID := patchID
	newObjOp := &crdtpatch.NewOperation{
		ID:       objID,
		NodeType: common.NodeTypeObj,
	}
	p1.AddOperation(newObjOp)

	// Apply the patch to create the object
	if err := p1.Apply(doc); err != nil {
		log.Fatalf("Failed to apply patch 1: %v", err)
	}

	// Get the root node and set its value to the new object
	rootNode, err := doc.GetNode(common.LogicalTimestamp{SID: 0, Counter: 0})
	if err != nil {
		log.Fatalf("Failed to get root node: %v", err)
	}

	objNode, err := doc.GetNode(objID)
	if err != nil {
		log.Fatalf("Failed to get object node: %v", err)
	}

	rootLWW := rootNode.(*crdt.LWWValueNode)
	rootLWW.SetValue(patchID, objNode)

	// Create a new patch to add a field to the object
	patchID2 := doc.NextTimestamp()
	p2 := crdtpatch.NewPatch(patchID2)

	// Create a new constant node for the field value
	nameID := patchID2
	nameOp := &crdtpatch.NewOperation{
		ID:       nameID,
		NodeType: common.NodeTypeCon,
		Value:    "John Doe",
	}
	p2.AddOperation(nameOp)

	// Create an insert operation to add the field to the object
	insOp := &crdtpatch.InsOperation{
		ID:       doc.NextTimestamp(),
		TargetID: objID,
		Value: map[string]interface{}{
			"name": "John Doe",
		},
	}
	p2.AddOperation(insOp)

	// Apply the patch to add the field
	if err := p2.Apply(doc); err != nil {
		log.Fatalf("Failed to apply patch 2: %v", err)
	}

	// Create a new patch to add another field to the object
	patchID3 := doc.NextTimestamp()
	p3 := crdtpatch.NewPatch(patchID3)

	// Create a new constant node for the field value
	ageID := patchID3
	ageOp := &crdtpatch.NewOperation{
		ID:       ageID,
		NodeType: common.NodeTypeCon,
		Value:    30,
	}
	p3.AddOperation(ageOp)

	// Create an insert operation to add the field to the object
	insOp2 := &crdtpatch.InsOperation{
		ID:       doc.NextTimestamp(),
		TargetID: objID,
		Value: map[string]interface{}{
			"age": 30,
		},
	}
	p3.AddOperation(insOp2)

	// Apply the patch to add the field
	if err := p3.Apply(doc); err != nil {
		log.Fatalf("Failed to apply patch 3: %v", err)
	}

	// Get the document view
	view, err := doc.View()
	if err != nil {
		log.Fatalf("Failed to get document view: %v", err)
	}

	// Convert the view to JSON
	viewJSON, err := json.MarshalIndent(view, "", "  ")
	if err != nil {
		log.Fatalf("Failed to convert view to JSON: %v", err)
	}

	fmt.Println("Document View:")
	fmt.Println(string(viewJSON))
}
