package api

import (
	"context"
	"encoding/json"
	"net/http"

	"tictactoe/transport/cmd/cqrs_demo/business"
)

// RPCRequest는 JSON-RPC 요청 구조체입니다.
type RPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	ID      interface{}     `json:"id"`
}

// RPCResponse는 JSON-RPC 응답 구조체입니다.
type RPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

// RPCError는 JSON-RPC 에러 구조체입니다.
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// BatchRPCHandler는 여러 RPC 요청을 처리하는 핸들러입니다.
type BatchRPCHandler struct {
	transportService *business.TransportService
}

// NewBatchRPCHandler는 새로운 BatchRPCHandler를 생성합니다.
func NewBatchRPCHandler(transportService *business.TransportService) *BatchRPCHandler {
	return &BatchRPCHandler{
		transportService: transportService,
	}
}

// HandleRPC는 RPC 요청을 처리합니다.
func (h *BatchRPCHandler) HandleRPC(w http.ResponseWriter, r *http.Request) {
	// 요청 본문 디코딩
	var requests []RPCRequest
	var isBatch bool

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&requests); err != nil {
		// 배열이 아닌 경우 단일 요청으로 처리
		var singleRequest RPCRequest
		r.Body.Close()
		r.Body = http.MaxBytesReader(w, r.Body, 1048576) // 1MB 제한
		if err := json.NewDecoder(r.Body).Decode(&singleRequest); err != nil {
			writeError(w, &RPCError{
				Code:    -32700,
				Message: "Parse error",
				Data:    err.Error(),
			}, nil)
			return
		}
		requests = []RPCRequest{singleRequest}
		isBatch = false
	} else {
		isBatch = true
	}

	// 빈 배열 요청 처리
	if len(requests) == 0 {
		writeError(w, &RPCError{
			Code:    -32600,
			Message: "Invalid Request",
			Data:    "Empty batch",
		}, nil)
		return
	}

	// 각 요청 처리
	responses := make([]RPCResponse, 0, len(requests))
	for _, req := range requests {
		response := h.processRequest(r.Context(), req)
		responses = append(responses, response)
	}

	// 응답 반환
	w.Header().Set("Content-Type", "application/json")
	if isBatch {
		json.NewEncoder(w).Encode(responses)
	} else {
		json.NewEncoder(w).Encode(responses[0])
	}
}

// processRequest는 단일 RPC 요청을 처리합니다.
func (h *BatchRPCHandler) processRequest(ctx context.Context, req RPCRequest) RPCResponse {
	// JSON-RPC 버전 확인
	if req.JSONRPC != "2.0" {
		return RPCResponse{
			JSONRPC: "2.0",
			Error: &RPCError{
				Code:    -32600,
				Message: "Invalid Request",
				Data:    "Invalid JSON-RPC version",
			},
			ID: req.ID,
		}
	}

	// 메서드에 따라 처리
	var result interface{}
	var err *RPCError

	switch req.Method {
	case "createTransport":
		result, err = h.handleCreateTransport(ctx, req.Params)
	case "joinTransport":
		result, err = h.handleJoinTransport(ctx, req.Params)
	case "startTransport":
		result, err = h.handleStartTransport(ctx, req.Params)
	case "getTransport":
		result, err = h.handleGetTransport(ctx, req.Params)
	case "getActiveTransports":
		result, err = h.handleGetActiveTransports(ctx, req.Params)
	case "raidTransport":
		result, err = h.handleRaidTransport(ctx, req.Params)
	case "defendTransport":
		result, err = h.handleDefendTransport(ctx, req.Params)
	default:
		err = &RPCError{
			Code:    -32601,
			Message: "Method not found",
			Data:    req.Method,
		}
	}

	// 응답 생성
	response := RPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
	}

	if err != nil {
		response.Error = err
	} else {
		response.Result = result
	}

	return response
}

// handleCreateTransport는 이송 생성 요청을 처리합니다.
func (h *BatchRPCHandler) handleCreateTransport(ctx context.Context, params json.RawMessage) (interface{}, *RPCError) {
	var createParams business.CreateTransportParams
	if err := json.Unmarshal(params, &createParams); err != nil {
		return nil, &RPCError{
			Code:    -32602,
			Message: "Invalid params",
			Data:    err.Error(),
		}
	}

	result, err := h.transportService.CreateTransport(ctx, createParams)
	if err != nil {
		return nil, convertToRPCError(err)
	}

	return result, nil
}

