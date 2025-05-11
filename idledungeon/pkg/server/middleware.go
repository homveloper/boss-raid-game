package server

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"time"

	"go.uber.org/zap"
)

// responseWriter는 http.ResponseWriter를 래핑하여 상태 코드와 응답 크기를 추적합니다.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	buffer     *bytes.Buffer
}

// newResponseWriter는 새로운 responseWriter를 생성합니다.
func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		buffer:         &bytes.Buffer{},
	}
}

// WriteHeader는 상태 코드를 설정하고 원래 ResponseWriter에 전달합니다.
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write는 응답 본문을 버퍼에 복사하고 원래 ResponseWriter에 전달합니다.
func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.buffer.Write(b)
	return rw.ResponseWriter.Write(b)
}

// LoggingMiddleware는 HTTP 요청과 응답을 로깅하는 미들웨어입니다.
func LoggingMiddleware(logger *zap.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 요청 정보 로깅
		logger.Debug("Request received",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("remoteAddr", r.RemoteAddr),
			zap.String("userAgent", r.UserAgent()),
		)

		// 요청 본문 읽기 (API 요청인 경우에만)
		var requestBody []byte
		if r.Method == http.MethodPost || r.Method == http.MethodPut {
			if r.Body != nil {
				var err error
				requestBody, err = io.ReadAll(r.Body)
				if err != nil {
					logger.Error("Failed to read request body", zap.Error(err))
				}
				r.Body = io.NopCloser(bytes.NewBuffer(requestBody))
			}
		}

		// 응답 래핑
		rw := newResponseWriter(w)

		// 다음 핸들러 호출 (패닉 복구 포함)
		func() {
			defer func() {
				if err := recover(); err != nil {
					stack := string(debug.Stack())
					logger.Error("Panic in handler",
						zap.Any("error", err),
						zap.String("stack", stack),
						zap.String("method", r.Method),
						zap.String("path", r.URL.Path),
						zap.String("remoteAddr", r.RemoteAddr),
					)

					// 콘솔에도 출력
					fmt.Printf("Panic in handler: %v\nStack: %s\n", err, stack)

					// 클라이언트에 500 응답
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(rw, r)
		}()

		// 요청 처리 시간 계산
		duration := time.Since(start)

		// 로그 레벨 결정
		logLevel := zap.InfoLevel
		if rw.statusCode >= 400 {
			logLevel = zap.WarnLevel
		}
		if rw.statusCode >= 500 {
			logLevel = zap.ErrorLevel
		}

		// 로그 필드 준비
		fields := []zap.Field{
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", rw.statusCode),
			zap.Duration("duration", duration),
			zap.String("ip", r.RemoteAddr),
			zap.String("user-agent", r.UserAgent()),
		}

		// API 요청인 경우 요청 본문 로깅
		if len(requestBody) > 0 && len(requestBody) < 1024 {
			fields = append(fields, zap.String("request", string(requestBody)))
		}

		// API 응답인 경우 응답 본문 로깅
		if r.URL.Path != "/" && r.URL.Path != "/events" && rw.buffer.Len() < 1024 {
			fields = append(fields, zap.String("response", rw.buffer.String()))
		}

		// 로그 출력
		logger.Check(logLevel, fmt.Sprintf("HTTP %s %s", r.Method, r.URL.Path)).Write(fields...)

		// 콘솔에도 출력 (에러인 경우)
		if rw.statusCode >= 400 {
			fmt.Printf("HTTP %s %s - Status: %d, Duration: %v\n",
				r.Method, r.URL.Path, rw.statusCode, duration)
		}
	})
}

// RecoveryMiddleware는 패닉을 복구하고 로깅하는 미들웨어입니다.
func RecoveryMiddleware(logger *zap.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
					zap.String("referer", r.Referer()),
				)

				// 콘솔에도 출력
				fmt.Printf("HTTP handler panic: %v\nMethod: %s\nPath: %s\nStack: %s\n",
					err, r.Method, r.URL.Path, stack)

				// 클라이언트에 500 응답
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// CORSMiddleware는 CORS 헤더를 추가하는 미들웨어입니다.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS 헤더 설정
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// OPTIONS 요청 처리
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// MiddlewareChain은 여러 미들웨어를 체인으로 연결합니다.
func MiddlewareChain(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for _, middleware := range middlewares {
		h = middleware(h)
	}
	return h
}
