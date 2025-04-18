package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
	"tictactoe/luvjson/crdtpatch"
	"tictactoe/luvjson/tracker"
)

// Document represents a collaborative document.
type Document struct {
	Title    string   `json:"title" crdt:"title"`
	Content  string   `json:"content" crdt:"content"`
	Authors  []string `json:"authors" crdt:"authors"`
	Modified string   `json:"modified" crdt:"modified"`
}

// Message represents a message between client and server.
type Message struct {
	ClientID int
	Patch    []byte
}

// MessageBroker represents a message broker for communication between clients and server.
type MessageBroker struct {
	ClientToServer chan Message
	ServerToClient chan Message
	mutex          sync.Mutex
}

// NewMessageBroker creates a new message broker.
func NewMessageBroker() *MessageBroker {
	return &MessageBroker{
		ClientToServer: make(chan Message, 100),
		ServerToClient: make(chan Message, 100),
	}
}

// Client represents a client editing the document.
type Client struct {
	ID       int
	Name     string
	Document *tracker.TrackableStruct
	Tracker  *tracker.Tracker
	Broker   *MessageBroker
}

// Server represents a server that synchronizes clients.
type Server struct {
	Clients    map[int]*ClientInfo
	Document   *Document
	Tracker    *tracker.Tracker
	PatchQueue [][]byte
	QueueMutex sync.Mutex
	Broker     *MessageBroker
	DocumentID string
}

// ClientInfo represents information about a connected client.
type ClientInfo struct {
	ID   int
	Name string
}

// NewServer creates a new server.
func NewServer(broker *MessageBroker) *Server {
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

	// Initialize the document with the tracker
	if err := t.InitializeDocument(serverDoc); err != nil {
		log.Fatalf("Failed to initialize document: %v", err)
	}

	// Note: We don't need to create a trackable struct for the server document
	// since we're using the tracker directly

	// Generate a unique document ID
	documentID := fmt.Sprintf("doc-%v", sid)

	return &Server{
		Clients:    make(map[int]*ClientInfo),
		Document:   serverDoc,
		Tracker:    t,
		PatchQueue: make([][]byte, 0),
		Broker:     broker,
		DocumentID: documentID,
	}
}

// RegisterClient registers a client with the server.
func (s *Server) RegisterClient(id int, name string) {
	s.QueueMutex.Lock()
	defer s.QueueMutex.Unlock()

	s.Clients[id] = &ClientInfo{
		ID:   id,
		Name: name,
	}

	fmt.Printf("Client %d (%s) connected to server\n", id, name)
}

// ProcessClientMessage processes a message from a client.
func (s *Server) ProcessClientMessage(msg Message) {
	// Print the patch for debugging
	fmt.Printf("Server received patch: %s\n", string(msg.Patch))

	// Apply the patch to the server's document
	if err := tracker.ApplyJSONCRDTPatch(s.Document, msg.Patch, s.Tracker.GetSessionID()); err != nil {
		log.Printf("Server failed to apply patch: %v", err)
	} else {
		// Broadcast the patch to all clients except the sender
		for clientID := range s.Clients {
			if clientID != msg.ClientID {
				s.Broker.ServerToClient <- Message{
					ClientID: clientID,
					Patch:    msg.Patch,
				}
			}
		}
	}
}

// Start starts the server.
func (s *Server) Start() {
	go func() {
		for msg := range s.Broker.ClientToServer {
			s.ProcessClientMessage(msg)
		}
	}()

	fmt.Println("Server started")
}

// NewClient creates a new client.
func NewClient(id int, name string, broker *MessageBroker, serverSID common.SessionID) *Client {
	// Create a new CRDT document for the client using the server's session ID
	doc := crdt.NewDocument(serverSID)

	// Create a tracker for the document
	t := tracker.NewTracker(doc, serverSID)

	return &Client{
		ID:      id,
		Name:    name,
		Tracker: t,
		Broker:  broker,
	}
}

