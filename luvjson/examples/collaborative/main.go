package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
	"tictactoe/luvjson/crdtpubsub"
	"tictactoe/luvjson/crdtpubsub/memory"
	"tictactoe/luvjson/tracker"
)

// Document represents a collaborative document.
type Document struct {
	Title    string   `json:"title" crdt:"title"`
	Content  string   `json:"content" crdt:"content"`
	Authors  []string `json:"authors" crdt:"authors"`
	Modified string   `json:"modified" crdt:"modified"`
}

// Client represents a client editing the document.
type Client struct {
	ID         int
	Name       string
	Document   *tracker.TrackableStruct
	Tracker    *tracker.Tracker
	PubSub     crdtpubsub.PubSub
	DocumentID string
}

// Server represents a server that synchronizes clients.
type Server struct {
	Document   *Document
	Tracker    *tracker.Tracker
	PubSub     crdtpubsub.PubSub
	DocumentID string
	QueueMutex sync.Mutex
	PatchQueue [][]byte
}

// NewServer creates a new server with a PubSub system.
func NewServer(pubsub crdtpubsub.PubSub) *Server {
	// Create a new CRDT document for the server
	sid := common.NewSessionID()
	doc := crdt.NewDocument(sid)

	// Create a tracker for the document
	t := tracker.NewTracker(doc, sid)

	// Initialize the server document
	serverDoc := &Document{
		Title:    "Collaborative Document",
		Content:  "This is a collaborative document.",
		Authors:  []string{"Server"},
		Modified: time.Now().Format(time.RFC3339),
	}

	// Create a trackable struct for the server document
	trackable, err := tracker.NewTrackableStruct(serverDoc, sid)
	if err != nil {
		log.Fatalf("Failed to create trackable struct: %v", err)
	}

	// Initialize the document with the trackable struct
	if err := t.Track(trackable); err != nil {
		log.Fatalf("Failed to track document: %v", err)
	}

	// Generate a unique document ID
	documentID := fmt.Sprintf("doc-%s", sid.String())

	return &Server{
		Document:   serverDoc,
		Tracker:    t,
		PubSub:     pubsub,
		DocumentID: documentID,
		PatchQueue: make([][]byte, 0),
	}
}

// InitializeClient initializes a client with the server's document.
func (s *Server) InitializeClient(client *Client) {
	// Initialize the client's document with the server's document
	clientDoc := &Document{
		Title:    s.Document.Title,
		Content:  s.Document.Content,
		Authors:  make([]string, len(s.Document.Authors)),
		Modified: s.Document.Modified,
	}
	copy(clientDoc.Authors, s.Document.Authors)

	// Create a trackable struct for the client document
	trackable, err := tracker.NewTrackableStruct(clientDoc, client.Tracker.GetSessionID())
	if err != nil {
		log.Printf("Failed to create trackable struct for client %d: %v", client.ID, err)
		return
	}

	// Initialize the client's document with the trackable struct
	if err := client.Tracker.Track(trackable); err != nil {
		log.Printf("Failed to track document for client %d: %v", client.ID, err)
		return
	}

	client.Document = trackable

	fmt.Printf("Client %d (%s) connected to server\n", client.ID, client.Name)
}

// ProcessPatch processes a patch received from a client.
func (s *Server) ProcessPatch(patch []byte) {
	s.QueueMutex.Lock()
	s.PatchQueue = append(s.PatchQueue, patch)
	s.QueueMutex.Unlock()

	// Print the patch for debugging
	fmt.Printf("Server received patch: %s\n", string(patch))

	// Apply the patch to the server's document
	if err := tracker.ApplyJSONCRDTPatch(s.Document, patch, s.Tracker.GetSessionID()); err != nil {
		log.Printf("Server failed to apply patch: %v", err)
	} else {
		fmt.Println("Server applied patch")

		// Print the server document after applying the patch
		fmt.Printf("Server document after applying patch: %+v\n", s.Document)
	}

	// Broadcast the patch to all clients
	ctx := context.Background()
	topic := fmt.Sprintf("%s-updates", s.DocumentID)
	if err := s.PubSub.PublishRaw(ctx, topic, patch, crdtpubsub.EncodingFormatJSON); err != nil {
		log.Printf("Failed to broadcast patch: %v", err)
	}
}

// Start starts the server.
func (s *Server) Start(ctx context.Context) {
	// Subscribe to client patches
	topic := fmt.Sprintf("%s-patches", s.DocumentID)
	if err := s.PubSub.Subscribe(ctx, topic, "server", func(ctx context.Context, topic string, data []byte, format crdtpubsub.EncodingFormat) error {
		// Process the patch
		s.ProcessPatch(data)
		return nil
	}); err != nil {
		log.Printf("Failed to subscribe to client patches: %v", err)
	}

	fmt.Println("Server started")
}

// NewClient creates a new client with a PubSub system.
func NewClient(id int, name string, pubsub crdtpubsub.PubSub, documentID string) *Client {
	// Create a new CRDT document for the client
	sid := common.NewSessionID()
	doc := crdt.NewDocument(sid)

	// Create a tracker for the document
	t := tracker.NewTracker(doc, sid)

	return &Client{
		ID:         id,
		Name:       name,
		Tracker:    t,
		PubSub:     pubsub,
		DocumentID: documentID,
	}
}

