# LuvJSON - JSON CRDT for Go

LuvJSON is a Go implementation of the JSON-CRDT and JSON-CRDT-Patch technologies, inspired by the [json-joy](https://github.com/streamich/json-joy) library. It provides a conflict-free replicated data type (CRDT) for JSON documents, allowing multiple users to concurrently edit the same document without conflicts.

## Features

- **JSON CRDT**: A full JSON-like conflict-free replicated data type (CRDT)
- **JSON CRDT Patch**: A patch protocol for describing changes to JSON CRDT documents
- **Multiple Encoding Formats**: Supports verbose, compact, and binary encoding formats
- **Node Types**: Supports all JSON data types, including null, boolean, number, string, array, and object
- **CRDT Algorithms**: Implements three CRDT algorithms: constant, last-write-wins register, and replicated growable array

## Installation

```bash
go get github.com/yourusername/luvjson
```

## Usage

### Creating a Document

```go
import (
    "tictactoe/luvjson/common"
    "tictactoe/luvjson/crdt"
)

// Create a new document with session ID 1
doc := crdt.NewDocument(common.NewSessionID())
```

### Creating and Applying a Patch

```go
import (
    "tictactoe/luvjson/common"
    "tictactoe/luvjson/crdt"
    "tictactoe/luvjson/crdtpatch"
)

// Create a new document
doc := crdt.NewDocument(common.NewSessionID())

// Create a new patch
patchID := common.LogicalTimestamp{SessionID: 1, Counter: 1}
p := crdtpatch.NewPatch(patchID)

// Add metadata to the patch
p.SetMetadata(map[string]interface{}{
    "author": "John Doe",
})

// Create a new object node
newObjOp := &crdtpatch.CreateOperation{
    ID:       patchID,
    NodeType: common.NodeTypeObj,
}
p.AddOperation(newObjOp)

// Apply the patch to the document
if err := p.Apply(doc); err != nil {
    log.Fatalf("Failed to apply patch: %v", err)
}
```

### Converting to JSON

```go
// Convert the patch to JSON
patchJSON, err := p.ToJSON(common.EncodingFormatVerbose)
if err != nil {
    log.Fatalf("Failed to convert patch to JSON: %v", err)
}

// Convert the document to JSON
docJSON, err := doc.ToJSON(common.EncodingFormatVerbose)
if err != nil {
    log.Fatalf("Failed to convert document to JSON: %v", err)
}

// Get the document view
view, err := doc.View()
if err != nil {
    log.Fatalf("Failed to get document view: %v", err)
}
```

## Architecture

LuvJSON consists of three main packages:

- **common**: Contains common types and utilities used by the other packages
- **crdt**: Implements the JSON CRDT document model
- **crdtpatch**: Implements the JSON CRDT Patch protocol

### CRDT Algorithms

LuvJSON implements three CRDT algorithms:

- **Constant**: A degenerate case of the most basic CRDT algorithm, which does not allow any concurrent edits
- **Last-Write-Wins (LWW)**: A CRDT algorithm that resolves conflicts by choosing the update with the highest timestamp
- **Replicated Growable Array (RGA)**: A CRDT algorithm for ordered lists, which allows concurrent insertions and deletions

### Node Types

LuvJSON supports seven node types:

- **con**: A constant value
- **val**: A LWW-Value
- **obj**: A LWW-Object
- **vec**: A LWW-Vector
- **str**: An RGA-String
- **bin**: An RGA-Binary blob
- **arr**: An RGA-Array

### Operation Types

LuvJSON supports four operation types:

- **new**: Creates a new CRDT node
- **ins**: Updates an existing CRDT node
- **del**: Deletes contents from an existing CRDT node
- **nop**: A no-op operation, which does nothing

## License

MIT
