package nodestorage

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TestVersionError는 VersionError 구조체와 관련 메서드를 테스트합니다.
func TestVersionError(t *testing.T) {
	// 테스트 데이터
	docID := primitive.NewObjectID()
	currentVersion := int64(1)
	storedVersion := int64(2)

	// VersionError 생성
	vErr := NewVersionError(docID, currentVersion, storedVersion)

	// 필드 확인
	assert.Equal(t, docID, vErr.DocumentID)
	assert.Equal(t, currentVersion, vErr.CurrentVersion)
	assert.Equal(t, storedVersion, vErr.StoredVersion)

	// Error 메서드 확인
	errMsg := vErr.Error()
	assert.Contains(t, errMsg, docID.Hex())
	assert.Contains(t, errMsg, "1")
	assert.Contains(t, errMsg, "2")

	// Is 메서드 확인
	assert.True(t, vErr.Is(ErrVersionMismatch))
	assert.False(t, vErr.Is(ErrNotFound))

	// Unwrap 메서드 확인
	assert.Equal(t, ErrVersionMismatch, vErr.Unwrap())

	// errors.Is 함수와 함께 사용
	assert.True(t, errors.Is(vErr, ErrVersionMismatch))
	assert.False(t, errors.Is(vErr, ErrNotFound))
}

// TestErrorVariables는 에러 변수들을 테스트합니다.
func TestErrorVariables(t *testing.T) {
	// 모든 에러 변수 확인
	assert.Equal(t, "document not found", ErrNotFound.Error())
	assert.Equal(t, "document version mismatch", ErrVersionMismatch.Error())
	assert.Equal(t, "invalid document", ErrInvalidDocument.Error())
	assert.Equal(t, "cache miss", ErrCacheMiss.Error())
	assert.Equal(t, "operation timed out", ErrTimeout.Error())
	assert.Equal(t, "maximum retries exceeded", ErrMaxRetriesExceeded.Error())
	assert.Equal(t, "storage is closed", ErrClosed.Error())
	assert.Equal(t, "version field is required in options", ErrMissingVersionField.Error())
}
