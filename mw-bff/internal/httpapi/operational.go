package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"regexp"
	"sync/atomic"
	"time"
)

const requestIDHeader = "X-Request-ID"

var safeRequestID = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

type operationalMetrics struct {
	requests   atomic.Uint64
	responses4 atomic.Uint64
	responses5 atomic.Uint64
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func operationalMiddleware(metrics *operationalMetrics, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(requestIDHeader)
		if !safeRequestID.MatchString(requestID) {
			requestID = newRequestID()
		}
		r.Header.Set(requestIDHeader, requestID)
		w.Header().Set(requestIDHeader, requestID)

		metrics.requests.Add(1)
		recorder := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		if status >= 500 {
			metrics.responses5.Add(1)
		} else if status >= 400 {
			metrics.responses4.Add(1)
		}
	})
}

func metricsHandler(service string, metrics *operationalMetrics, started time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"service":             service,
			"uptime_seconds":      int64(time.Since(started).Seconds()),
			"requests_total":      metrics.requests.Load(),
			"responses_4xx_total": metrics.responses4.Load(),
			"responses_5xx_total": metrics.responses5.Load(),
		})
	}
}

func newRequestID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "mw-" + time.Now().UTC().Format("20060102T150405.000000000")
	}
	return "mw-" + hex.EncodeToString(raw[:])
}
