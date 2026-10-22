package atualizador

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"timao-api/internal/acervo"
	"timao-api/internal/banco"
)

var ctx = context.Background()

func init() { acervo.EsperaParaFechar = 0 }

// gerar grava em path um banco com n partidas, gerado em quando.
func gerar(t *testing.T, path string, n int, quando time.Time) {
	t.Helper()
	esc, err := banco.Criar(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	fonte, err := esc.Fonte(ctx, banco.Fonte{Nome: "teste", URL: "u", Licenca: "CC0", ColetadoEm: quando})
	if err != nil {
		t.Fatal(err)
	}
	inicio := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		p := banco.Partida{Categoria: banco.Masculino, Temporada: 2025, Data: inicio.AddDate(0, 0, 3*i).Format("2006-01-02"),
			Adversario: fmt.Sprintf("Time %d", i), Mando: "casa"}
		if _, err := esc.Partida(ctx, p, fonte); err != nil {
			t.Fatal(err)
		}
	}
	if err := esc.Concluir(ctx, quando); err != nil {
		t.Fatal(err)
	}
}

func novo(t *testing.T, imp Importador) (*Atualizador, string) {
	t.Helper()
	base := filepath.Join(t.TempDir(), "timao.db")
	ac := acervo.Vazio()
	t.Cleanup(func() { ac.Fechar() })
	a := Novo(base, time.Hour, imp, ac, slog.New(slog.NewTextHandler(io.Discard, nil)))
	a.Agora = func() time.Time { return time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC) }
	return a, base
}

func partidas(t *testing.T, ac *acervo.Acervo) int {
	t.Helper()
	c, err := ac.Contagens(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return c["partidas masculino"]
}

func TestAbrirEscolheOMaisRecente(t *testing.T) {
	a, base := novo(t, nil)
	gerar(t, base, 5, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
	recente := a.geracao(time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC))
	gerar(t, recente, 7, time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC))
	if err := os.WriteFile(a.geracao(time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)), []byte("corrompido"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := a.Abrir(ctx); err != nil {
		t.Fatal(err)
	}
	if s := a.Status(); s.Arquivo != recente || !s.Disponivel || s.GeradoEm.Day() != 15 {
		t.Fatalf("deveria abrir a geração de 15/10 e ignorar o arquivo corrompido: %+v", s)
	}
	if n := partidas(t, a.Acervo); n != 7 {
		t.Fatalf("esperava o banco com 7 partidas, veio %d", n)
	}
}

func TestAbrirSemBanco(t *testing.T) {
	a, _ := novo(t, nil)
	if err := a.Abrir(ctx); err != nil {
		t.Fatal(err)
	}
	if a.Status().Disponivel {
		t.Fatal("sem banco, o acervo não deveria estar disponível")
	}
}

func TestAtualizarTrocaELimpa(t *testing.T) {
	a, base := novo(t, func(ctx context.Context, destino string) error {
		gerar(t, destino, 10, time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC))
		return nil
	})
	gerar(t, base, 10, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
	velha := a.geracao(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
	gerar(t, velha, 3, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
	if err := a.Abrir(ctx); err != nil {
		t.Fatal(err)
	}

	if err := a.Atualizar(ctx); err != nil {
		t.Fatal(err)
	}
	s := a.Status()
	if s.Arquivo != a.geracao(a.Agora()) || s.UltimoErro != "" || s.Atualizando || s.UltimaTentativa == nil {
		t.Fatalf("status depois de atualizar: %+v", s)
	}
	if _, err := os.Stat(velha); !os.IsNotExist(err) {
		t.Error("a geração antiga deveria ter sido apagada")
	}
	if _, err := os.Stat(base); err != nil {
		t.Error("o banco principal nunca deveria ser apagado")
	}
}

func TestAtualizarRecusaBancoComMenosDados(t *testing.T) {
	a, base := novo(t, func(ctx context.Context, destino string) error {
		gerar(t, destino, 8, time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)) // 8 de 10: abaixo de 90%
		return nil
	})
	gerar(t, base, 10, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
	if err := a.Abrir(ctx); err != nil {
		t.Fatal(err)
	}

	err := a.Atualizar(ctx)
	if err == nil || !strings.Contains(err.Error(), "partidas masculino: 10 -> 8") {
		t.Fatalf("esperava recusa pela queda de partidas, veio %v", err)
	}
	if n := partidas(t, a.Acervo); n != 10 {
		t.Fatalf("o banco antigo deveria continuar em uso, veio %d partidas", n)
	}
	if _, err := os.Stat(a.geracao(a.Agora())); !os.IsNotExist(err) {
		t.Error("o banco recusado deveria ter sido apagado")
	}
	if a.Status().UltimoErro == "" {
		t.Error("o status deveria trazer o erro")
	}
}

func TestAtualizarComFalhaNaImportacao(t *testing.T) {
	a, _ := novo(t, func(ctx context.Context, destino string) error {
		os.WriteFile(destino, []byte("pela metade"), 0o644)
		return errors.New("Wikipédia fora do ar")
	})
	if err := a.Atualizar(ctx); err == nil || !strings.Contains(err.Error(), "fora do ar") {
		t.Fatalf("esperava o erro da importação, veio %v", err)
	}
	if _, err := os.Stat(a.geracao(a.Agora())); !os.IsNotExist(err) {
		t.Error("o arquivo da importação que falhou deveria ter sido apagado")
	}
}

func TestRodarImportaLogoSemBanco(t *testing.T) {
	chamou := make(chan struct{}, 1)
	a, _ := novo(t, func(ctx context.Context, destino string) error {
		gerar(t, destino, 2, time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC))
		chamou <- struct{}{}
		return nil
	})
	a.Agora = time.Now
	c, cancelar := context.WithCancel(ctx)
	fim := make(chan struct{})
	go func() { a.Rodar(c); close(fim) }()

	select {
	case <-chamou:
	case <-time.After(10 * time.Second):
		t.Fatal("sem banco, a primeira importação deveria rodar na hora")
	}
	// A importação terminou; espera a troca do banco e o agendamento da próxima (daqui a 1h).
	agendada := func(s Status) bool {
		return s.Disponivel && s.ProximaAtualizacao != nil && time.Until(*s.ProximaAtualizacao) > 50*time.Minute
	}
	for limite := time.Now().Add(10 * time.Second); !agendada(a.Status()); {
		if time.Now().After(limite) {
			t.Fatalf("depois da primeira importação, o acervo deveria ficar disponível e a próxima ser agendada: %+v", a.Status())
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancelar()
	select {
	case <-fim:
	case <-time.After(10 * time.Second):
		t.Fatal("Rodar deveria parar quando o contexto termina")
	}
}

func TestRodarDesligado(t *testing.T) {
	a, _ := novo(t, func(ctx context.Context, destino string) error {
		t.Fatal("com Periodo 0, não deveria importar")
		return nil
	})
	a.Periodo = 0
	a.Rodar(ctx) // volta na hora
	if a.Status().AtualizacaoLigada {
		t.Fatal("o status deveria dizer que a atualização está desligada")
	}
}
