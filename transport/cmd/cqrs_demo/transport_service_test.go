package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"tictactoe/transport/cqrs/application"
	"tictactoe/transport/cqrs/domain"
	"tictactoe/transport/cqrs/infrastructure"
)

// MockCommandBus is a mock implementation of the CommandBus interface
type MockCommandBus struct {
	mock.Mock
}

func (m *MockCommandBus) Dispatch(ctx context.Context, command domain.Command) error {
	args := m.Called(ctx, command)
	return args.Error(0)
}

func (m *MockCommandBus) RegisterHandler(commandType string, handler infrastructure.CommandHandler) error {
	args := m.Called(commandType, handler)
	return args.Error(0)
}

// MockCollection is a mock implementation of the mongo.Collection
type MockCollection struct {
	*mongo.Collection
}

func newTestMongoCollection() *mongo.Collection {
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		panic(err)
	}
	return client.Database("test").Collection("test")
}

func TestCreateTransportHandler(t *testing.T) {
	// Setup
	mockCommandBus := new(MockCommandBus)
	mockCollection := newTestMongoCollection()

	service := NewTransportService(mockCommandBus, mockCollection)

	// Create request
	reqBody := CreateTransportRequest{
		AllianceID:      "alliance1",
		PlayerID:        "player1",
		PlayerName:      "Player One",
		MineID:          "mine1",
		MineName:        "Gold Mine",
		MineLevel:       1,
		GeneralID:       "general1",
		GoldAmount:      100,
		MaxParticipants: 5,
		PrepTime:        30,
		TransportTime:   60,
	}
	reqJSON, _ := json.Marshal(reqBody)

	// Setup mock expectations
	mockCommandBus.On("Dispatch", mock.Anything, mock.MatchedBy(func(cmd domain.Command) bool {
		createCmd, ok := cmd.(*domain.CreateTransportCommand)
		return ok &&
			createCmd.AllianceID == reqBody.AllianceID &&
			createCmd.PlayerID == reqBody.PlayerID &&
			createCmd.GoldAmount == reqBody.GoldAmount
	})).Return(nil)

	// Create request
	req, err := http.NewRequest("POST", "/transports", bytes.NewBuffer(reqJSON))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	handler := http.HandlerFunc(service.CreateTransportHandler)
	handler.ServeHTTP(rr, req)

	// Check response
	assert.Equal(t, http.StatusOK, rr.Code)

	// Verify mock expectations
	mockCommandBus.AssertExpectations(t)
}

func TestStartTransportHandler(t *testing.T) {
	// Setup
	mockCommandBus := new(MockCommandBus)
	mockCollection := newTestMongoCollection()

	service := NewTransportService(mockCommandBus, mockCollection)

	// Setup mock expectations
	transportID := "transport1"
	transport := application.TransportReadModel{
		ID:            transportID,
		TransportTime: 60,
	}

	// Create a mock SingleResult that returns our transport
	mockResult := mongo.NewSingleResultFromDocument(transport, nil, nil)
	mockCollection.On("FindOne", mock.Anything, map[string]interface{}{"_id": transportID}, mock.Anything).Return(mockResult)

	mockCommandBus.On("Dispatch", mock.Anything, mock.MatchedBy(func(cmd domain.Command) bool {
		startCmd, ok := cmd.(*domain.StartTransportCommand)
		return ok && startCmd.AggregateID() == transportID
	})).Return(nil)

	// Create request
	req, err := http.NewRequest("POST", "/transports/"+transportID+"/start", nil)
	assert.NoError(t, err)

	// Add URL parameters
	vars := map[string]string{
		"id": transportID,
	}
	req = mux.SetURLVars(req, vars)

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	handler := http.HandlerFunc(service.StartTransportHandler)
	handler.ServeHTTP(rr, req)

	// Check response
	assert.Equal(t, http.StatusOK, rr.Code)

	// Verify mock expectations
	mockCommandBus.AssertExpectations(t)
	mockCollection.AssertExpectations(t)
}

