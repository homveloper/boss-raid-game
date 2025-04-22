package crdtstorage

import (
	"time"
)

// EditOptions는 편집 작업에 대한 옵션을 정의합니다.
type EditOptions struct {
	// MaxRetries는 충돌 발생 시 최대 재시도 횟수입니다.
	MaxRetries int

	// UseDistributedLock은 분산 락 사용 여부입니다.
	UseDistributedLock bool

	// RetryDelay는 재시도 간 기본 지연 시간입니다.
	RetryDelay time.Duration

	// ExponentialBackoff는 지수 백오프 사용 여부입니다.
	ExponentialBackoff bool

	// SaveAfterEdit는 편집 후 자동 저장 여부입니다.
	SaveAfterEdit bool

	// TransactionTimeout은 트랜잭션 타임아웃 시간입니다.
	TransactionTimeout time.Duration
}

// DefaultEditOptions는 기본 편집 옵션을 반환합니다.
func DefaultEditOptions() *EditOptions {
	return &EditOptions{
		MaxRetries:         3,
		UseDistributedLock: false,
		RetryDelay:         100 * time.Millisecond,
		ExponentialBackoff: true,
		SaveAfterEdit:      true,
		TransactionTimeout: 30 * time.Second,
	}
}

// EditOption은 편집 옵션을 설정하는 함수 타입입니다.
type EditOption func(*EditOptions)

// WithMaxRetries는 최대 재시도 횟수를 설정합니다.
func WithMaxRetries(maxRetries int) EditOption {
	return func(o *EditOptions) {
		o.MaxRetries = maxRetries
	}
}

// WithDistributedLock은 분산 락 사용 여부를 설정합니다.
func WithDistributedLock(use bool) EditOption {
	return func(o *EditOptions) {
		o.UseDistributedLock = use
	}
}

// WithRetryDelay는 재시도 간 기본 지연 시간을 설정합니다.
func WithRetryDelay(delay time.Duration) EditOption {
	return func(o *EditOptions) {
		o.RetryDelay = delay
	}
}

// WithExponentialBackoff는 지수 백오프 사용 여부를 설정합니다.
func WithExponentialBackoff(use bool) EditOption {
	return func(o *EditOptions) {
		o.ExponentialBackoff = use
	}
}

// WithSaveAfterEdit는 편집 후 자동 저장 여부를 설정합니다.
func WithSaveAfterEdit(save bool) EditOption {
	return func(o *EditOptions) {
		o.SaveAfterEdit = save
	}
}

// WithTransactionTimeout은 트랜잭션 타임아웃 시간을 설정합니다.
func WithTransactionTimeout(timeout time.Duration) EditOption {
	return func(o *EditOptions) {
		o.TransactionTimeout = timeout
	}
}
