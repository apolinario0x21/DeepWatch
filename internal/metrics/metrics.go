// Package metrics define e registra as métricas Prometheus da aplicação,
// além do middleware HTTP que as alimenta. Os nomes das métricas são
// mantidos estáveis pois são referenciados nas regras de alerta e no
// dashboard do Grafana.
package metrics

import (
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// Metrics agrega os coletores da aplicação. Use New para instanciá-lo com
// um registrador dedicado (facilitando testes isolados).
type Metrics struct {
	UptimeSeconds        prometheus.Counter
	HealthcheckStatus    prometheus.Gauge
	RequestDuration      *prometheus.HistogramVec
	ExternalDepsDuration *prometheus.HistogramVec
	RequestsTotal        *prometheus.CounterVec
}

// New cria os coletores e os registra em reg. Coletores baseados em
// runtime (goroutines e memória) usam GaugeFunc e são registrados aqui,
// junto aos coletores padrão de Go/processo do client_golang.
func New(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		UptimeSeconds: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "api_uptime_seconds",
			Help: "Tempo contínuo de operação da API em segundos.",
		}),
		HealthcheckStatus: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "api_healthcheck_status",
			Help: "Status atual do health check da API (1 para UP, 0 para DOWN).",
		}),
		RequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "api_request_duration_seconds",
			Help:    "Duração das requisições HTTP em segundos.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		}, []string{"method", "path", "status_code"}),
		ExternalDepsDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "api_external_dependencies_duration_seconds",
			Help:    "Duração de chamadas a dependências externas em segundos.",
			Buckets: prometheus.DefBuckets,
		}, []string{"dependency_name"}),
		RequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "api_requests_total",
			Help: "Total de requisições HTTP recebidas.",
		}, []string{"method", "path", "status_code"}),
	}

	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		m.UptimeSeconds,
		m.HealthcheckStatus,
		m.RequestDuration,
		m.ExternalDepsDuration,
		m.RequestsTotal,
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "api_goroutines_count",
			Help: "Número de goroutines atualmente ativas.",
		}, func() float64 {
			return float64(runtime.NumGoroutine())
		}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "api_memory_usage_bytes",
			Help: "Consumo de memória da aplicação (HeapAlloc).",
		}, func() float64 {
			var stats runtime.MemStats
			runtime.ReadMemStats(&stats)
			return float64(stats.Alloc)
		}),
	)

	return m
}

// ObserveExternalDependency mede a duração de uma chamada a uma dependência
// externa identificada por name.
func (m *Metrics) ObserveExternalDependency(name string, d time.Duration) {
	m.ExternalDepsDuration.WithLabelValues(name).Observe(d.Seconds())
}

// Middleware captura método, path e status code de cada requisição,
// alimentando os contadores e o histograma de duração.
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		start := time.Now()
		next.ServeHTTP(rw, r)
		duration := time.Since(start).Seconds()

		labels := []string{r.Method, r.URL.Path, strconv.Itoa(rw.statusCode)}
		m.RequestsTotal.WithLabelValues(labels...).Inc()
		m.RequestDuration.WithLabelValues(labels...).Observe(duration)
	})
}

// responseWriter intercepta o status code escrito pelo handler para que o
// middleware possa registrá-lo nas métricas.
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

// Write garante que uma resposta sem WriteHeader explícito seja contabilizada
// como 200 (comportamento padrão do net/http).
func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}
