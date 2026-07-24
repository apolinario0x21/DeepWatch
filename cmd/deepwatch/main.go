// Command deepwatch inicializa a aplicação de observabilidade: carrega a
// configuração, configura o log estruturado e delega o ciclo de vida ao
// pacote server.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/apolinario0x21/deepwatch/internal/config"
	"github.com/apolinario0x21/deepwatch/internal/server"
)

func main() {
	cfg := config.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	if err := server.New(cfg).Run(context.Background()); err != nil {
		slog.Error("aplicação encerrada com erro", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
