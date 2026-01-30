package api

import (
	"context"
	"net/http"
	"time"

	"aichatplayers/internal/logging"
)

type ctxKey string

const requestIDKey ctxKey = "request_id"

func RequestIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}

func WithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-Id")
		if reqID == "" {
			reqID = generateRequestID()
		}
		ctx := context.WithValue(r.Context(), requestIDKey, reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := RequestIDFromContext(r.Context())
		start := time.Now()
		recorder := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		logging.Infof(
			"ts=%s request_id=%s transaction_id=%s method=%s path=%s status=%d bytes=%d duration_ms=%d remote_addr=%s user_agent=%q",
			start.Format(time.RFC3339),
			reqID,
			reqID,
			r.Method,
			r.URL.Path,
			recorder.status,
			recorder.bytes,
			time.Since(start).Milliseconds(),
			r.RemoteAddr,
			r.UserAgent(),
		)
	})
}

func LimitBodySize(limit int64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		next.ServeHTTP(w, r)
	})
}

func generateRequestID() string {
	return time.Now().Format("20060102T150405.000000000")
}

type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	size, err := r.ResponseWriter.Write(data)
	r.bytes += size
	return size, err
}
