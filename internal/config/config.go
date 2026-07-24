// Package config carrega a configuração da aplicação a partir de variáveis
// de ambiente, aplicando valores padrão seguros quando ausentes.
package config

import (
	"log/slog"
	"os"
	"strings"
	"time"
)

// Config agrega todos os parâmetros de execução da aplicação.
type Config struct {
	// Port é a porta TCP onde o servidor HTTP escuta (env APP_PORT).
	Port string
	// LogLevel controla o nível mínimo de log estruturado (env LOG_LEVEL).
	LogLevel slog.Level

	// Timeouts do http.Server, protegendo contra conexões lentas (Slowloris).
	ReadTimeout       time.Duration // env APP_READ_TIMEOUT
	ReadHeaderTimeout time.Duration // env APP_READ_HEADER_TIMEOUT
	WriteTimeout      time.Duration // env APP_WRITE_TIMEOUT
	IdleTimeout       time.Duration // env APP_IDLE_TIMEOUT

	// ShutdownTimeout é o prazo máximo para drenar conexões no encerramento.
	ShutdownTimeout time.Duration // env APP_SHUTDOWN_TIMEOUT
}

// Load lê a configuração das variáveis de ambiente, aplicando defaults.
func Load() Config {
	return Config{
		Port:              getEnv("APP_PORT", "8080"),
		LogLevel:          parseLevel(getEnv("LOG_LEVEL", "info")),
		ReadTimeout:       getDuration("APP_READ_TIMEOUT", 15*time.Second),
		ReadHeaderTimeout: getDuration("APP_READ_HEADER_TIMEOUT", 5*time.Second),
		WriteTimeout:      getDuration("APP_WRITE_TIMEOUT", 15*time.Second),
		IdleTimeout:       getDuration("APP_IDLE_TIMEOUT", 60*time.Second),
		ShutdownTimeout:   getDuration("APP_SHUTDOWN_TIMEOUT", 10*time.Second),
	}
}

// Addr retorna o endereço de bind no formato ":porta".
func (c Config) Addr() string {
	return ":" + c.Port
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
		slog.Warn("valor de duração inválido, usando default",
			slog.String("env", key), slog.String("value", v), slog.Duration("default", fallback))
	}
	return fallback
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
