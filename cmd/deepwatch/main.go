// Command deepwatch inicializa a aplicação de observabilidade: carrega a
// configuração, configura o log estruturado e delega o ciclo de vida ao
// pacote server. Com a flag -healthcheck, executa uma verificação de saúde
// e sai (usada pelo HEALTHCHECK do Docker em imagem sem shell).
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/apolinario0x21/deepwatch/internal/config"
	"github.com/apolinario0x21/deepwatch/internal/server"
)

// version é injetada em tempo de build via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	healthcheck := flag.Bool("healthcheck", false, "executa uma verificação de saúde e sai (código 0 se OK)")
	flag.Parse()

	cfg := config.Load()

	if *healthcheck {
		if err := runHealthcheck(cfg.Port); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)
	slog.Info("iniciando DeepWatch", slog.String("version", version))

	if err := server.New(cfg).Run(context.Background()); err != nil {
		slog.Error("aplicação encerrada com erro", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

// runHealthcheck faz uma requisição ao endpoint /health local e retorna erro
// se a aplicação não estiver saudável.
func runHealthcheck(port string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/health")
	if err != nil {
		return fmt.Errorf("healthcheck falhou: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck: status inesperado %d", resp.StatusCode)
	}
	return nil
}
