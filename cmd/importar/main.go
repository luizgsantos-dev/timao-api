// Comando importar gera o timao.db a partir de fontes abertas (veja o pacote
// internal/importar). A API também atualiza o banco sozinha, uma vez por semana
// (veja ATUALIZAR_A_CADA no README); este comando serve para gerar à mão.
//
//	go run ./cmd/importar                 # gera ./timao.db
//	go run ./cmd/importar -saida dados.db -v
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"

	"timao-api/internal/importar"
)

func main() {
	saida := flag.String("saida", "timao.db", "arquivo SQLite a gerar")
	detalhes := flag.Bool("v", false, "mostra os avisos de cada fonte (jogos ignorados, datas corrigidas, placares divergentes)")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := importar.Executar(ctx, logger, *saida, *detalhes); err != nil {
		logger.Error("importação falhou", "erro", err)
		os.Exit(1)
	}
}
