package server

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// LoggingMiddleware는 HTTP 요청에 대한 로깅을 수행하는 미들웨어입니다.
func LoggingMiddleware(logger *zap.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 요청 정보 로깅
		logger.Info("Request started",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("remote_addr", r.RemoteAddr),
			zap.String("user_agent", r.UserAgent()),
		)

		// 응답 래퍼 생성
		wrapper := &responseWrapper{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // 기본값
		}

		// 다음 핸들러 호출
		next.ServeHTTP(wrapper, r)

		// 응답 정보 로깅
		duration := time.Since(start)
		logger.Info("Request completed",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", wrapper.statusCode),
			zap.Duration("duration", duration),
		)
	})
}

// RecoveryMiddleware는 패닉 발생 시 복구하고 로깅하는 미들웨어입니다.
func RecoveryMiddleware(logger *zap.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// 패닉 발생 시 로깅
				logger.Error("Panic recovered",
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path),
					zap.Any("error", err),
				)

				// 클라이언트에 500 에러 응답
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		// 다음 핸들러 호출
		next.ServeHTTP(w, r)
	})
}

// WithError는 요청 컨텍스트에 에러를 저장하는 유틸리티 함수입니다.
func WithError(r *http.Request, err error) *http.Request {
	ctx := r.Context()
	ctx = context.WithValue(ctx, "error", err)
	return r.WithContext(ctx)
}

// responseWrapper는 응답 상태 코드를 캡처하기 위한 http.ResponseWriter 래퍼입니다.
type responseWrapper struct {
	http.ResponseWriter
	statusCode  int
	wroteHeader bool
}

// WriteHeader는 상태 코드를 캡처하고 원래 ResponseWriter에 위임합니다.
func (rw *responseWrapper) WriteHeader(statusCode int) {
	if !rw.wroteHeader {
		rw.statusCode = statusCode
		rw.wroteHeader = true
		rw.ResponseWriter.WriteHeader(statusCode)
	}
}

// Write는 Write 메서드를 오버라이드하여 상태 코드가 설정되지 않은 경우 기본값을 설정합니다.
func (rw *responseWrapper) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// Flush는 http.Flusher 인터페이스를 구현합니다 (SSE에 필요).
func (rw *responseWrapper) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Hijack은 http.Hijacker 인터페이스를 구현합니다 (WebSocket에 필요).
func (rw *responseWrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
}
