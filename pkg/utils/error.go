package utils

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
)

// 컨텍스트 키 타입 정의
type contextKey string

// 에러 컨텍스트 키
const (
	ErrorContextKey contextKey = "error"
)

// ErrorContext는 에러 정보를 저장하는 구조체
type ErrorContext struct {
	Error     error
	Message   string
	Stack     string
	Code      int
	RequestID string
	Path      string
	Method    string
}

// WithError는 HTTP 요청 컨텍스트에 에러 정보를 저장합니다.
func WithError(r *http.Request, err error) *http.Request {
	if r == nil {
		return nil
	}

	// 기존 에러 컨텍스트 확인
	var errCtx *ErrorContext
	if existingCtx := GetErrorContext(r.Context()); existingCtx != nil {
		// 기존 에러 컨텍스트가 있으면 복사
		errCtx = existingCtx
		errCtx.Error = err
	} else {
		// 새 에러 컨텍스트 생성
		errCtx = &ErrorContext{
			Error:   err,
			Message: err.Error(),
			Stack:   string(debug.Stack()),
			Code:    http.StatusInternalServerError,
			Path:    r.URL.Path,
			Method:  r.Method,
		}
	}

	// 요청 ID가 있으면 설정
	if requestID := r.Header.Get("X-Request-ID"); requestID != "" {
		errCtx.RequestID = requestID
	}

	// 컨텍스트에 에러 정보 저장
	ctx := context.WithValue(r.Context(), ErrorContextKey, errCtx)
	return r.WithContext(ctx)
}

// WithErrorAndCode는 HTTP 요청 컨텍스트에 에러 정보와 상태 코드를 저장합니다.
func WithErrorAndCode(r *http.Request, err error, code int) *http.Request {
	if r == nil {
		return nil
	}

	// WithError로 에러 정보 저장
	newReq := WithError(r, err)
	
	// 에러 컨텍스트 가져오기
	errCtx := GetErrorContext(newReq.Context())
	if errCtx != nil {
		// 상태 코드 설정
		errCtx.Code = code
	}

	return newReq
}

// WithErrorAndMessage는 HTTP 요청 컨텍스트에 에러 정보와 사용자 정의 메시지를 저장합니다.
func WithErrorAndMessage(r *http.Request, err error, message string) *http.Request {
	if r == nil {
		return nil
	}

	// WithError로 에러 정보 저장
	newReq := WithError(r, err)
	
	// 에러 컨텍스트 가져오기
	errCtx := GetErrorContext(newReq.Context())
	if errCtx != nil {
		// 메시지 설정
		errCtx.Message = message
	}

	return newReq
}

// WithErrorAndCodeAndMessage는 HTTP 요청 컨텍스트에 에러 정보, 상태 코드, 사용자 정의 메시지를 저장합니다.
func WithErrorAndCodeAndMessage(r *http.Request, err error, code int, message string) *http.Request {
	if r == nil {
		return nil
	}

	// WithError로 에러 정보 저장
	newReq := WithError(r, err)
	
	// 에러 컨텍스트 가져오기
	errCtx := GetErrorContext(newReq.Context())
	if errCtx != nil {
		// 상태 코드와 메시지 설정
		errCtx.Code = code
		errCtx.Message = message
	}

	return newReq
}

// GetErrorContext는 컨텍스트에서 에러 정보를 가져옵니다.
func GetErrorContext(ctx context.Context) *ErrorContext {
	if ctx == nil {
		return nil
	}

	// 컨텍스트에서 에러 정보 가져오기
	if errCtx, ok := ctx.Value(ErrorContextKey).(*ErrorContext); ok {
		return errCtx
	}

	return nil
}

// HasError는 컨텍스트에 에러가 있는지 확인합니다.
func HasError(ctx context.Context) bool {
	return GetErrorContext(ctx) != nil
}

// WriteError는 에러 응답을 클라이언트에 전송합니다.
func WriteError(w http.ResponseWriter, r *http.Request) {
	// 에러 컨텍스트 가져오기
	errCtx := GetErrorContext(r.Context())
	if errCtx == nil {
		// 에러 컨텍스트가 없으면 기본 에러 응답
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 에러 응답 전송
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(errCtx.Code)

	// JSON 형식의 에러 응답
	fmt.Fprintf(w, `{"error":"%s","code":%d,"path":"%s","method":"%s"`,
		errCtx.Message, errCtx.Code, errCtx.Path, errCtx.Method)
	
	// 요청 ID가 있으면 추가
	if errCtx.RequestID != "" {
		fmt.Fprintf(w, `,"request_id":"%s"`, errCtx.RequestID)
	}

	// JSON 닫기
	fmt.Fprint(w, "}")
}