func TestGetTransportHandler(t *testing.T) {
	// Setup
	mockCommandBus := new(MockCommandBus)
	mockCollection := newTestMongoCollection()

	service := NewTransportService(mockCommandBus, mockCollection)

	// Setup mock expectations
	transportID := "transport1"
	transport := application.TransportReadModel{
		ID:            transportID,
		AllianceID:    "alliance1",
		PlayerID:      "player1",
		Status:        "PREPARING",
		GoldAmount:    100,
		TransportTime: 60,
	}

	// Create a mock SingleResult that returns our transport
	mockResult := mongo.NewSingleResultFromDocument(transport, nil, nil)
	mockCollection.On("FindOne", mock.Anything, map[string]interface{}{"_id": transportID}, mock.Anything).Return(mockResult)

	// Create request
	req, err := http.NewRequest("GET", "/transports/"+transportID, nil)
	assert.NoError(t, err)

	// Add URL parameters
	vars := map[string]string{
		"id": transportID,
	}
	req = mux.SetURLVars(req, vars)

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	handler := http.HandlerFunc(service.GetTransportHandler)
	handler.ServeHTTP(rr, req)

	// Check response
	assert.Equal(t, http.StatusOK, rr.Code)

	// Parse response
	var responseTransport application.TransportReadModel
	err = json.Unmarshal(rr.Body.Bytes(), &responseTransport)
	assert.NoError(t, err)

	// Verify response data
	assert.Equal(t, transport.ID, responseTransport.ID)
	assert.Equal(t, transport.AllianceID, responseTransport.AllianceID)
	assert.Equal(t, transport.Status, responseTransport.Status)
	assert.Equal(t, transport.GoldAmount, responseTransport.GoldAmount)

	// Verify mock expectations
	mockCollection.AssertExpectations(t)
}

func TestRaidTransportHandler(t *testing.T) {
	// Setup
	mockCommandBus := new(MockCommandBus)
	mockCollection := newTestMongoCollection()

	service := NewTransportService(mockCommandBus, mockCollection)

	// Create request
	reqBody := RaidTransportRequest{
		RaiderID:   "raider1",
		RaiderName: "Raider One",
	}
	reqJSON, _ := json.Marshal(reqBody)

	// Setup mock expectations
	transportID := "transport1"
	mockCommandBus.On("Dispatch", mock.Anything, mock.MatchedBy(func(cmd domain.Command) bool {
		raidCmd, ok := cmd.(*domain.RaidTransportCommand)
		return ok &&
			raidCmd.AggregateID() == transportID &&
			raidCmd.RaiderID == reqBody.RaiderID &&
			raidCmd.RaiderName == reqBody.RaiderName
	})).Return(nil)

	// Create request
	req, err := http.NewRequest("POST", "/transports/"+transportID+"/raid", bytes.NewBuffer(reqJSON))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	// Add URL parameters
	vars := map[string]string{
		"id": transportID,
	}
	req = mux.SetURLVars(req, vars)

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	handler := http.HandlerFunc(service.RaidTransportHandler)
	handler.ServeHTTP(rr, req)

	// Check response
	assert.Equal(t, http.StatusOK, rr.Code)

	// Verify mock expectations
	mockCommandBus.AssertExpectations(t)
}

func TestDefendTransportHandler(t *testing.T) {
	// Setup
	mockCommandBus := new(MockCommandBus)
	mockCollection := newTestMongoCollection()

	service := NewTransportService(mockCommandBus, mockCollection)

	// Create request
	reqBody := DefendTransportRequest{
		DefenderID:   "defender1",
		DefenderName: "Defender One",
		Successful:   true,
	}
	reqJSON, _ := json.Marshal(reqBody)

	// Setup mock expectations
	transportID := "transport1"
	transport := application.TransportReadModel{
		ID:         transportID,
		GoldAmount: 100,
	}

	// Create a mock SingleResult that returns our transport
	mockResult := mongo.NewSingleResultFromDocument(transport, nil, nil)
	mockCollection.On("FindOne", mock.Anything, map[string]interface{}{"_id": transportID}, mock.Anything).Return(mockResult)

	mockCommandBus.On("Dispatch", mock.Anything, mock.MatchedBy(func(cmd domain.Command) bool {
		defendCmd, ok := cmd.(*domain.DefendTransportCommand)
		return ok &&
			defendCmd.AggregateID() == transportID &&
			defendCmd.DefenderID == reqBody.DefenderID &&
			defendCmd.DefenderName == reqBody.DefenderName &&
			defendCmd.Successful == reqBody.Successful
	})).Return(nil)

	// Create request
	req, err := http.NewRequest("POST", "/transports/"+transportID+"/defend", bytes.NewBuffer(reqJSON))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	// Add URL parameters
	vars := map[string]string{
		"id": transportID,
	}
	req = mux.SetURLVars(req, vars)

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	handler := http.HandlerFunc(service.DefendTransportHandler)
	handler.ServeHTTP(rr, req)

	// Check response
	assert.Equal(t, http.StatusOK, rr.Code)

	// Verify mock expectations
	mockCommandBus.AssertExpectations(t)
	mockCollection.AssertExpectations(t)
}
