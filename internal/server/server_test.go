package server_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/apolinario0x21/deepwatch/internal/config"
	"github.com/apolinario0x21/deepwatch/internal/handlers"
	"github.com/apolinario0x21/deepwatch/internal/metrics"
	"github.com/apolinario0x21/deepwatch/internal/server"
)

func TestRouterRoutes(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.New(reg)
	health := handlers.NewHealth(m.HealthcheckStatus)
	h := handlers.New(m, handlers.WithSleeper(func(time.Duration) {}))
	router := server.NewRouter(reg, m, h, health)

	cases := []struct {
		path       string
		wantStatus int
		wantBody   string
	}{
		{"/error", http.StatusInternalServerError, "This is an error!"},
		{"/health", http.StatusOK, "OK"},
		{"/data", http.StatusOK, "Data endpoint."},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if rec.Code != tc.wantStatus {
			t.Errorf("%s: status = %d, esperado %d", tc.path, rec.Code, tc.wantStatus)
		}
		if !strings.Contains(rec.Body.String(), tc.wantBody) {
			t.Errorf("%s: corpo = %q, esperado conter %q", tc.path, rec.Body.String(), tc.wantBody)
		}
	}

	// /metrics deve expor as métricas registradas após o tráfego acima.
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/metrics: status = %d, esperado 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"api_requests_total", "api_healthcheck_status", "go_goroutines"} {
		if !strings.Contains(body, want) {
			t.Errorf("/metrics não contém %q", want)
		}
	}
}

func TestRunGracefulShutdown(t *testing.T) {
	cfg := config.Load()
	cfg.Port = strconv.Itoa(freePort(t))
	cfg.ShutdownTimeout = 2 * time.Second

	srv := server.New(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Run(ctx) }()

	// Aguarda o servidor aceitar conexões.
	base := "http://127.0.0.1:" + cfg.Port
	waitReady(t, base+"/health")

	// Cancela o contexto (equivalente a receber SIGTERM) e valida o encerramento limpo.
	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run retornou erro no shutdown: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run não encerrou dentro do prazo")
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("não foi possível obter porta livre: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func waitReady(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url) //nolint:noctx // teste local simples
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("servidor não ficou pronto a tempo")
}
