# LuvJSON API

The LuvJSON API package provides a high-level API for working with JSON CRDT documents. It is inspired by the json-joy library's Model API and provides a similar interface for manipulating CRDT documents.

## Features

- **High-level API**: Work with CRDT documents using a simple, intuitive API
- **Node-specific APIs**: Specialized APIs for different node types (object, string, value, etc.)
- **Path-based Access**: Access nodes using path expressions
- **Event System**: Register callbacks for various document events
- **Method Chaining**: Chain method calls for concise code

## Usage

### Creating a Model

```go
// Create a new model with a random session ID
model := api.NewModel(common.NewSessionID())
```

### Setting the Root Value

```go
// Set the root value to an object
model.GetApi().Root(map[string]interface{}{
    "counter": 0,
    "text":    "Hello",
})
```

### Manipulating Objects

```go
// Update fields in an object
model.GetApi().Obj([]interface{}{}).Set(map[string]interface{}{
    "counter": 25,
})

// Delete a field
model.GetApi().Obj([]interface{}{}).Del("counter")
```

### Manipulating Strings

```go
// Get a string node
textNode, err := api.ResolveNode(model.GetDocument(), api.Path{api.StringPathElement("text")})
if err != nil {
    log.Fatalf("Failed to resolve text node: %v", err)
}

// Create a string API for the node
strApi := model.GetApi().Wrap(textNode).(*api.StrApi)

// Insert text
strApi.Ins(5, " world!")

// Delete text
strApi.Del(5, 6)
```

### Getting the Document View

```go
// Get the current view of the document
view, err := model.View()
if err != nil {
    log.Fatalf("Failed to get view: %v", err)
}
fmt.Printf("View: %v\n", view)
```

### Flushing Changes

```go
// Flush the changes and get the patch
patch := model.GetApi().Flush()
fmt.Printf("Patch: %v\n", patch)
```

### Registering Event Callbacks

```go
// Register a callback for the onBeforePatch event
model.GetApi().OnBeforePatch(func(patch *crdtpatch.Patch) {
    fmt.Printf("Before applying patch: %v\n", patch)
})

// Register a callback for the onPatch event
model.GetApi().OnPatch(func(patch *crdtpatch.Patch) {
    fmt.Printf("After applying patch: %v\n", patch)
})
```

## Path Expressions

The API supports several ways to specify paths to nodes in the document:

### String Paths

```go
// Parse a string path
path, err := api.ParsePath("/foo/bar")
if err != nil {
    log.Fatalf("Failed to parse path: %v", err)
}
```

### Slice Paths

```go
// Use a string slice
path := []string{"foo", "bar"}

// Use an int slice
path := []int{0, 1}

// Use an interface slice
path := []interface{}{"foo", 0, "bar"}
```

### Resolving Nodes

```go
// Resolve a node at a path
node, err := api.ResolveNode(model.GetDocument(), path)
if err != nil {
    log.Fatalf("Failed to resolve node: %v", err)
}
```

## Advanced Usage

### Creating Node-specific APIs

```go
// Create an API for a specific node
nodeApi := model.GetApi().Wrap(node)

// Use the API based on the node type
switch api := nodeApi.(type) {
case *api.ObjApi:
    api.Set(map[string]interface{}{"foo": "bar"})
case *api.StrApi:
    api.Ins(0, "Hello")
case *api.ValApi:
    api.Set(42)
case *api.ConApi:
    value, _ := api.Get()
    fmt.Printf("Value: %v\n", value)
}
```

### Applying Patches

```go
// Apply a patch to the document
if err := model.ApplyPatch(patch); err != nil {
    log.Fatalf("Failed to apply patch: %v", err)
}
```
