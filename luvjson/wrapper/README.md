# LuvJSON Wrapper

The LuvJSON Wrapper package provides a high-level API for working with JSON CRDT documents using Go structs. It allows you to use the power of CRDTs without having to understand the underlying CRDT operations.

## Features

- **Struct-based API**: Work with Go structs instead of raw CRDT operations
- **Automatic CRDT Operations**: Automatically generate CRDT operations from struct changes
- **Struct Tags**: Use struct tags to control how fields are mapped to CRDT nodes
- **Nested Structs**: Support for nested structs and arrays
- **Conflict Resolution**: Automatic conflict resolution using CRDT semantics

## Usage

### Basic Usage

```go
// Create a new CRDT document
doc := wrapper.NewCRDTDocument(1)

// Define a struct
type User struct {
    Name    string   `json:"name" crdt:"name"`
    Age     int      `json:"age" crdt:"age"`
    Email   string   `json:"email" crdt:"email"`
    Tags    []string `json:"tags" crdt:"tags"`
}

// Create a user
user := User{
    Name:  "John Doe",
    Age:   30,
    Email: "john@example.com",
    Tags:  []string{"developer", "golang"},
}

// Initialize the document from the user struct
if err := doc.FromStruct(user); err != nil {
    log.Fatalf("Failed to initialize document: %v", err)
}

// Get the user from the document
var retrievedUser User
if err := doc.ToStruct(&retrievedUser); err != nil {
    log.Fatalf("Failed to get user from document: %v", err)
}

// Update a field
if err := doc.UpdateField("age", 31); err != nil {
    log.Fatalf("Failed to update field: %v", err)
}

// Update the user struct
user.Email = "john.doe@example.com"

// Update the document from the user struct
if err := doc.UpdateStruct(user); err != nil {
    log.Fatalf("Failed to update document: %v", err)
}
```

### Struct Tags

You can use struct tags to control how fields are mapped to CRDT nodes:

```go
type User struct {
    Name    string `crdt:"name"`           // Use "name" as the field name
    Age     int    `crdt:"age"`            // Use "age" as the field name
    Email   string `crdt:"email"`          // Use "email" as the field name
    Secret  string `crdt:"secret,ignore"`  // Ignore this field
    ReadOnly string `crdt:"readonly,readonly"` // Read-only field
}
```

If no `crdt` tag is provided, the wrapper will use the `json` tag or the field name.

### Nested Structs

The wrapper supports nested structs:

```go
type Address struct {
    Street  string `crdt:"street"`
    City    string `crdt:"city"`
    Country string `crdt:"country"`
}

type User struct {
    Name    string  `crdt:"name"`
    Address Address `crdt:"address"`
}
```

### Arrays and Slices

The wrapper supports arrays and slices:

```go
type User struct {
    Name  string   `crdt:"name"`
    Tags  []string `crdt:"tags"`
}
```

## Advanced Features

### Nested Field Updates

You can update nested fields using dot notation:

```go
if err := doc.UpdateNestedField("address.city", "New York"); err != nil {
    log.Fatalf("Failed to update nested field: %v", err)
}
```

### Creating Arrays and Objects

You can create arrays and objects:

```go
// Create an array
if err := doc.CreateArray("tags"); err != nil {
    log.Fatalf("Failed to create array: %v", err)
}

// Append to an array
if err := doc.AppendToArray("tags", "developer"); err != nil {
    log.Fatalf("Failed to append to array: %v", err)
}

// Create an object
if err := doc.CreateObject("address"); err != nil {
    log.Fatalf("Failed to create object: %v", err)
}
```

### Deleting Fields

You can delete fields:

```go
if err := doc.DeleteField("email"); err != nil {
    log.Fatalf("Failed to delete field: %v", err)
}
```

### Getting Field Values

You can get field values:

```go
value, err := doc.GetFieldValue("age")
if err != nil {
    log.Fatalf("Failed to get field value: %v", err)
}
fmt.Printf("Age: %v\n", value)
```

## Collaborative Editing

The wrapper can be used to build collaborative editing applications:

```go
// Create a server document
serverDoc := wrapper.NewCRDTDocument(0)

// Create client documents
client1Doc := wrapper.NewCRDTDocument(1)
client2Doc := wrapper.NewCRDTDocument(2)

// Initialize documents
initialData := MyStruct{...}
serverDoc.FromStruct(initialData)
client1Doc.FromStruct(initialData)
client2Doc.FromStruct(initialData)

// Client 1 makes changes
client1Data := MyStruct{...}
client1Doc.UpdateStruct(client1Data)

// Generate a patch from client 1
patch1, err := client1Doc.GetPatch(common.EncodingFormatVerbose)
if err != nil {
    log.Fatalf("Failed to get patch: %v", err)
}

// Apply the patch to the server
serverDoc.ApplyPatch(patch1, common.EncodingFormatVerbose)

// Client 2 makes changes
client2Data := MyStruct{...}
client2Doc.UpdateStruct(client2Data)

// Generate a patch from client 2
patch2, err := client2Doc.GetPatch(common.EncodingFormatVerbose)
if err != nil {
    log.Fatalf("Failed to get patch: %v", err)
}

// Apply the patch to the server
serverDoc.ApplyPatch(patch2, common.EncodingFormatVerbose)

// Broadcast patches to all clients
client1Doc.ApplyPatch(patch2, common.EncodingFormatVerbose)
client2Doc.ApplyPatch(patch1, common.EncodingFormatVerbose)
```

## Limitations

- The wrapper currently only supports a subset of CRDT operations
- Some advanced features like watching for changes are not implemented
- The wrapper does not handle network communication or synchronization
- The wrapper does not handle concurrent updates to the same field
