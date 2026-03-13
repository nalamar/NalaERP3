package apihttp

import (
	"context"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

type contextKey string

const (
	requestIDKey     contextKey = "request_id"
	correlationIDKey contextKey = "correlation_id"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

func requestContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestID := middleware.GetReqID(req.Context())
		correlationID := strings.TrimSpace(req.Header.Get("X-Correlation-ID"))
		if correlationID == "" {
			correlationID = requestID
		}
		ctx := context.WithValue(req.Context(), requestIDKey, requestID)
		ctx = context.WithValue(ctx, correlationIDKey, correlationID)
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("X-Correlation-ID", correlationID)
		next.ServeHTTP(w, req.WithContext(ctx))
	})
}

func requestLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, req)

		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}

		log.Printf(
			"http_request method=%s path=%s query=%q status=%d duration_ms=%d bytes=%d request_id=%s correlation_id=%s remote_ip=%s user_agent=%q",
			req.Method,
			req.URL.Path,
			req.URL.RawQuery,
			status,
			time.Since(start).Milliseconds(),
			rec.bytes,
			RequestIDFromContext(req.Context()),
			CorrelationIDFromContext(req.Context()),
			realIP(req),
			req.UserAgent(),
		)
	})
}

func panicRecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf(
					"http_panic method=%s path=%s request_id=%s correlation_id=%s remote_ip=%s panic=%v stack=%q",
					req.Method,
					req.URL.Path,
					RequestIDFromContext(req.Context()),
					CorrelationIDFromContext(req.Context()),
					realIP(req),
					rec,
					string(debug.Stack()),
				)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, req)
	})
}

func logAPIError(req *http.Request, code int, errCode, msg string, err error) {
	if req == nil {
		log.Printf("http_api_error status=%d code=%s message=%q err=%v", code, errCode, msg, err)
		return
	}
	log.Printf(
		"http_api_error method=%s path=%s status=%d code=%s message=%q err=%v request_id=%s correlation_id=%s remote_ip=%s",
		req.Method,
		req.URL.Path,
		code,
		errCode,
		msg,
		err,
		RequestIDFromContext(req.Context()),
		CorrelationIDFromContext(req.Context()),
		realIP(req),
	)
}

func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey).(string)
	return v
}

func CorrelationIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(correlationIDKey).(string)
	return v
}

func realIP(req *http.Request) string {
	if ip := strings.TrimSpace(req.Header.Get("X-Real-IP")); ip != "" {
		return ip
	}
	if ip := strings.TrimSpace(req.Header.Get("X-Forwarded-For")); ip != "" {
		parts := strings.Split(ip, ",")
		return strings.TrimSpace(parts[0])
	}
	return req.RemoteAddr
}
