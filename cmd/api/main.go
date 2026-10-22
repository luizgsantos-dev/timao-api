package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // garante o fuso America/Sao_Paulo mesmo em containers sem tzdata

	"timao-api/internal/acervo"
	"timao-api/internal/atualizador"
	"timao-api/internal/brasilapi"
	"timao-api/internal/data"
	"timao-api/internal/importar"
	"timao-api/internal/jogos"
	"timao-api/internal/server"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	addr := env("ADDR", ":8080")
	brasil := brasilapi.New(
		brasilapi.WithBaseURL(env("BRASILAPI_URL", "https://brasilapi.com.br")),
	)

	calendario, err := carregarJogos(logger, env("JOGOS_FILE", ""))
	if err != nil {
		logger.Error("falha ao carregar o calendário de jogos", "erro", err)
		os.Exit(1)
	}

	periodo, err := time.ParseDuration(env("ATUALIZAR_A_CADA", "168h"))
	if err != nil {
		logger.Error("ATUALIZAR_A_CADA inválido; use algo como 168h (uma semana) ou 0 para desligar", "erro", err)
		os.Exit(1)
	}
	ac := acervo.Vazio()
	defer ac.Fechar()
	atual := atualizador.Novo(env("DB_FILE", "timao.db"), periodo, func(ctx context.Context, destino string) error {
		return importar.Executar(ctx, logger, destino, false)
	}, ac, logger)
	if err := atual.Abrir(context.Background()); err != nil {
		logger.Error("falha ao abrir o acervo", "erro", err)
		os.Exit(1)
	}

	api := server.New(data.New(), brasil, calendario, logger).ComAcervo(ac).ComAtualizador(atual)

	srv := &http.Server{
		Addr:              addr,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Encerramento gracioso: espera as requisições em andamento terminarem.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Atualização do acervo em segundo plano (desligada com ATUALIZAR_A_CADA=0).
	go atual.Rodar(ctx)

	done := make(chan struct{})
	go func() {
		defer close(done)
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("falha ao encerrar o servidor", "erro", err)
		}
	}()

	logger.Info("apita o árbitro, bola rolando", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("servidor caiu", "erro", err)
		os.Exit(1)
	}
	<-done
	logger.Info("fim de jogo")
}

// carregarJogos lê o calendário de path. Sem path, usa o calendário de exemplo embutido.
func carregarJogos(logger *slog.Logger, path string) (*jogos.Static, error) {
	if path == "" {
		logger.Warn("usando calendário de EXEMPLO com adversários fictícios; defina JOGOS_FILE com o seu jogos.json")
		return jogos.Exemplo()
	}
	calendario, err := jogos.FromFile(path)
	if err == nil {
		logger.Info("calendário de jogos carregado", "arquivo", path)
	}
	return calendario, err
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