// EditDocument edits the document.
func (c *Client) EditDocument(doc *Document) {
	// Get the current document
	currentDoc := c.Document.GetData().(*Document)

	// Print the document before update
	fmt.Printf("Client %d document before update: %+v\n", c.ID, currentDoc)

	// Update the document and get the patch
	patch, err := tracker.GenerateJSONCRDTPatch(currentDoc, doc, c.Tracker.GetSessionID())
	if err != nil {
		log.Printf("Client %d failed to generate patch: %v", c.ID, err)
		return
	}

	// Apply the patch to the client's document
	if err := tracker.ApplyJSONCRDTPatch(currentDoc, patch, c.Tracker.GetSessionID()); err != nil {
		log.Printf("Client %d failed to apply patch: %v", c.ID, err)
		return
	}

	// Print the document after update
	fmt.Printf("Client %d document after update: %+v\n", c.ID, currentDoc)

	// Print the patch for debugging
	fmt.Printf("Client %d patch: %s\n", c.ID, string(patch))

	// Send the patch to the server via PubSub
	ctx := context.Background()
	topic := fmt.Sprintf("%s-patches", c.DocumentID)
	if err := c.PubSub.PublishRaw(ctx, topic, patch, crdtpubsub.EncodingFormatJSON); err != nil {
		log.Printf("Client %d failed to send patch: %v", c.ID, err)
		fmt.Printf("Client %d (%s) edited the document but failed to send the patch\n", c.ID, c.Name)
	} else {
		fmt.Printf("Client %d (%s) edited the document and sent a patch to the server\n", c.ID, c.Name)
	}
}

// GetDocument gets the document.
func (c *Client) GetDocument() *Document {
	return c.Document.GetData().(*Document)
}

func main() {
	// Create a context
	ctx := context.Background()

	// Create a shared PubSub system
	pubsub, err := memory.NewPubSub()
	if err != nil {
		log.Fatalf("Failed to create PubSub: %v", err)
	}
	defer pubsub.Close()

	// Create a server
	server := NewServer(pubsub)

	// Start the server
	server.Start(ctx)

	// Create clients
	client1 := NewClient(1, "Alice", pubsub, server.DocumentID)
	client2 := NewClient(2, "Bob", pubsub, server.DocumentID)
	client3 := NewClient(3, "Charlie", pubsub, server.DocumentID)

	// Initialize clients with the server's document
	server.InitializeClient(client1)
	server.InitializeClient(client2)
	server.InitializeClient(client3)

	// Subscribe clients to server updates
	subscribeTopic := fmt.Sprintf("%s-updates", server.DocumentID)
	for _, client := range []*Client{client1, client2, client3} {
		subscriberID := fmt.Sprintf("client-%d", client.ID)
		if err := pubsub.Subscribe(ctx, subscribeTopic, subscriberID, func(ctx context.Context, topic string, data []byte, format crdtpubsub.EncodingFormat) error {
			// Apply the patch to the client's document
			clientDoc := client.Document.GetData().(*Document)
			if err := tracker.ApplyJSONCRDTPatch(clientDoc, data, client.Tracker.GetSessionID()); err != nil {
				log.Printf("Client %d failed to apply patch: %v", client.ID, err)
			} else {
				fmt.Printf("Client %d received update\n", client.ID)
				fmt.Printf("Client %d document after applying patch: %+v\n", client.ID, clientDoc)
			}
			return nil
		}); err != nil {
			log.Printf("Failed to subscribe client %d: %v", client.ID, err)
		}
	}

	// Wait for a moment to ensure all clients are initialized
	time.Sleep(100 * time.Millisecond)

	// Create a custom EditField function to edit only specific fields
	editField := func(client *Client, fieldName string, value interface{}) {
		// Get the current document
		doc := client.GetDocument()

		// Create a new document with the updated field
		newDoc := &Document{
			Title:    doc.Title,
			Content:  doc.Content,
			Authors:  make([]string, len(doc.Authors)),
			Modified: time.Now().Format(time.RFC3339),
		}
		copy(newDoc.Authors, doc.Authors)

		// Update only the specified field
		switch fieldName {
		case "title":
			newDoc.Title = value.(string)
		case "content":
			newDoc.Content = value.(string)
		case "authors":
			newDoc.Authors = value.([]string)
		}

		// Update the document
		client.EditDocument(newDoc)

		// Wait for changes to propagate
		time.Sleep(500 * time.Millisecond)
	}

	// Simulate clients editing the document
	// Client 1 edits the title only
	editField(client1, "title", "Collaborative CRDT Document")

	// Client 2 edits the content only
	editField(client2, "content", "This is a collaborative document using CRDT.")

	// Client 3 edits the authors only
	editField(client3, "authors", []string{"Server", "Alice", "Bob", "Charlie"})

	// Wait for changes to propagate
	time.Sleep(1 * time.Second)

	// Print the final documents
	fmt.Println("\nFinal Documents:")

	fmt.Println("\nClient 1 (Alice):")
	doc1Final := client1.GetDocument()
	printDocument(doc1Final)

	fmt.Println("\nClient 2 (Bob):")
	doc2Final := client2.GetDocument()
	printDocument(doc2Final)

	fmt.Println("\nClient 3 (Charlie):")
	doc3Final := client3.GetDocument()
	printDocument(doc3Final)

	fmt.Println("\nServer:")
	printDocument(server.Document)
}

// printDocument prints a document.
func printDocument(doc *Document) {
	fmt.Printf("Title: %s\n", doc.Title)
	fmt.Printf("Content: %s\n", doc.Content)
	fmt.Printf("Authors: %v\n", doc.Authors)
	fmt.Printf("Modified: %s\n", doc.Modified)
}
