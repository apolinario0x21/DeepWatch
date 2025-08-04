package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	apiGoroutinesCount = promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "api_goroutines_count",
		Help: "Número de goroutines atualmente ativas.",
	}, func() float64 {
		return float64(runtime.NumGoroutine())
	})

	apiMemoryUsageBytes = promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "api_memory_usage_bytes",
		Help: "Consumo de memória da aplicação (HeapAlloc).",
	}, func() float64 {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return float64(m.Alloc)
	})
)

func recordUptime() {
	go func() {
		for {
			apiUptimeSeconds.Inc()
			time.Sleep(1 * time.Second)
		}
	}()
}

func callExternalAPI() {
	timer := prometheus.NewTimer(apiExternalDepsDuration.WithLabelValues("external_service"))
	defer timer.ObserveDuration()

	time.Sleep(time.Duration(rand.Intn(150)) * time.Millisecond)
}

// Middleware para capturar dados: métricas de requisição
func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &responseWriter{w, http.StatusOK}

		start := time.Now()

		next.ServeHTTP(rw, r)

		duration := time.Since(start).Seconds()
		method := r.Method
		path := r.URL.Path
		statusCode := fmt.Sprintf("%d", rw.statusCode)

		apiRequestsTotal.WithLabelValues(method, path, statusCode).Inc()

		apiRequestDuration.WithLabelValues(method, path, statusCode).Observe(duration)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	callExternalAPI()
	if rand.Intn(100) == 0 {
		time.Sleep(600 * time.Millisecond)
	} else {
		time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hello, World! Simulating p99 latency."))
}

func dataHandler(w http.ResponseWriter, r *http.Request) {
	time.Sleep(600 * time.Millisecond)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Data endpoint."))
}

func errorHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("This is an error!"))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	apiHealthcheckStatus.Set(1)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func main() {
	rand.New(rand.NewSource(time.Now().UnixNano()))

	recordUptime()

	mux := http.NewServeMux()

	finalHomeHandler := http.HandlerFunc(homeHandler)
	finalDataHandler := http.HandlerFunc(dataHandler)
	finalErrorHandler := http.HandlerFunc(errorHandler)
	finalHealthHandler := http.HandlerFunc(healthHandler)

	// Aplica o middleware de métricas a TODOS os handlers de negócio
	mux.Handle("/", metricsMiddleware(finalHomeHandler))
	mux.Handle("/data", metricsMiddleware(finalDataHandler))
	mux.Handle("/error", metricsMiddleware(finalErrorHandler))

	// Registra os handlers que NÃO precisam do middleware de métricas de requisição
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/health", finalHealthHandler)

	fmt.Println("Application up and running on port 8080")
	http.ListenAndServe(":8080", mux)
}
