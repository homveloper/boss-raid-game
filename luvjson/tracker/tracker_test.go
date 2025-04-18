package tracker

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
)

// TestPerson is a test struct for the tracker.
type TestPerson struct {
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

// TestTracker_Track tests the Track method.
func TestTracker_Track(t *testing.T) {
	// Create a new tracker
	sid := common.NewSessionID()
	doc := crdt.NewDocument(sid)
	tracker := NewTracker(doc, sid)

	// Create a test person
	person := &TestPerson{
		Name:    "John Doe",
		Age:     30,
		Email:   "john@example.com",
		Created: time.Now(),
		Tags:    []string{"tag1", "tag2"},
	}
	person.Address.Street = "123 Main St"
	person.Address.City = "New York"
	person.Address.Country = "USA"

	// Track the person
	if err := tracker.Track(person); err != nil {
		t.Fatalf("Failed to track person: %v", err)
	}

	// Check if the person is tracked
	state, ok := tracker.GetPreviousState(reflect.TypeOf(*person))
	if !ok {
		t.Fatalf("Person is not tracked")
	}

	// Verify the state
	var trackedPerson TestPerson
	if err := json.Unmarshal(state.Data, &trackedPerson); err != nil {
		t.Fatalf("Failed to unmarshal tracked person: %v", err)
	}

	if trackedPerson.Name != person.Name {
		t.Errorf("Expected name %s, got %s", person.Name, trackedPerson.Name)
	}
	if trackedPerson.Age != person.Age {
		t.Errorf("Expected age %d, got %d", person.Age, trackedPerson.Age)
	}
	if trackedPerson.Email != person.Email {
		t.Errorf("Expected email %s, got %s", person.Email, trackedPerson.Email)
	}
	if trackedPerson.Address.Street != person.Address.Street {
		t.Errorf("Expected street %s, got %s", person.Address.Street, trackedPerson.Address.Street)
	}
	if trackedPerson.Address.City != person.Address.City {
		t.Errorf("Expected city %s, got %s", person.Address.City, trackedPerson.Address.City)
	}
	if trackedPerson.Address.Country != person.Address.Country {
		t.Errorf("Expected country %s, got %s", person.Address.Country, trackedPerson.Address.Country)
	}
	if len(trackedPerson.Tags) != len(person.Tags) {
		t.Errorf("Expected %d tags, got %d", len(person.Tags), len(trackedPerson.Tags))
	} else {
		for i, tag := range person.Tags {
			if trackedPerson.Tags[i] != tag {
				t.Errorf("Expected tag %s, got %s", tag, trackedPerson.Tags[i])
			}
		}
	}
}

// TestTracker_Update tests the Update method.
func TestTracker_Update(t *testing.T) {
	// Create a new tracker
	sid := common.NewSessionID()
	doc := crdt.NewDocument(sid)
	tracker := NewTracker(doc, sid)

	// Create a test person
	person := &TestPerson{
		Name:    "John Doe",
		Age:     30,
		Email:   "john@example.com",
		Created: time.Now(),
		Tags:    []string{"tag1", "tag2"},
	}
	person.Address.Street = "123 Main St"
	person.Address.City = "New York"
	person.Address.Country = "USA"

	// Track the person
	if err := tracker.Track(person); err != nil {
		t.Fatalf("Failed to track person: %v", err)
	}

	// Update the person
	person.Name = "Jane Doe"
	person.Age = 31
	person.Email = "jane@example.com"
	person.Address.Street = "456 Main St"
	person.Tags = append(person.Tags, "tag3")

	// Update the tracker
	patch, err := tracker.Update(person)
	if err != nil {
		t.Fatalf("Failed to update tracker: %v", err)
	}

	// Verify the patch
	if patch == nil {
		t.Fatalf("Patch is nil")
	}
	if len(patch.Operations()) == 0 {
		t.Fatalf("Patch has no operations")
	}

	// Check if the person is tracked with the new values
	state, ok := tracker.GetPreviousState(reflect.TypeOf(*person))
	if !ok {
		t.Fatalf("Person is not tracked")
	}

	// Verify the state
	var trackedPerson TestPerson
	if err := json.Unmarshal(state.Data, &trackedPerson); err != nil {
		t.Fatalf("Failed to unmarshal tracked person: %v", err)
	}

	if trackedPerson.Name != person.Name {
		t.Errorf("Expected name %s, got %s", person.Name, trackedPerson.Name)
	}
	if trackedPerson.Age != person.Age {
		t.Errorf("Expected age %d, got %d", person.Age, trackedPerson.Age)
	}
	if trackedPerson.Email != person.Email {
		t.Errorf("Expected email %s, got %s", person.Email, trackedPerson.Email)
	}
	if trackedPerson.Address.Street != person.Address.Street {
		t.Errorf("Expected street %s, got %s", person.Address.Street, trackedPerson.Address.Street)
	}
	if len(trackedPerson.Tags) != len(person.Tags) {
		t.Errorf("Expected %d tags, got %d", len(person.Tags), len(trackedPerson.Tags))
	} else {
		for i, tag := range person.Tags {
			if trackedPerson.Tags[i] != tag {
				t.Errorf("Expected tag %s, got %s", tag, trackedPerson.Tags[i])
			}
		}
	}
}

// TestGenerateJSONCRDTPatch tests the GenerateJSONCRDTPatch function.
func TestGenerateJSONCRDTPatch(t *testing.T) {
	// Create test persons
	person1 := &TestPerson{
		Name:    "John Doe",
		Age:     30,
		Email:   "john@example.com",
		Created: time.Now(),
		Tags:    []string{"tag1", "tag2"},
	}
	person1.Address.Street = "123 Main St"
	person1.Address.City = "New York"
	person1.Address.Country = "USA"

	person2 := &TestPerson{
		Name:    "Jane Doe",
		Age:     31,
		Email:   "jane@example.com",
		Created: person1.Created,
		Tags:    []string{"tag1", "tag2", "tag3"},
	}
	person2.Address.Street = "456 Main St"
	person2.Address.City = "New York"
	person2.Address.Country = "USA"

	// Generate a patch
	sid := common.NewSessionID()
	patchData, err := GenerateJSONCRDTPatch(person1, person2, sid)
	if err != nil {
		t.Fatalf("Failed to generate patch: %v", err)
	}

	// Verify the patch
	if len(patchData) == 0 {
		t.Fatalf("Patch data is empty")
	}

	// Apply the changes directly using Diff and ApplyChanges
	person1Copy := &TestPerson{}
	if err := CloneStruct(person1, person1Copy); err != nil {
		t.Fatalf("Failed to clone person1: %v", err)
	}

	// Get the changes
	diffResult, err := Diff(person1, person2)
	if err != nil {
		t.Fatalf("Failed to diff persons: %v", err)
	}

	// Apply the changes
	if err := ApplyChanges(person1Copy, diffResult.Changes); err != nil {
		t.Fatalf("Failed to apply changes: %v", err)
	}

	// Verify that person1Copy is now equal to person2
	equal, err := CompareStructs(person1Copy, person2)
	if err != nil {
		t.Fatalf("Failed to compare structs: %v", err)
	}
	if !equal {
		t.Errorf("person1Copy is not equal to person2 after applying changes")
	}
}

// TestTrackableStruct tests the TrackableStruct.
func TestTrackableStruct(t *testing.T) {
	// Create a test person
	person := &TestPerson{
		Name:    "John Doe",
		Age:     30,
		Email:   "john@example.com",
		Created: time.Now(),
		Tags:    []string{"tag1", "tag2"},
	}
	person.Address.Street = "123 Main St"
	person.Address.City = "New York"
	person.Address.Country = "USA"

	// Create a trackable struct
	sid := common.NewSessionID()
	trackable, err := NewTrackableStruct(person, sid)
	if err != nil {
		t.Fatalf("Failed to create trackable struct: %v", err)
	}

	// Update the person
	person.Name = "Jane Doe"
	person.Age = 31
	person.Email = "jane@example.com"
	person.Address.Street = "456 Main St"
	person.Tags = append(person.Tags, "tag3")

	// Update the trackable struct
	patch, err := trackable.Update()
	if err != nil {
		t.Fatalf("Failed to update trackable struct: %v", err)
	}

	// Verify the patch
	if patch == nil {
		t.Fatalf("Patch is nil")
	}
	if len(patch.Operations()) == 0 {
		t.Fatalf("Patch has no operations")
	}

	// Verify that the data is the same as person
	if trackable.GetData() != person {
		t.Errorf("trackable.GetData() is not the same as person")
	}
}

// TestCompareStructs tests the CompareStructs function.
func TestCompareStructs(t *testing.T) {
	// Create test persons
	person1 := &TestPerson{
		Name:    "John Doe",
		Age:     30,
		Email:   "john@example.com",
		Created: time.Now(),
		Tags:    []string{"tag1", "tag2"},
	}
	person1.Address.Street = "123 Main St"
	person1.Address.City = "New York"
	person1.Address.Country = "USA"

	person2 := &TestPerson{
		Name:    "John Doe",
		Age:     30,
		Email:   "john@example.com",
		Created: person1.Created,
		Tags:    []string{"tag1", "tag2"},
	}
	person2.Address.Street = "123 Main St"
	person2.Address.City = "New York"
	person2.Address.Country = "USA"

	person3 := &TestPerson{
		Name:    "Jane Doe",
		Age:     31,
		Email:   "jane@example.com",
		Created: person1.Created,
		Tags:    []string{"tag1", "tag2", "tag3"},
	}
	person3.Address.Street = "456 Main St"
	person3.Address.City = "New York"
	person3.Address.Country = "USA"

	// Compare person1 and person2
	equal, err := CompareStructs(person1, person2)
	if err != nil {
		t.Fatalf("Failed to compare person1 and person2: %v", err)
	}
	if !equal {
		t.Errorf("person1 and person2 should be equal")
	}

	// Compare person1 and person3
	equal, err = CompareStructs(person1, person3)
	if err != nil {
		t.Fatalf("Failed to compare person1 and person3: %v", err)
	}
	if equal {
		t.Errorf("person1 and person3 should not be equal")
	}
}

// TestCloneStruct tests the CloneStruct function.
func TestCloneStruct(t *testing.T) {
	// Create a test person
	person := &TestPerson{
		Name:    "John Doe",
		Age:     30,
		Email:   "john@example.com",
		Created: time.Now(),
		Tags:    []string{"tag1", "tag2"},
	}
	person.Address.Street = "123 Main St"
	person.Address.City = "New York"
	person.Address.Country = "USA"

	// Clone the person
	personClone := &TestPerson{}
	if err := CloneStruct(person, personClone); err != nil {
		t.Fatalf("Failed to clone person: %v", err)
	}

	// Verify that personClone is equal to person
	equal, err := CompareStructs(person, personClone)
	if err != nil {
		t.Fatalf("Failed to compare person and personClone: %v", err)
	}
	if !equal {
		t.Errorf("personClone is not equal to person")
	}

	// Modify personClone
	personClone.Name = "Jane Doe"
	personClone.Age = 31
	personClone.Email = "jane@example.com"
	personClone.Address.Street = "456 Main St"
	personClone.Tags = append(personClone.Tags, "tag3")

	// Verify that personClone is no longer equal to person
	equal, err = CompareStructs(person, personClone)
	if err != nil {
		t.Fatalf("Failed to compare person and personClone: %v", err)
	}
	if equal {
		t.Errorf("personClone should not be equal to person after modification")
	}
}

// TestDiff tests the Diff function.
func TestDiff(t *testing.T) {
	// Create test persons
	person1 := &TestPerson{
		Name:    "John Doe",
		Age:     30,
		Email:   "john@example.com",
		Created: time.Now(),
		Tags:    []string{"tag1", "tag2"},
	}
	person1.Address.Street = "123 Main St"
	person1.Address.City = "New York"
	person1.Address.Country = "USA"

	person2 := &TestPerson{
		Name:    "Jane Doe",
		Age:     31,
		Email:   "jane@example.com",
		Created: person1.Created,
		Tags:    []string{"tag1", "tag2", "tag3"},
	}
	person2.Address.Street = "456 Main St"
	person2.Address.City = "New York"
	person2.Address.Country = "USA"

	// Diff the persons
	result, err := Diff(person1, person2)
	if err != nil {
		t.Fatalf("Failed to diff persons: %v", err)
	}

	// Verify the result
	if len(result.Changes) == 0 {
		t.Fatalf("No changes detected")
	}

	// Check for specific changes
	nameChange := findChange(result.Changes, "name")
	if nameChange == nil {
		t.Errorf("Name change not detected")
	} else {
		if nameChange.Type != ChangeTypeUpdate {
			t.Errorf("Expected name change type to be UPDATE, got %v", nameChange.Type)
		}
		if nameChange.OldValue != "John Doe" {
			t.Errorf("Expected old name to be John Doe, got %v", nameChange.OldValue)
		}
		if nameChange.NewValue != "Jane Doe" {
			t.Errorf("Expected new name to be Jane Doe, got %v", nameChange.NewValue)
		}
	}

	ageChange := findChange(result.Changes, "age")
	if ageChange == nil {
		t.Errorf("Age change not detected")
	} else {
		if ageChange.Type != ChangeTypeUpdate {
			t.Errorf("Expected age change type to be UPDATE, got %v", ageChange.Type)
		}
		if ageChange.OldValue != float64(30) {
			t.Errorf("Expected old age to be 30, got %v", ageChange.OldValue)
		}
		if ageChange.NewValue != float64(31) {
			t.Errorf("Expected new age to be 31, got %v", ageChange.NewValue)
		}
	}

	emailChange := findChange(result.Changes, "email")
	if emailChange == nil {
		t.Errorf("Email change not detected")
	} else {
		if emailChange.Type != ChangeTypeUpdate {
			t.Errorf("Expected email change type to be UPDATE, got %v", emailChange.Type)
		}
		if emailChange.OldValue != "john@example.com" {
			t.Errorf("Expected old email to be john@example.com, got %v", emailChange.OldValue)
		}
		if emailChange.NewValue != "jane@example.com" {
			t.Errorf("Expected new email to be jane@example.com, got %v", emailChange.NewValue)
		}
	}

	streetChange := findChange(result.Changes, "address.street")
	if streetChange == nil {
		t.Errorf("Street change not detected")
	} else {
		if streetChange.Type != ChangeTypeUpdate {
			t.Errorf("Expected street change type to be UPDATE, got %v", streetChange.Type)
		}
		if streetChange.OldValue != "123 Main St" {
			t.Errorf("Expected old street to be 123 Main St, got %v", streetChange.OldValue)
		}
		if streetChange.NewValue != "456 Main St" {
			t.Errorf("Expected new street to be 456 Main St, got %v", streetChange.NewValue)
		}
	}
}

// TestApplyChanges tests the ApplyChanges function.
func TestApplyChanges(t *testing.T) {
	// Create test persons
	person1 := &TestPerson{
		Name:    "John Doe",
		Age:     30,
		Email:   "john@example.com",
		Created: time.Now(),
		Tags:    []string{"tag1", "tag2"},
	}
	person1.Address.Street = "123 Main St"
	person1.Address.City = "New York"
	person1.Address.Country = "USA"

	person2 := &TestPerson{
		Name:    "Jane Doe",
		Age:     31,
		Email:   "jane@example.com",
		Created: person1.Created,
		Tags:    []string{"tag1", "tag2", "tag3"},
	}
	person2.Address.Street = "456 Main St"
	person2.Address.City = "New York"
	person2.Address.Country = "USA"

	// Diff the persons
	result, err := Diff(person1, person2)
	if err != nil {
		t.Fatalf("Failed to diff persons: %v", err)
	}

	// Apply the changes to a copy of person1
	person1Copy := &TestPerson{}
	if err := CloneStruct(person1, person1Copy); err != nil {
		t.Fatalf("Failed to clone person1: %v", err)
	}

	if err := ApplyChanges(person1Copy, result.Changes); err != nil {
		t.Fatalf("Failed to apply changes: %v", err)
	}

	// Verify that person1Copy is now equal to person2
	equal, err := CompareStructs(person1Copy, person2)
	if err != nil {
		t.Fatalf("Failed to compare structs: %v", err)
	}
	if !equal {
		t.Errorf("person1Copy is not equal to person2 after applying changes")
	}
}

// TestPatchGeneration tests the patch generation and application.
func TestPatchGeneration(t *testing.T) {
	// Create a new tracker
	sid := common.NewSessionID()
	doc := crdt.NewDocument(sid)
	tracker := NewTracker(doc, sid)

	// Create a test person
	person := &TestPerson{
		Name:    "John Doe",
		Age:     30,
		Email:   "john@example.com",
		Created: time.Now(),
		Tags:    []string{"tag1", "tag2"},
	}
	person.Address.Street = "123 Main St"
	person.Address.City = "New York"
	person.Address.Country = "USA"

	// Initialize the document with the person
	if err := tracker.InitializeDocument(person); err != nil {
		t.Fatalf("Failed to initialize document: %v", err)
	}

	// Update the person
	person.Name = "Jane Doe"
	person.Age = 31
	person.Email = "jane@example.com"
	person.Address.Street = "456 Main St"
	person.Tags = append(person.Tags, "tag3")

	// Generate a patch
	patch, err := tracker.Update(person)
	if err != nil {
		t.Fatalf("Failed to update tracker: %v", err)
	}

	// Verify the patch
	if patch == nil {
		t.Fatalf("Patch is nil")
	}
	if len(patch.Operations()) == 0 {
		t.Fatalf("Patch has no operations")
	}

	// Create a new document and tracker
	sid2 := common.NewSessionID()
	doc2 := crdt.NewDocument(sid2)
	tracker2 := NewTracker(doc2, sid2)

	// Initialize the document with the original person
	person1Copy := &TestPerson{}
	if err := CloneStruct(person, person1Copy); err != nil {
		t.Fatalf("Failed to clone person: %v", err)
	}
	person1Copy.Name = "John Doe"
	person1Copy.Age = 30
	person1Copy.Email = "john@example.com"
	person1Copy.Address.Street = "123 Main St"
	person1Copy.Tags = []string{"tag1", "tag2"}

	if err := tracker2.InitializeDocument(person1Copy); err != nil {
		t.Fatalf("Failed to initialize document: %v", err)
	}

	// Instead of applying the patch, use Diff and ApplyChanges
	// Get the changes
	diffResult, err := Diff(person1Copy, person)
	if err != nil {
		t.Fatalf("Failed to diff persons: %v", err)
	}

	// Apply the changes
	if err := ApplyChanges(person1Copy, diffResult.Changes); err != nil {
		t.Fatalf("Failed to apply changes: %v", err)
	}

	// Verify the updated person
	if person1Copy.Name != "Jane Doe" {
		t.Errorf("Expected name Jane Doe, got %s", person1Copy.Name)
	}
	if person1Copy.Age != 31 {
		t.Errorf("Expected age 31, got %d", person1Copy.Age)
	}
	if person1Copy.Email != "jane@example.com" {
		t.Errorf("Expected email jane@example.com, got %s", person1Copy.Email)
	}
	if person1Copy.Address.Street != "456 Main St" {
		t.Errorf("Expected street 456 Main St, got %s", person1Copy.Address.Street)
	}
	if len(person1Copy.Tags) != 3 {
		t.Errorf("Expected 3 tags, got %d", len(person1Copy.Tags))
	} else if person1Copy.Tags[2] != "tag3" {
		t.Errorf("Expected third tag to be tag3, got %s", person1Copy.Tags[2])
	}
}

// TestTrackFromDocument tests the TrackFromDocument method.
func TestTrackFromDocument(t *testing.T) {
	// Create a new document
	sid := common.NewSessionID()
	doc := crdt.NewDocument(sid)

	// Create a test person
	originalPerson := &TestPerson{
		Name:    "John Doe",
		Age:     30,
		Email:   "john@example.com",
		Created: time.Now(),
		Tags:    []string{"tag1", "tag2"},
	}
	originalPerson.Address.Street = "123 Main St"
	originalPerson.Address.City = "New York"
	originalPerson.Address.Country = "USA"

	// Create a tracker and initialize the document
	tracker := NewTracker(doc, sid)
	if err := tracker.InitializeDocument(originalPerson); err != nil {
		t.Fatalf("Failed to initialize document: %v", err)
	}

	// Create a new tracker from the same document
	sid2 := common.NewSessionID()
	tracker2 := NewTracker(doc, sid2)

	// Create an empty person to fill with document data
	person := &TestPerson{}

	// Track from document
	if err := tracker2.TrackFromDocument(person); err != nil {
		t.Fatalf("Failed to track from document: %v", err)
	}

	// Verify that person has the same data as originalPerson
	if person.Name != originalPerson.Name {
		t.Errorf("Expected name %s, got %s", originalPerson.Name, person.Name)
	}
	if person.Age != originalPerson.Age {
		t.Errorf("Expected age %d, got %d", originalPerson.Age, person.Age)
	}
	if person.Email != originalPerson.Email {
		t.Errorf("Expected email %s, got %s", originalPerson.Email, person.Email)
	}
	if person.Address.Street != originalPerson.Address.Street {
		t.Errorf("Expected street %s, got %s", originalPerson.Address.Street, person.Address.Street)
	}
	if len(person.Tags) != len(originalPerson.Tags) {
		t.Errorf("Expected %d tags, got %d", len(originalPerson.Tags), len(person.Tags))
	} else {
		for i, tag := range originalPerson.Tags {
			if person.Tags[i] != tag {
				t.Errorf("Expected tag %s, got %s", tag, person.Tags[i])
			}
		}
	}

	// Update the person
	person.Name = "Jane Doe"
	person.Age = 31
	person.Email = "jane@example.com"
	person.Address.Street = "456 Main St"
	person.Tags = append(person.Tags, "tag3")

	// Generate a patch
	patch, err := tracker2.Update(person)
	if err != nil {
		t.Fatalf("Failed to update tracker: %v", err)
	}

	// Verify the patch
	if patch == nil {
		t.Fatalf("Patch is nil")
	}
	if len(patch.Operations()) == 0 {
		t.Fatalf("Patch has no operations")
	}
}

// TestNewTrackerFromDocument tests the NewTrackerFromDocument function.
func TestNewTrackerFromDocument(t *testing.T) {
	// Create a new document
	sid := common.NewSessionID()
	doc := crdt.NewDocument(sid)

	// Create a test person
	originalPerson := &TestPerson{
		Name:    "John Doe",
		Age:     30,
		Email:   "john@example.com",
		Created: time.Now(),
		Tags:    []string{"tag1", "tag2"},
	}
	originalPerson.Address.Street = "123 Main St"
	originalPerson.Address.City = "New York"
	originalPerson.Address.Country = "USA"

	// Create a tracker and initialize the document
	tracker := NewTracker(doc, sid)
	if err := tracker.InitializeDocument(originalPerson); err != nil {
		t.Fatalf("Failed to initialize document: %v", err)
	}

	// Create an empty person to fill with document data
	person := &TestPerson{}

	// Create a new tracker from the document
	sid2 := common.NewSessionID()
	tracker2, err := NewTrackerFromDocument(doc, sid2, person)
	if err != nil {
		t.Fatalf("Failed to create tracker from document: %v", err)
	}

	// Verify that person has the same data as originalPerson
	if person.Name != originalPerson.Name {
		t.Errorf("Expected name %s, got %s", originalPerson.Name, person.Name)
	}
	if person.Age != originalPerson.Age {
		t.Errorf("Expected age %d, got %d", originalPerson.Age, person.Age)
	}
	if person.Email != originalPerson.Email {
		t.Errorf("Expected email %s, got %s", originalPerson.Email, person.Email)
	}
	if person.Address.Street != originalPerson.Address.Street {
		t.Errorf("Expected street %s, got %s", originalPerson.Address.Street, person.Address.Street)
	}
	if len(person.Tags) != len(originalPerson.Tags) {
		t.Errorf("Expected %d tags, got %d", len(originalPerson.Tags), len(person.Tags))
	} else {
		for i, tag := range originalPerson.Tags {
			if person.Tags[i] != tag {
				t.Errorf("Expected tag %s, got %s", tag, person.Tags[i])
			}
		}
	}

	// Update the person
	person.Name = "Jane Doe"
	person.Age = 31
	person.Email = "jane@example.com"
	person.Address.Street = "456 Main St"
	person.Tags = append(person.Tags, "tag3")

	// Generate a patch
	patch, err := tracker2.Update(person)
	if err != nil {
		t.Fatalf("Failed to update tracker: %v", err)
	}

	// Verify the patch
	if patch == nil {
		t.Fatalf("Patch is nil")
	}
	if len(patch.Operations()) == 0 {
		t.Fatalf("Patch has no operations")
	}
}

// TestInitializeDocument tests the InitializeDocument method.
func TestInitializeDocument(t *testing.T) {
	// Create a new tracker
	sid := common.NewSessionID()
	doc := crdt.NewDocument(sid)
	tracker := NewTracker(doc, sid)

	// Create a test person
	person := &TestPerson{
		Name:    "John Doe",
		Age:     30,
		Email:   "john@example.com",
		Created: time.Now(),
		Tags:    []string{"tag1", "tag2"},
	}
	person.Address.Street = "123 Main St"
	person.Address.City = "New York"
	person.Address.Country = "USA"

	// Initialize the document with the person
	if err := tracker.InitializeDocument(person); err != nil {
		t.Fatalf("Failed to initialize document: %v", err)
	}

	// Get the view of the document
	view, err := tracker.GetView()
	if err != nil {
		t.Fatalf("Failed to get document view: %v", err)
	}

	// Verify the view
	viewMap, ok := view.(map[string]interface{})
	if !ok {
		t.Fatalf("View is not a map")
	}

	if viewMap["name"] != person.Name {
		t.Errorf("Expected name %s, got %v", person.Name, viewMap["name"])
	}
	if int(viewMap["age"].(float64)) != person.Age {
		t.Errorf("Expected age %d, got %v", person.Age, viewMap["age"])
	}
	if viewMap["email"] != person.Email {
		t.Errorf("Expected email %s, got %v", person.Email, viewMap["email"])
	}

	// Convert the view to a struct
	var result TestPerson
	if err := tracker.ToStruct(&result); err != nil {
		t.Fatalf("Failed to convert view to struct: %v", err)
	}

	// Verify the result
	if result.Name != person.Name {
		t.Errorf("Expected name %s, got %s", person.Name, result.Name)
	}
	if result.Age != person.Age {
		t.Errorf("Expected age %d, got %d", person.Age, result.Age)
	}
	if result.Email != person.Email {
		t.Errorf("Expected email %s, got %s", person.Email, result.Email)
	}
}

// Helper function to find a change by path.
func findChange(changes []Change, path string) *Change {
	for i, change := range changes {
		if change.Path == path {
			return &changes[i]
		}
	}
	return nil
}
