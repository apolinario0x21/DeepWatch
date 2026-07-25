// Package handlers reúne os handlers HTTP de negócio da aplicação e o
// verificador de saúde. As latências simuladas usam um "sleeper" injetável,
// permitindo testes rápidos e determinísticos.
package handlers

import (
	"log/slog"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/apolinario0x21/deepwatch/internal/metrics"
)

// Handlers agrupa as dependências dos handlers de negócio.
type Handlers struct {
	metrics *metrics.Metrics
	sleep   func(time.Duration)
}

// Option customiza a construção de Handlers.
type Option func(*Handlers)

// WithSleeper substitui a função de espera (padrão time.Sleep). Útil em
// testes para eliminar as latências simuladas.
func WithSleeper(f func(time.Duration)) Option {
	return func(h *Handlers) { h.sleep = f }
}

// New constrói os handlers de negócio a partir das métricas da aplicação.
func New(m *metrics.Metrics, opts ...Option) *Handlers {
	h := &Handlers{metrics: m, sleep: time.Sleep}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// callExternalAPI simula uma chamada a uma dependência externa e registra sua
// duração no histograma correspondente.
func (h *Handlers) callExternalAPI() {
	start := time.Now()
	h.sleep(time.Duration(rand.IntN(150)) * time.Millisecond)
	h.metrics.ObserveExternalDependency("external_service", time.Since(start))
}

// Home é o endpoint principal para testes de carga; simula uma cauda de
// latência (p99) esporádica.
func (h *Handlers) Home(w http.ResponseWriter, _ *http.Request) {
	h.callExternalAPI()
	if rand.IntN(100) == 0 {
		h.sleep(600 * time.Millisecond)
	} else {
		h.sleep(time.Duration(rand.IntN(50)) * time.Millisecond)
	}

	w.WriteHeader(http.StatusOK)
	writeText(w, "Hello, World! Simulating p99 latency.")
}

// Data simula um endpoint com latência controlada.
func (h *Handlers) Data(w http.ResponseWriter, _ *http.Request) {
	h.sleep(600 * time.Millisecond)
	w.WriteHeader(http.StatusOK)
	writeText(w, "Data endpoint.")
}

// Error retorna sempre um erro 500, útil para exercitar o alerta de taxa de
// erros.
func (h *Handlers) Error(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
	writeText(w, "This is an error!")
}

// writeText escreve um corpo de texto simples, registrando eventuais falhas.
func writeText(w http.ResponseWriter, msg string) {
	if _, err := w.Write([]byte(msg)); err != nil {
		slog.Error("falha ao escrever resposta", slog.String("error", err.Error()))
	}
}
