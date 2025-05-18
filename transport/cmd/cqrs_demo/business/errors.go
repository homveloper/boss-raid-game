package business

import (
	"fmt"
)

// ValidationError는 입력 유효성 검증 실패 시 발생하는 에러입니다.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: %s - %s", e.Field, e.Message)
}

// NewValidationError는 새로운 ValidationError를 생성합니다.
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}

// NotFoundError는 리소스를 찾을 수 없을 때 발생하는 에러입니다.
type NotFoundError struct {
	ResourceType string
	ID           string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s with ID %s not found", e.ResourceType, e.ID)
}

// NewNotFoundError는 새로운 NotFoundError를 생성합니다.
func NewNotFoundError(resourceType, id string) *NotFoundError {
	return &NotFoundError{
		ResourceType: resourceType,
		ID:           id,
	}
}

// ConflictError는 리소스 충돌 시 발생하는 에러입니다.
type ConflictError struct {
	ResourceType string
	Message      string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("%s conflict: %s", e.ResourceType, e.Message)
}

// NewConflictError는 새로운 ConflictError를 생성합니다.
func NewConflictError(resourceType, message string) *ConflictError {
	return &ConflictError{
		ResourceType: resourceType,
		Message:      message,
	}
}

// AuthorizationError는 권한 부족 시 발생하는 에러입니다.
type AuthorizationError struct {
	Message string
}

func (e *AuthorizationError) Error() string {
	return fmt.Sprintf("authorization error: %s", e.Message)
}

// NewAuthorizationError는 새로운 AuthorizationError를 생성합니다.
func NewAuthorizationError(message string) *AuthorizationError {
	return &AuthorizationError{
		Message: message,
	}
}

// ServerError는 서버 내부 오류 시 발생하는 에러입니다.
type ServerError struct {
	Message string
	Cause   error
}

func (e *ServerError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("server error: %s - caused by: %v", e.Message, e.Cause)
	}
	return fmt.Sprintf("server error: %s", e.Message)
}

// NewServerError는 새로운 ServerError를 생성합니다.
func NewServerError(message string, cause error) *ServerError {
	return &ServerError{
		Message: message,
		Cause:   cause,
	}
}
