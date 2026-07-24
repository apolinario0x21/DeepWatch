package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func newTestMetrics(t *testing.T) *Metrics {
	t.Helper()
	return New(prometheus.NewRegistry())
}

func TestMiddlewareRecordsStatusAndCounts(t *testing.T) {
	m := newTestMetrics(t)

	h := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("nope"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, esperado 404", rec.Code)
	}
	if got := testutil.ToFloat64(m.RequestsTotal.WithLabelValues("GET", "/missing", "404")); got != 1 {
		t.Errorf("api_requests_total{404} = %v, esperado 1", got)
	}
	if got := testutil.CollectAndCount(m.RequestDuration); got != 1 {
		t.Errorf("séries de api_request_duration_seconds = %d, esperado 1", got)
	}
}

func TestMiddlewareDefaultsToStatus200(t *testing.T) {
	m := newTestMetrics(t)

	// Handler que só escreve o corpo, sem WriteHeader explícito.
	h := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := testutil.ToFloat64(m.RequestsTotal.WithLabelValues("POST", "/", "200")); got != 1 {
		t.Errorf("api_requests_total{200} = %v, esperado 1", got)
	}
}

func TestResponseWriterIgnoresSecondWriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

	rw.WriteHeader(http.StatusInternalServerError)
	rw.WriteHeader(http.StatusOK) // deve ser ignorado

	if rw.statusCode != http.StatusInternalServerError {
		t.Errorf("statusCode = %d, esperado 500", rw.statusCode)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("recorder.Code = %d, esperado 500", rec.Code)
	}
}

func TestObserveExternalDependency(t *testing.T) {
	m := newTestMetrics(t)

	m.ObserveExternalDependency("external_service", 5*time.Millisecond)

	if got := testutil.CollectAndCount(m.ExternalDepsDuration); got != 1 {
		t.Errorf("séries de api_external_dependencies_duration_seconds = %d, esperado 1", got)
	}
}
