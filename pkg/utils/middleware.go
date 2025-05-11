package utils

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"go.uber.org/zap"
)

// ErrorHandlerMiddleware는 에러를 처리하는 미들웨어입니다.
func ErrorHandlerMiddleware(logger *zap.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 패닉 복구
		defer func() {
			if err := recover(); err != nil {
				// 스택 트레이스 캡처
				stack := string(debug.Stack())

				// 에러 로깅
				logger.Error("HTTP handler panic",
					zap.Any("error", err),
					zap.String("stack", stack),
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path),
					zap.String("ip", r.RemoteAddr),
					zap.String("userAgent", r.UserAgent()),
				)

				// 콘솔에도 출력
				fmt.Printf("HTTP handler panic: %v\nStack: %s\n", err, stack)

				// 에러 응답 전송
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		// 다음 핸들러 호출
		next.ServeHTTP(w, r)

		// 에러 컨텍스트 확인
		if errCtx := GetErrorContext(r.Context()); errCtx != nil {
			// 에러 로깅
			logger.Error("Request error",
				zap.Error(errCtx.Error),
				zap.String("message", errCtx.Message),
				zap.Int("code", errCtx.Code),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("ip", r.RemoteAddr),
				zap.String("userAgent", r.UserAgent()),
			)

			// 에러 응답 전송
			WriteError(w, r)
		}
	})
}

// RequestIDMiddleware는 요청 ID를 생성하는 미들웨어입니다.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 요청 ID 확인
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			// 요청 ID가 없으면 생성
			requestID = GenerateRequestID()
			r.Header.Set("X-Request-ID", requestID)
		}

		// 응답 헤더에 요청 ID 설정
		w.Header().Set("X-Request-ID", requestID)

		// 다음 핸들러 호출
		next.ServeHTTP(w, r)
	})
}

// GenerateRequestID는 고유한 요청 ID를 생성합니다.
func GenerateRequestID() string {
	// 현재 시간과 랜덤 문자열을 조합하여 고유한 ID 생성
	return fmt.Sprintf("%d-%s", GetTimestamp(), RandomString(8))
}

// GetTimestamp는 현재 시간을 밀리초 단위로 반환합니다.
func GetTimestamp() int64 {
	return GetTimeNow().UnixNano() / int64(1000000)
}

// RandomString은 지정된 길이의 랜덤 문자열을 생성합니다.
func RandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[GetRandom().Intn(len(charset))]
	}
	return string(result)
}
