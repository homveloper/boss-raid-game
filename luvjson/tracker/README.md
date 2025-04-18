# LuvJSON Tracker

The LuvJSON Tracker package provides functionality for tracking changes to Go structs and automatically generating CRDT patches. It allows you to detect and track state changes in a CRDT Document by comparing before and after struct states.

## Features

- **Automatic CRDT Patch Generation**: Automatically generate CRDT patches by comparing before and after struct states
- **Struct Tracking**: Track changes to structs over time
- **Integration with CRDT Document**: Apply generated patches to CRDT documents
- **Utility Functions**: Helper functions for comparing and cloning structs

## Usage

### Basic Usage

```go
// Create a CRDT document
doc := crdt.NewDocument(common.NewSessionID())

// Create a tracker
tracker := tracker.NewTracker(doc, 1)

// Create a struct to track
person := &Person{
    Name: "John Doe",
    Age:  30,
}

// Start tracking the struct
if err := tracker.Track(person); err != nil {
    log.Fatalf("Failed to track person: %v", err)
}

// Update the struct
person.Name = "Jane Doe"
person.Age = 31

// Generate a patch from the changes
patch, err := tracker.Update(person)
if err != nil {
    log.Fatalf("Failed to update tracker: %v", err)
}

// Apply the patch to the document
if err := tracker.ApplyPatch(patch); err != nil {
    log.Fatalf("Failed to apply patch: %v", err)
}
```

### Using TrackableStruct

```go
// Create a struct to track
person := &Person{
    Name: "John Doe",
    Age:  30,
}

// Create a trackable struct
trackable, err := tracker.NewTrackableStruct(person, 1)
if err != nil {
    log.Fatalf("Failed to create trackable struct: %v", err)
}

// Update the struct
person.Name = "Jane Doe"
person.Age = 31

// Generate a patch from the changes
patch, err := trackable.Update()
if err != nil {
    log.Fatalf("Failed to update trackable struct: %v", err)
}
```

### Generating and Applying Patches

```go
// Create two structs
person1 := &Person{
    Name: "John Doe",
    Age:  30,
}

person2 := &Person{
    Name: "Jane Doe",
    Age:  31,
}

// Generate a patch from the differences
patchData, err := tracker.GenerateJSONCRDTPatch(person1, person2, 1)
if err != nil {
    log.Fatalf("Failed to generate patch: %v", err)
}

// Apply the patch to a copy of person1
person1Copy := &Person{}
if err := tracker.CloneStruct(person1, person1Copy); err != nil {
    log.Fatalf("Failed to clone person1: %v", err)
}

if err := tracker.ApplyJSONCRDTPatch(person1Copy, patchData, 1); err != nil {
    log.Fatalf("Failed to apply patch: %v", err)
}
```

## Advanced Usage

### Initializing a Document with a Struct

```go
// Create a CRDT document
doc := crdt.NewDocument(common.NewSessionID())

// Create a tracker
tracker := tracker.NewTracker(doc, 1)

// Create a struct to initialize the document with
person := &Person{
    Name: "John Doe",
    Age:  30,
}

// Initialize the document with the struct
if err := tracker.InitializeDocument(person); err != nil {
    log.Fatalf("Failed to initialize document: %v", err)
}

// Update the struct
person.Name = "Jane Doe"
person.Age = 31

// Generate a patch from the changes
patch, err := tracker.Update(person)
if err != nil {
    log.Fatalf("Failed to update tracker: %v", err)
}

// Apply the patch to the document
if err := tracker.ApplyPatch(patch); err != nil {
    log.Fatalf("Failed to apply patch: %v", err)
}

// Get the current view of the document
view, err := tracker.GetView()
if err != nil {
    log.Fatalf("Failed to get document view: %v", err)
}

// Convert the view to a struct
var result Person
if err := tracker.ToStruct(&result); err != nil {
    log.Fatalf("Failed to convert view to struct: %v", err)
}
```
