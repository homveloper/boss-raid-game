// Package crdtedit provides a user-friendly API for manipulating CRDT documents.
//
// The crdtedit package abstracts the complexity of CRDT operations and provides
// a simple, intuitive interface for working with CRDT documents. It allows users
// to manipulate CRDT documents as if they were regular Go data structures like
// maps, arrays, and primitive values.
//
// Key features:
// - Path-based access to document nodes
// - Type-specific editors for different node types
// - Structured editing with automatic operation generation
// - Support for initializing documents from structs or JSON
// - Query capabilities for retrieving document data
package crdtedit
