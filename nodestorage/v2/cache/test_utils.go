package cache

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TestDocument is a simple test document type
type TestDocument struct {
	ID   primitive.ObjectID
	Name string
	Age  int
}
