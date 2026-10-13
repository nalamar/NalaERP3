package apihttp

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"runtime/debug"
	"sort"
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

		queryKeys, queryOther := loggableQueryKeys(req.URL.RawQuery)
		log.Printf(
			"http_request method=%s path=%s query_keys=%q query_other=%d status=%d duration_ms=%d bytes=%d request_id=%s correlation_id=%s remote_ip=%s user_agent=%q",
			req.Method,
			req.URL.Path,
			queryKeys,
			queryOther,
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

// loggedQueryKeys sind die Namen der Query-Parameter, die im Request-Log
// erscheinen duerfen (ADR 0026). Geloggt werden ausschliesslich NAMEN aus
// dieser Liste - nie ein Wert.
//
// Werte tragen personenbezogene Daten: der Suchbegriff q ist bei Kontakten
// ein Name, employee_id zeigt auf einen Mitarbeiter. Und auch ein NAME ist
// nicht automatisch harmlos: bei "?Mueller" ist "Mueller" der Name eines
// Parameters ohne Wert. Deshalb eine feste Liste statt "alle Namen".
//
// Fehlt hier ein neuer Parameter, ist das kein Leck, nur weniger Auskunft:
// er wird dann unter query_other mitgezaehlt statt genannt.
var loggedQueryKeys = map[string]bool{
	"action": true, "bestellung_id": true, "bis": true, "contact_id": true, "datum": true,
	"employee_id": true, "format": true, "from": true, "include_inactive": true,
	"include_storniert": true, "invoice_type": true, "kategorie": true, "kind": true,
	"lieferant_id": true, "limit": true, "material_id": true, "menge": true,
	"min_length_mm": true, "offset": true, "path": true, "project_id": true, "q": true,
	"quote_id": true, "rolle": true, "source_sales_order_id": true, "status": true,
	"summary": true, "supplier_id": true, "to": true, "typ": true, "von": true,
	"warehouse_id": true,
}

// loggableQueryKeys liefert fuer das Request-Log die bekannten
// Parameternamen eines Query-Strings (sortiert, je einmal, durch Komma
// getrennt) und die Anzahl der uebrigen Parameter.
func loggableQueryKeys(rawQuery string) (string, int) {
	if rawQuery == "" {
		return "", 0
	}
	// Ein fehlerhafter Query-String liefert, was sich lesen liess; der
	// Rest zaehlt als ein unbekannter Parameter. Der Rohtext selbst wird
	// in keinem Fall geloggt.
	values, err := url.ParseQuery(rawQuery)
	other := 0
	if err != nil {
		other++
	}
	known := make([]string, 0, len(values))
	for key := range values {
		if loggedQueryKeys[key] {
			known = append(known, key)
		} else {
			other++
		}
	}
	sort.Strings(known)
	return strings.Join(known, ","), other
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
