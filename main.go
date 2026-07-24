package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/apolinario0x21/deepwatch/internal/config"
)

var (
	apiUptimeSeconds = promauto.NewCounter(prometheus.CounterOpts{
		Name: "api_uptime_seconds",
		Help: "Tempo contínuo de operação da API em segundos.",
	})

	apiHealthcheckStatus = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "api_healthcheck_status",
		Help: "Status atual do health check da API (1 para UP, 0 para DOWN).",
	})

	apiRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "api_request_duration_seconds",
		Help:    "Duração das requisições HTTP em segundos.",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
	}, []string{"method", "path", "status_code"})

	apiExternalDepsDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "api_external_dependencies_duration_seconds",
		Help:    "Duração de chamadas a dependências externas em segundos.",
		Buckets: prometheus.DefBuckets,
	}, []string{"dependency_name"})

	apiRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "api_requests_total",
		Help: "Total de requisições HTTP recebidas.",
	}, []string{"method", "path", "status_code"})

	_ = promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "api_goroutines_count",
		Help: "Número de goroutines atualmente ativas.",
	}, func() float64 {
		return float64(runtime.NumGoroutine())
	})

	_ = promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "api_memory_usage_bytes",
		Help: "Consumo de memória da aplicação (HeapAlloc).",
	}, func() float64 {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return float64(m.Alloc)
	})
)

// health mantém o estado de saúde da aplicação de forma coerente com a
// métrica exposta: sempre que o estado muda, a gauge é atualizada (1/0).
type health struct {
	up atomic.Bool
}

func newHealth() *health {
	h := &health{}
	h.set(true)
	return h
}

func (h *health) set(up bool) {
	h.up.Store(up)
	if up {
		apiHealthcheckStatus.Set(1)
	} else {
		apiHealthcheckStatus.Set(0)
	}
}

func (h *health) healthy() bool { return h.up.Load() }

// recordUptime incrementa a métrica de uptime a cada segundo até o contexto
// ser cancelado, encerrando a goroutine de forma limpa no shutdown.
func recordUptime(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			apiUptimeSeconds.Inc()
		}
	}
}

func callExternalAPI() {
	timer := prometheus.NewTimer(apiExternalDepsDuration.WithLabelValues("external_service"))
	defer timer.ObserveDuration()

	time.Sleep(time.Duration(rand.Intn(150)) * time.Millisecond)
}

// metricsMiddleware captura método, path e status para alimentar os
// contadores e o histograma de duração das requisições.
func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		start := time.Now()
		next.ServeHTTP(rw, r)
		duration := time.Since(start).Seconds()

		method := r.Method
		path := r.URL.Path
		statusCode := strconv.Itoa(rw.statusCode)

		apiRequestsTotal.WithLabelValues(method, path, statusCode).Inc()
		apiRequestDuration.WithLabelValues(method, path, statusCode).Observe(duration)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode  int
	wroteHeader bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.statusCode = code
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(code)
}

func homeHandler(w http.ResponseWriter, _ *http.Request) {
	callExternalAPI()
	if rand.Intn(100) == 0 {
		time.Sleep(600 * time.Millisecond)
	} else {
		time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)
	}

	w.WriteHeader(http.StatusOK)
	writeBody(w, "Hello, World! Simulating p99 latency.")
}

func dataHandler(w http.ResponseWriter, _ *http.Request) {
	time.Sleep(600 * time.Millisecond)
	w.WriteHeader(http.StatusOK)
	writeBody(w, "Data endpoint.")
}

func errorHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
	writeBody(w, "This is an error!")
}

func healthHandler(h *health) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if h.healthy() {
			w.WriteHeader(http.StatusOK)
			writeBody(w, "OK")
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		writeBody(w, "DOWN")
	}
}

func writeBody(w http.ResponseWriter, msg string) {
	if _, err := w.Write([]byte(msg)); err != nil {
		slog.Error("falha ao escrever resposta", slog.String("error", err.Error()))
	}
}

func main() {
	if err := run(); err != nil {
		slog.Error("aplicação encerrada com erro", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	// Contexto cancelado ao receber SIGINT/SIGTERM, disparando o shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	h := newHealth()

	go recordUptime(ctx)

	mux := http.NewServeMux()
	mux.Handle("/", metricsMiddleware(http.HandlerFunc(homeHandler)))
	mux.Handle("/data", metricsMiddleware(http.HandlerFunc(dataHandler)))
	mux.Handle("/error", metricsMiddleware(http.HandlerFunc(errorHandler)))
	mux.Handle("/health", healthHandler(h))
	mux.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           mux,
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	// O servidor roda em goroutine própria; erros fatais viajam pelo canal.
	serverErr := make(chan error, 1)
	go func() {
		slog.Info("aplicação iniciada", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return fmt.Errorf("servidor http: %w", err)
	case <-ctx.Done():
		slog.Info("sinal de encerramento recebido, iniciando graceful shutdown")
	}

	// Marca a aplicação como DOWN para que balanceadores parem de rotear.
	h.set(false)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown falhou: %w", err)
	}

	slog.Info("aplicação encerrada com sucesso")
	return nil
}