// Initialize initializes the client with a document.
func (c *Client) Initialize(doc *Document) error {
	// Create a copy of the document
	clientDoc := &Document{
		Title:    doc.Title,
		Content:  doc.Content,
		Authors:  make([]string, len(doc.Authors)),
		Modified: doc.Modified,
	}
	copy(clientDoc.Authors, doc.Authors)

	// Initialize the document with the client's tracker
	if err := c.Tracker.InitializeDocument(clientDoc); err != nil {
		return fmt.Errorf("failed to initialize document: %w", err)
	}

	// Create a trackable struct for the client document
	trackable, err := tracker.NewTrackableStruct(clientDoc, c.Tracker.GetSessionID())
	if err != nil {
		return fmt.Errorf("failed to create trackable struct: %w", err)
	}

	c.Document = trackable
	fmt.Printf("Client %d (%s) connected to server\n", c.ID, c.Name)
	return nil
}

// StartListening starts listening for messages from the server.
func (c *Client) StartListening() {
	go func() {
		for msg := range c.Broker.ServerToClient {
			if msg.ClientID == c.ID {
				c.ProcessServerMessage(msg)
			}
		}
	}()
}

// ProcessServerMessage processes a message from the server.
func (c *Client) ProcessServerMessage(msg Message) {
	// Apply the patch to the client's document
	clientDoc := c.Document.GetData().(*Document)
	if err := tracker.ApplyJSONCRDTPatch(clientDoc, msg.Patch, c.Tracker.GetSessionID()); err != nil {
		log.Printf("Client %d failed to apply patch: %v", c.ID, err)
	} else {
		fmt.Printf("Client %d received update\n", c.ID)

		// Print the client document after applying the patch
		fmt.Printf("Client %d document after applying patch: %+v\n", c.ID, clientDoc)
	}
}

// EditDocument edits the document.
func (c *Client) EditDocument(doc *Document) {
	// Get the current document
	currentDoc := c.Document.GetData().(*Document)

	// Print the document before update
	fmt.Printf("Client %d document before update: %+v\n", c.ID, currentDoc)

	// Create a copy of the document with the updated fields
	updatedDoc := &Document{
		Title:    doc.Title,
		Content:  doc.Content,
		Authors:  make([]string, len(doc.Authors)),
		Modified: doc.Modified,
	}
	copy(updatedDoc.Authors, doc.Authors)

	// Update the document directly
	currentDoc.Title = updatedDoc.Title
	currentDoc.Content = updatedDoc.Content
	currentDoc.Authors = make([]string, len(updatedDoc.Authors))
	copy(currentDoc.Authors, updatedDoc.Authors)
	currentDoc.Modified = updatedDoc.Modified

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

	// Send the patch to the server
	c.Broker.ClientToServer <- Message{
		ClientID: c.ID,
		Patch:    patch,
	}
	fmt.Printf("Client %d (%s) edited the document and sent a patch to the server\n", c.ID, c.Name)
}

// GetDocument gets the document.
func (c *Client) GetDocument() *Document {
	return c.Document.GetData().(*Document)
}

func main() {
	// Create a message broker
	broker := NewMessageBroker()

	// Create a server
	server := NewServer(broker)

	// Start the server
	server.Start()

	// Create clients with the server's session ID
	serverSID := server.Tracker.GetSessionID()
	client1 := NewClient(1, "Alice", broker, serverSID)
	client2 := NewClient(2, "Bob", broker, serverSID)
	client3 := NewClient(3, "Charlie", broker, serverSID)

	// Register clients with the server
	server.RegisterClient(client1.ID, client1.Name)
	server.RegisterClient(client2.ID, client2.Name)
	server.RegisterClient(client3.ID, client3.Name)

	// Start clients listening for messages
	client1.StartListening()
	client2.StartListening()
	client3.StartListening()

	// Initialize clients with the server's document
	if err := client1.Initialize(server.Document); err != nil {
		log.Fatalf("Failed to initialize client 1: %v", err)
	}
	if err := client2.Initialize(server.Document); err != nil {
		log.Fatalf("Failed to initialize client 2: %v", err)
	}
	if err := client3.Initialize(server.Document); err != nil {
		log.Fatalf("Failed to initialize client 3: %v", err)
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
