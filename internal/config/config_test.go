package config

import (
	"log/slog"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	// Força as variáveis a vazio para exercitar os valores padrão.
	for _, k := range []string{
		"APP_PORT", "LOG_LEVEL", "APP_READ_TIMEOUT", "APP_READ_HEADER_TIMEOUT",
		"APP_WRITE_TIMEOUT", "APP_IDLE_TIMEOUT", "APP_SHUTDOWN_TIMEOUT",
	} {
		t.Setenv(k, "")
	}

	cfg := Load()

	if cfg.Port != "8080" {
		t.Errorf("Port padrão = %q, esperado 8080", cfg.Port)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel padrão = %v, esperado Info", cfg.LogLevel)
	}
	if cfg.ReadTimeout != 15*time.Second {
		t.Errorf("ReadTimeout padrão = %v, esperado 15s", cfg.ReadTimeout)
	}
	if cfg.ReadHeaderTimeout != 5*time.Second {
		t.Errorf("ReadHeaderTimeout padrão = %v, esperado 5s", cfg.ReadHeaderTimeout)
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Errorf("ShutdownTimeout padrão = %v, esperado 10s", cfg.ShutdownTimeout)
	}
	if cfg.Addr() != ":8080" {
		t.Errorf("Addr() = %q, esperado :8080", cfg.Addr())
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("APP_PORT", "9999")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("APP_READ_TIMEOUT", "30s")
	t.Setenv("APP_SHUTDOWN_TIMEOUT", "2s")

	cfg := Load()

	if cfg.Port != "9999" {
		t.Errorf("Port = %q, esperado 9999", cfg.Port)
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v, esperado Debug", cfg.LogLevel)
	}
	if cfg.ReadTimeout != 30*time.Second {
		t.Errorf("ReadTimeout = %v, esperado 30s", cfg.ReadTimeout)
	}
	if cfg.ShutdownTimeout != 2*time.Second {
		t.Errorf("ShutdownTimeout = %v, esperado 2s", cfg.ShutdownTimeout)
	}
	if cfg.Addr() != ":9999" {
		t.Errorf("Addr() = %q, esperado :9999", cfg.Addr())
	}
}

func TestGetDurationInvalidFallsBack(t *testing.T) {
	t.Setenv("APP_READ_TIMEOUT", "not-a-duration")
	if got := getDuration("APP_READ_TIMEOUT", 7*time.Second); got != 7*time.Second {
		t.Errorf("getDuration inválido = %v, esperado fallback 7s", got)
	}
}

func TestParseLevel(t *testing.T) {
	cases := []struct {
		in   string
		want slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"INFO", slog.LevelInfo},
		{" warn ", slog.LevelWarn}, // whitespace deve ser aparado
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"", slog.LevelInfo},
		{"unknown", slog.LevelInfo},
	}
	for _, tc := range cases {
		if got := parseLevel(tc.in); got != tc.want {
			t.Errorf("parseLevel(%q) = %v, esperado %v", tc.in, got, tc.want)
		}
	}
}