// handleJoinTransport는 이송 참가 요청을 처리합니다.
func (h *BatchRPCHandler) handleJoinTransport(ctx context.Context, params json.RawMessage) (interface{}, *RPCError) {
	var joinParams business.JoinTransportParams
	if err := json.Unmarshal(params, &joinParams); err != nil {
		return nil, &RPCError{
			Code:    -32602,
			Message: "Invalid params",
			Data:    err.Error(),
		}
	}

	result, err := h.transportService.JoinTransport(ctx, joinParams)
	if err != nil {
		return nil, convertToRPCError(err)
	}

	return result, nil
}

// handleStartTransport는 이송 시작 요청을 처리합니다.
func (h *BatchRPCHandler) handleStartTransport(ctx context.Context, params json.RawMessage) (interface{}, *RPCError) {
	var startParams business.StartTransportParams
	if err := json.Unmarshal(params, &startParams); err != nil {
		return nil, &RPCError{
			Code:    -32602,
			Message: "Invalid params",
			Data:    err.Error(),
		}
	}

	result, err := h.transportService.StartTransport(ctx, startParams)
	if err != nil {
		return nil, convertToRPCError(err)
	}

	return result, nil
}

// handleGetTransport는 이송 조회 요청을 처리합니다.
func (h *BatchRPCHandler) handleGetTransport(ctx context.Context, params json.RawMessage) (interface{}, *RPCError) {
	var getParams business.GetTransportParams
	if err := json.Unmarshal(params, &getParams); err != nil {
		return nil, &RPCError{
			Code:    -32602,
			Message: "Invalid params",
			Data:    err.Error(),
		}
	}

	result, err := h.transportService.GetTransport(ctx, getParams)
	if err != nil {
		return nil, convertToRPCError(err)
	}

	return result, nil
}

// handleGetActiveTransports는 활성 이송 목록 조회 요청을 처리합니다.
func (h *BatchRPCHandler) handleGetActiveTransports(ctx context.Context, params json.RawMessage) (interface{}, *RPCError) {
	var listParams business.GetActiveTransportsParams
	if err := json.Unmarshal(params, &listParams); err != nil {
		return nil, &RPCError{
			Code:    -32602,
			Message: "Invalid params",
			Data:    err.Error(),
		}
	}

	result, err := h.transportService.GetActiveTransports(ctx, listParams)
	if err != nil {
		return nil, convertToRPCError(err)
	}

	return result, nil
}

// handleRaidTransport는 이송 약탈 요청을 처리합니다.
func (h *BatchRPCHandler) handleRaidTransport(ctx context.Context, params json.RawMessage) (interface{}, *RPCError) {
	var raidParams business.RaidTransportParams
	if err := json.Unmarshal(params, &raidParams); err != nil {
		return nil, &RPCError{
			Code:    -32602,
			Message: "Invalid params",
			Data:    err.Error(),
		}
	}

	result, err := h.transportService.RaidTransport(ctx, raidParams)
	if err != nil {
		return nil, convertToRPCError(err)
	}

	return result, nil
}

// handleDefendTransport는 이송 방어 요청을 처리합니다.
func (h *BatchRPCHandler) handleDefendTransport(ctx context.Context, params json.RawMessage) (interface{}, *RPCError) {
	var defendParams business.DefendTransportParams
	if err := json.Unmarshal(params, &defendParams); err != nil {
		return nil, &RPCError{
			Code:    -32602,
			Message: "Invalid params",
			Data:    err.Error(),
		}
	}

	result, err := h.transportService.DefendTransport(ctx, defendParams)
	if err != nil {
		return nil, convertToRPCError(err)
	}

	return result, nil
}

// writeError는 에러 응답을 작성합니다.
func writeError(w http.ResponseWriter, err *RPCError, id interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // JSON-RPC는 항상 200 OK 반환

	response := RPCResponse{
		JSONRPC: "2.0",
		Error:   err,
		ID:      id,
	}

	json.NewEncoder(w).Encode(response)
}

// convertToRPCError는 일반 에러를 RPC 에러로 변환합니다.
func convertToRPCError(err error) *RPCError {
	if err == nil {
		return nil
	}

	// 비즈니스 에러 타입에 따라 적절한 에러 코드 할당
	// 실제 구현에서는 더 세분화된 에러 처리 필요
	switch err.(type) {
	case *business.ValidationError:
		return &RPCError{
			Code:    -32001,
			Message: "Validation error",
			Data:    err.Error(),
		}
	case *business.NotFoundError:
		return &RPCError{
			Code:    -32002,
			Message: "Not found",
			Data:    err.Error(),
		}
	case *business.ConflictError:
		return &RPCError{
			Code:    -32003,
			Message: "Conflict",
			Data:    err.Error(),
		}
	default:
		return &RPCError{
			Code:    -32000,
			Message: "Server error",
			Data:    err.Error(),
		}
	}
}
