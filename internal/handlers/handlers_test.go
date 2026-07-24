package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/apolinario0x21/deepwatch/internal/handlers"
	"github.com/apolinario0x21/deepwatch/internal/metrics"
)

// noSleep elimina as latências simuladas para manter os testes rápidos.
func noSleep(_ time.Duration) {}

func newHandlers(t *testing.T) (*handlers.Handlers, *metrics.Metrics) {
	t.Helper()
	m := metrics.New(prometheus.NewRegistry())
	return handlers.New(m, handlers.WithSleeper(noSleep)), m
}

func TestHome(t *testing.T) {
	h, m := newHandlers(t)

	rec := httptest.NewRecorder()
	h.Home(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, esperado 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Hello, World!") {
		t.Errorf("corpo inesperado: %q", rec.Body.String())
	}
	// Home deve registrar uma observação de dependência externa.
	if got := testutil.CollectAndCount(m.ExternalDepsDuration); got != 1 {
		t.Errorf("dependências externas observadas = %d, esperado 1", got)
	}
}

func TestData(t *testing.T) {
	h, _ := newHandlers(t)

	rec := httptest.NewRecorder()
	h.Data(rec, httptest.NewRequest(http.MethodGet, "/data", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, esperado 200", rec.Code)
	}
	if rec.Body.String() != "Data endpoint." {
		t.Errorf("corpo = %q, esperado 'Data endpoint.'", rec.Body.String())
	}
}

func TestError(t *testing.T) {
	h, _ := newHandlers(t)

	rec := httptest.NewRecorder()
	h.Error(rec, httptest.NewRequest(http.MethodGet, "/error", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, esperado 500", rec.Code)
	}
	if rec.Body.String() != "This is an error!" {
		t.Errorf("corpo = %q, esperado 'This is an error!'", rec.Body.String())
	}
}

func TestHealthUpAndDown(t *testing.T) {
	m := metrics.New(prometheus.NewRegistry())
	health := handlers.NewHealth(m.HealthcheckStatus)
	handler := health.Handler()

	// Estado inicial: UP.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "OK" {
		t.Errorf("UP: status=%d body=%q, esperado 200/OK", rec.Code, rec.Body.String())
	}
	if got := testutil.ToFloat64(m.HealthcheckStatus); got != 1 {
		t.Errorf("gauge = %v, esperado 1 quando UP", got)
	}

	// Transição para DOWN.
	health.Set(false)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusServiceUnavailable || rec.Body.String() != "DOWN" {
		t.Errorf("DOWN: status=%d body=%q, esperado 503/DOWN", rec.Code, rec.Body.String())
	}
	if got := testutil.ToFloat64(m.HealthcheckStatus); got != 0 {
		t.Errorf("gauge = %v, esperado 0 quando DOWN", got)
	}
	if health.Healthy() {
		t.Error("Healthy() = true após Set(false)")
	}
}
