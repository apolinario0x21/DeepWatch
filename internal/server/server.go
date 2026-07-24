// Package server compõe as dependências da aplicação (config, métricas,
// handlers e health) em um http.Server e gerencia seu ciclo de vida,
// incluindo o graceful shutdown.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/apolinario0x21/deepwatch/internal/config"
	"github.com/apolinario0x21/deepwatch/internal/handlers"
	"github.com/apolinario0x21/deepwatch/internal/metrics"
)

// Server encapsula o http.Server e as dependências necessárias para o seu
// ciclo de vida.
type Server struct {
	cfg     config.Config
	http    *http.Server
	metrics *metrics.Metrics
	health  *handlers.Health
}

// New constrói o servidor: cria um registrador dedicado, instancia as
// métricas (que já incluem os coletores padrão de Go/processo), os handlers
// e monta as rotas.
func New(cfg config.Config) *Server {
	reg := prometheus.NewRegistry()
	m := metrics.New(reg)
	health := handlers.NewHealth(m.HealthcheckStatus)
	h := handlers.New(m)

	return &Server{
		cfg:     cfg,
		metrics: m,
		health:  health,
		http: &http.Server{
			Addr:              cfg.Addr(),
			Handler:           NewRouter(reg, m, h, health),
			ReadTimeout:       cfg.ReadTimeout,
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
		},
	}
}

// NewRouter monta o roteador com as rotas de negócio (com middleware de
// métricas), o health check e o endpoint /metrics.
func NewRouter(reg *prometheus.Registry, m *metrics.Metrics, h *handlers.Handlers, health *handlers.Health) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", m.Middleware(http.HandlerFunc(h.Home)))
	mux.Handle("/data", m.Middleware(http.HandlerFunc(h.Data)))
	mux.Handle("/error", m.Middleware(http.HandlerFunc(h.Error)))
	mux.Handle("/health", health.Handler())
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	return mux
}

// Run inicia o servidor e bloqueia até receber SIGINT/SIGTERM ou um erro
// fatal, executando então o graceful shutdown.
func (s *Server) Run(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go recordUptime(ctx, s.metrics)

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("aplicação iniciada", slog.String("addr", s.http.Addr))
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
	s.health.Set(false)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
	defer cancel()

	if err := s.http.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown falhou: %w", err)
	}

	slog.Info("aplicação encerrada com sucesso")
	return nil
}

// recordUptime incrementa a métrica de uptime a cada segundo até o contexto
// ser cancelado, encerrando a goroutine de forma limpa no shutdown.
func recordUptime(ctx context.Context, m *metrics.Metrics) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.UptimeSeconds.Inc()
		}
	}
}
