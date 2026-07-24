package handlers

import (
	"net/http"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
)

// Health mantém o estado de saúde da aplicação de forma coerente com a
// métrica exposta: cada transição de estado atualiza a gauge (1=UP, 0=DOWN).
type Health struct {
	up    atomic.Bool
	gauge prometheus.Gauge
}

// NewHealth cria um verificador de saúde já marcado como UP, sincronizando a
// gauge fornecida.
func NewHealth(gauge prometheus.Gauge) *Health {
	h := &Health{gauge: gauge}
	h.Set(true)
	return h
}

// Set atualiza o estado de saúde e a métrica associada.
func (h *Health) Set(up bool) {
	h.up.Store(up)
	if up {
		h.gauge.Set(1)
	} else {
		h.gauge.Set(0)
	}
}

// Healthy reporta se a aplicação está saudável.
func (h *Health) Healthy() bool { return h.up.Load() }

// Handler responde 200/OK quando saudável e 503/DOWN caso contrário.
func (h *Health) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if h.Healthy() {
			w.WriteHeader(http.StatusOK)
			writeText(w, "OK")
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		writeText(w, "DOWN")
	}
}
