package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

// responseWriter is a wrapper for http.ResponseWriter that captures the status code and response body
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

// newResponseWriter creates a new responseWriter
func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		body:           &bytes.Buffer{},
	}
}

// WriteHeader captures the status code
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write captures the response body
func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.body.Write(b)
	return rw.ResponseWriter.Write(b)
}

// Flush implements the http.Flusher interface
func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// LoggingMiddleware logs request and response details
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Start timer
		start := time.Now()

		// Read request body
		var requestBody []byte
		if r.Body != nil {
			requestBody, _ = io.ReadAll(r.Body)
			// Restore the body for the next handler
			r.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		// Create a custom response writer to capture the response
		rw := newResponseWriter(w)

		// Process the request
		next.ServeHTTP(rw, r)

		// Calculate duration
		duration := time.Since(start)

		// Format request body for logging
		var requestJSON interface{}
		if len(requestBody) > 0 {
			if err := json.Unmarshal(requestBody, &requestJSON); err != nil {
				requestJSON = string(requestBody)
			}
		}

		// Format response body for logging
		var responseJSON interface{}
		if rw.body.Len() > 0 {
			if err := json.Unmarshal(rw.body.Bytes(), &responseJSON); err != nil {
				responseJSON = rw.body.String()
			}
		}

		// Create log entry
		logEntry := map[string]interface{}{
			"timestamp":    time.Now().Format(time.RFC3339),
			"method":       r.Method,
			"path":         r.URL.Path,
			"query":        r.URL.RawQuery,
			"status_code":  rw.statusCode,
			"duration_ms":  duration.Milliseconds(),
			"request_body": requestJSON,
			"response":     responseJSON,
		}

		// Log the entry
		logJSON, _ := json.MarshalIndent(logEntry, "", "  ")
		log.Printf("REQUEST LOG: %s\n", string(logJSON))
	})
}

// RecoveryMiddleware recovers from panics and logs the error
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// Log the stack trace
				stackTrace := debug.Stack()
				log.Printf("PANIC RECOVERED: %v\n%s\n", err, stackTrace)

				// Return an error response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				errorResponse := map[string]interface{}{
					"error":       "Internal Server Error",
					"reason":      fmt.Sprintf("%v", err),
					"stack_trace": string(stackTrace),
				}
				json.NewEncoder(w).Encode(errorResponse)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// ApplyMiddleware applies multiple middleware to a handler
func ApplyMiddleware(handler http.Handler, middleware ...func(http.Handler) http.Handler) http.Handler {
	for _, m := range middleware {
		handler = m(handler)
	}
	return handler
}
