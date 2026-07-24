package server

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/apolinario0x21/deepwatch/internal/metrics"
)

func TestRecordUptimeStopsOnContextCancel(t *testing.T) {
	m := metrics.New(prometheus.NewRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		recordUptime(ctx, m)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("recordUptime não encerrou após cancelamento do contexto")
	}
}

func TestRecordUptimeIncrements(t *testing.T) {
	m := metrics.New(prometheus.NewRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go recordUptime(ctx, m)

	// Após ~1,2s o contador deve ter incrementado ao menos uma vez.
	time.Sleep(1200 * time.Millisecond)
	if got := testutil.ToFloat64(m.UptimeSeconds); got < 1 {
		t.Errorf("api_uptime_seconds = %v, esperado >= 1", got)
	}
}
