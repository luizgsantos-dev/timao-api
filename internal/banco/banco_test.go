package banco

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCriarEAbrir(t *testing.T) {
	ctx := context.Background()
	destino := filepath.Join(t.TempDir(), "timao.db")

	esc, err := Criar(ctx, destino)
	if err != nil {
		t.Fatal(err)
	}
	fonte, err := esc.Fonte(ctx, Fonte{Nome: "Teste", URL: "https://exemplo", Licenca: "CC0", ColetadoEm: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	pessoa, err := esc.Pessoa(ctx, Pessoa{Wikidata: "Q1", Nome: "Sócrates", Genero: Masculino}, fonte)
	if err != nil {
		t.Fatal(err)
	}
	jogos := 297
	if err := esc.Passagem(ctx, pessoa, Passagem{Categoria: Masculino, Funcao: "jogador", Inicio: "1978", Jogos: &jogos}, fonte); err != nil {
		t.Fatal(err)
	}
	titulo := Titulo{Categoria: Masculino, Competicao: "Campeonato Paulista", Slug: "paulista", Escopo: "estadual", Oficial: true, Ano: 1982}
	if novo, err := esc.Titulo(ctx, titulo, fonte); err != nil || !novo {
		t.Fatalf("título: novo=%v err=%v", novo, err)
	}
	if novo, err := esc.Titulo(ctx, titulo, fonte); err != nil || novo {
		t.Fatalf("título repetido deveria ser ignorado: novo=%v err=%v", novo, err)
	}

	pro, contra := 2, 1
	partida := Partida{Categoria: Feminino, Temporada: 2025, Data: "2025-03-01", Adversario: "Palmeiras", Mando: "casa",
		GolsPro: &pro, GolsContra: &contra,
		Gols: []Gol{{Lado: "pro", Autor: "Gabi Zanotti", Minuto: "10", Tipo: "normal"}}}
	if novo, err := esc.Partida(ctx, partida, fonte); err != nil || !novo {
		t.Fatalf("primeira gravação: novo=%v err=%v", novo, err)
	}
	if novo, err := esc.Partida(ctx, partida, fonte); err != nil || novo {
		t.Fatalf("partida repetida deveria ser ignorada: novo=%v err=%v", novo, err)
	}

	vizinhas, err := esc.PartidasPerto(ctx, Feminino, "2025-03-04", 3)
	if err != nil || len(vizinhas) != 1 || vizinhas[0].Adversario != "Palmeiras" || *vizinhas[0].GolsPro != 2 {
		t.Fatalf("PartidasPerto a 3 dias: %+v %v", vizinhas, err)
	}
	if vizinhas, err := esc.PartidasPerto(ctx, Feminino, "2025-03-05", 3); err != nil || len(vizinhas) != 0 {
		t.Fatalf("PartidasPerto a 4 dias não deveria achar: %+v %v", vizinhas, err)
	}

	if _, err := os.Stat(destino); !os.IsNotExist(err) {
		t.Fatal("o destino só deveria existir depois de Concluir")
	}
	if err := esc.Concluir(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}

	db, err := Abrir(destino)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var nPartidas, nGols int
	if err := db.QueryRow(`SELECT (SELECT COUNT(*) FROM partida), (SELECT COUNT(*) FROM gol)`).Scan(&nPartidas, &nGols); err != nil {
		t.Fatal(err)
	}
	if nPartidas != 1 || nGols != 1 {
		t.Fatalf("esperava 1 partida e 1 gol, veio %d e %d", nPartidas, nGols)
	}
	if _, err := db.Exec(`DELETE FROM gol`); err == nil {
		t.Fatal("o banco deveria abrir só para leitura")
	}
}

func TestDescartarMantemBancoAnterior(t *testing.T) {
	ctx := context.Background()
	destino := filepath.Join(t.TempDir(), "timao.db")
	if err := os.WriteFile(destino, []byte("anterior"), 0o644); err != nil {
		t.Fatal(err)
	}

	esc, err := Criar(ctx, destino)
	if err != nil {
		t.Fatal(err)
	}
	esc.Descartar()

	b, err := os.ReadFile(destino)
	if err != nil || string(b) != "anterior" {
		t.Fatalf("o banco anterior deveria continuar intacto: %q %v", b, err)
	}
	if _, err := os.Stat(destino + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("o arquivo temporário deveria ter sido apagado")
	}
}

func TestMesmoAdversario(t *testing.T) {
	casos := []struct {
		a, b string
		want bool
	}{
		{"CA Mineiro", "Atlético Mineiro", true},
		{"Grêmio FBPA", "Grêmio", true},
		{"EC Vitória", "Vitória", true},
		{"Atlético Mineiro", "Athletico Paranaense", false},
		{"São Paulo", "São Bento", false},
		{"Santos", "Palmeiras", false},
	}
	for _, c := range casos {
		if got := MesmoAdversario(c.a, c.b); got != c.want {
			t.Errorf("MesmoAdversario(%q, %q) = %v, esperava %v", c.a, c.b, got, c.want)
		}
	}
}

func TestEnriquecer(t *testing.T) {
	ctx := context.Background()
	esc, err := Criar(ctx, filepath.Join(t.TempDir(), "timao.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer esc.Descartar()
	fonte, _ := esc.Fonte(ctx, Fonte{Nome: "t", URL: "u", Licenca: "l", ColetadoEm: time.Now()})
	n := func(v int) *int { return &v }
	if _, err := esc.Partida(ctx, Partida{Categoria: Masculino, Temporada: 2005, Data: "2005-05-01", Adversario: "Santos",
		Mando: "casa", GolsPro: n(2), GolsContra: n(1), Estadio: "Pacaembu"}, fonte); err != nil {
		t.Fatal(err)
	}
	vs, _ := esc.PartidasPerto(ctx, Masculino, "2005-05-01", 0)
	id := vs[0].ID

	comGols := Partida{GolsPro: n(2), GolsContra: n(1), Estadio: "Outro", Arbitro: "Fulano", Gols: []Gol{
		{Lado: "pro", Autor: "Tevez", Minuto: "10", Tipo: "normal"},
		{Lado: "pro", Autor: "Nilmar", Minuto: "50", Tipo: "normal"},
		{Lado: "contra", Autor: "Robinho", Minuto: "80", Tipo: "penalti"},
	}}
	if mudou, err := esc.Enriquecer(ctx, id, comGols); err != nil || !mudou {
		t.Fatalf("primeira vez deveria mudar: %v %v", mudou, err)
	}
	if mudou, err := esc.Enriquecer(ctx, id, comGols); err != nil || mudou {
		t.Fatalf("segunda vez não deveria mudar nada (gols e árbitro já existem): %v %v", mudou, err)
	}
	var estadio, arbitro string
	var gols int
	_ = esc.tx.QueryRowContext(ctx, `SELECT estadio, arbitro, (SELECT COUNT(*) FROM gol) FROM partida WHERE id = ?`, id).Scan(&estadio, &arbitro, &gols)
	if estadio != "Pacaembu" || arbitro != "Fulano" || gols != 3 {
		t.Fatalf("não sobrescreve o estádio, preenche o árbitro e grava 3 gols: %q %q %d", estadio, arbitro, gols)
	}

	// Gols que não somam o placar não entram.
	if _, err := esc.Partida(ctx, Partida{Categoria: Masculino, Temporada: 2005, Data: "2005-06-01", Adversario: "Palmeiras",
		Mando: "fora", GolsPro: n(1), GolsContra: n(0)}, fonte); err != nil {
		t.Fatal(err)
	}
	vs, _ = esc.PartidasPerto(ctx, Masculino, "2005-06-01", 0)
	incompleto := Partida{GolsPro: n(1), GolsContra: n(0), Gols: []Gol{}}
	if mudou, _ := esc.Enriquecer(ctx, vs[0].ID, incompleto); mudou {
		t.Fatal("sem gols nem campos novos, não deveria mudar")
	}
}
