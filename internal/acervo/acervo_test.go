package acervo_test

import (
	"context"
	"errors"
	"testing"

	"timao-api/internal/acervo"
	"timao-api/internal/acervo/acervotest"
	"timao-api/internal/banco"
)

var ctx = context.Background()

func TestPessoas(t *testing.T) {
	ac := acervotest.Novo(t)

	masc, err := ac.Pessoas(ctx, acervo.FiltroPessoas{Categoria: banco.Masculino})
	if err != nil {
		t.Fatal(err)
	}
	if masc.Total != 2 || len(masc.Pessoas) != 2 || masc.Pessoas[0].Nome != "Sócrates" {
		t.Fatalf("masculino: %+v", masc)
	}
	if len(masc.Pessoas[0].Passagens) != 1 || *masc.Pessoas[0].Passagens[0].Jogos != 297 {
		t.Fatalf("passagem do Sócrates: %+v", masc.Pessoas[0].Passagens)
	}

	busca, err := ac.Pessoas(ctx, acervo.FiltroPessoas{Categoria: banco.Masculino, Nome: "SOCRATES"})
	if err != nil || busca.Total != 1 {
		t.Fatalf("busca sem acento deveria achar Sócrates: %+v %v", busca, err)
	}
	tecnicos, err := ac.Pessoas(ctx, acervo.FiltroPessoas{Categoria: banco.Masculino, Funcao: "tecnico"})
	if err != nil || tecnicos.Total != 1 || tecnicos.Pessoas[0].Nome != "Tite" {
		t.Fatalf("técnicos: %+v %v", tecnicos, err)
	}
	fem, err := ac.Pessoas(ctx, acervo.FiltroPessoas{Categoria: banco.Feminino})
	if err != nil || fem.Total != 1 || fem.Pessoas[0].Nome != "Tamires" {
		t.Fatalf("feminino: %+v %v", fem, err)
	}
	pagina, err := ac.Pessoas(ctx, acervo.FiltroPessoas{Categoria: banco.Masculino, Paginacao: acervo.Paginacao{Pagina: 2, Limite: 1}})
	if err != nil || pagina.Total != 2 || len(pagina.Pessoas) != 1 || pagina.Pessoas[0].Nome != "Tite" {
		t.Fatalf("paginação: %+v %v", pagina, err)
	}
}

func TestPessoa(t *testing.T) {
	ac := acervotest.Novo(t)

	p, err := ac.Pessoa(ctx, banco.Masculino, "q1")
	if err != nil {
		t.Fatal(err)
	}
	if p.GolsEmPartidas == nil || *p.GolsEmPartidas != 2 {
		t.Fatalf("Sócrates tem 2 gols nas partidas do acervo, veio %v", p.GolsEmPartidas)
	}
	if p.WikipediaEN != "https://en.wikipedia.org/wiki/S%C3%B3crates" {
		t.Errorf("link da Wikipédia: %s", p.WikipediaEN)
	}
	if _, err := ac.Pessoa(ctx, banco.Feminino, "Q1"); !errors.Is(err, acervo.ErrNaoEncontrado) {
		t.Fatalf("Sócrates não deveria aparecer no feminino: %v", err)
	}

	gols, err := ac.GolsDaPessoa(ctx, banco.Feminino, "Q2")
	if err != nil || len(gols) != 2 || gols[0].Partida.Adversario != "Palmeiras" {
		t.Fatalf("gols da Tamires: %+v %v", gols, err)
	}
}

func TestPartidas(t *testing.T) {
	ac := acervotest.Novo(t)

	todas, err := ac.Partidas(ctx, acervo.FiltroPartidas{Categoria: banco.Masculino})
	if err != nil {
		t.Fatal(err)
	}
	if todas.Total != 3 || todas.Partidas[0].Data != "2025-04-10" {
		t.Fatalf("esperava 3 partidas, mais recente primeiro: %+v", todas)
	}
	want := acervo.Resumo{Jogos: 3, Vitorias: 1, Empates: 1, Derrotas: 1, GolsPro: 3, GolsContra: 4}
	if todas.Resumo != want {
		t.Fatalf("resumo: %+v, esperava %+v", todas.Resumo, want)
	}
	if todas.Partidas[0].Resultado != "empate" || *todas.Partidas[0].Placar.PenaltisCorinthians != 4 {
		t.Fatalf("empate com pênaltis: %+v", todas.Partidas[0])
	}

	casos := []struct {
		nome string
		f    acervo.FiltroPartidas
		want int
	}{
		{"temporada", acervo.FiltroPartidas{Temporada: 2025}, 2},
		{"adversário sem acento", acervo.FiltroPartidas{Adversario: "sao paulo"}, 1},
		{"competição parcial", acervo.FiltroPartidas{Competicao: "paulista"}, 1},
		{"resultado", acervo.FiltroPartidas{Resultado: "derrota"}, 1},
		{"mando", acervo.FiltroPartidas{Mando: "casa"}, 2},
		{"período", acervo.FiltroPartidas{De: "2025-01-01", Ate: "2025-03-31"}, 1},
	}
	for _, c := range casos {
		c.f.Categoria = banco.Masculino
		got, err := ac.Partidas(ctx, c.f)
		if err != nil || got.Total != c.want {
			t.Errorf("%s: total %d, esperava %d (%v)", c.nome, got.Total, c.want, err)
		}
	}

	fem, err := ac.Partidas(ctx, acervo.FiltroPartidas{Categoria: banco.Feminino, Crescente: true})
	if err != nil || fem.Total != 2 || fem.Resumo.Jogos != 1 || fem.Partidas[1].Placar != nil {
		t.Fatalf("feminino (jogo futuro sem placar fora do resumo): %+v %v", fem, err)
	}
}

func TestPartida(t *testing.T) {
	ac := acervotest.Novo(t)

	p, err := ac.Partida(ctx, banco.Masculino, "2025-03-01")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Gols) != 3 || p.Gols[0].Wikidata != "Q1" || p.Gols[0].Tipo != "penalti" || p.Gols[2].Time != "adversario" {
		t.Fatalf("gols: %+v", p.Gols)
	}
	if p.Fonte.Licenca != "CC BY-SA 4.0" {
		t.Errorf("a partida deveria trazer a fonte da Wikipédia: %+v", p.Fonte)
	}
	if _, err := ac.Partida(ctx, banco.Feminino, "2025-03-01"); !errors.Is(err, acervo.ErrNaoEncontrado) {
		t.Fatalf("partida masculina não deveria aparecer no feminino: %v", err)
	}
}

func TestArtilharia(t *testing.T) {
	ac := acervotest.Novo(t)

	masc, err := ac.Artilharia(ctx, acervo.FiltroArtilharia{Categoria: banco.Masculino})
	if err != nil {
		t.Fatal(err)
	}
	if len(masc) != 1 || masc[0].Nome != "Sócrates" || masc[0].Gols != 2 || masc[0].Penaltis != 1 || masc[0].Wikidata != "Q1" {
		t.Fatalf("artilharia masculina (gol contra fora): %+v", masc)
	}
	fem, err := ac.Artilharia(ctx, acervo.FiltroArtilharia{Categoria: banco.Feminino})
	if err != nil || len(fem) != 2 || fem[0].Nome != "Tamires" || fem[0].Gols != 2 {
		t.Fatalf("artilharia feminina: %+v %v", fem, err)
	}
}

func TestTemporadas(t *testing.T) {
	ac := acervotest.Novo(t)

	ts, err := ac.Temporadas(ctx, banco.Masculino)
	if err != nil || len(ts) != 2 || ts[0].Temporada != 2024 || ts[1].Resumo.Jogos != 2 || len(ts[1].Fontes) != 1 {
		t.Fatalf("temporadas: %+v %v", ts, err)
	}
}

func TestTitulos(t *testing.T) {
	ac := acervotest.Novo(t)

	masc, err := ac.Titulos(ctx, acervo.FiltroTitulos{Categoria: banco.Masculino})
	if err != nil {
		t.Fatal(err)
	}
	if len(masc) != 2 || masc[0].Slug != "mundial-de-clubes" || masc[0].Quantidade != 2 || masc[0].Anos[1] != 2012 {
		t.Fatalf("oficiais do masculino, mundial primeiro: %+v", masc)
	}
	todos, err := ac.Titulos(ctx, acervo.FiltroTitulos{Categoria: banco.Masculino, Tipo: "todos"})
	if err != nil || len(todos) != 3 || todos[2].Oficial {
		t.Fatalf("todos, com o não oficial por último: %+v %v", todos, err)
	}
	cont, err := ac.Titulos(ctx, acervo.FiltroTitulos{Categoria: banco.Masculino, Escopo: "continental"})
	if err != nil || len(cont) != 1 || cont[0].Slug != "libertadores" {
		t.Fatalf("filtro por escopo: %+v %v", cont, err)
	}

	lib, err := ac.Titulo(ctx, banco.Feminino, "Libertadores")
	if err != nil {
		t.Fatal(err)
	}
	if lib.Quantidade != 2 || len(lib.Notas) != 1 || lib.Notas[0].Ano != 2017 || len(lib.Edicoes) != 2 {
		t.Fatalf("Libertadores feminina: %+v", lib)
	}
	if _, err := ac.Titulo(ctx, banco.Feminino, "mundial-de-clubes"); !errors.Is(err, acervo.ErrNaoEncontrado) {
		t.Fatalf("o feminino não tem Mundial no banco de teste: %v", err)
	}
}

func TestElenco(t *testing.T) {
	ac := acervotest.Novo(t)

	e, err := ac.Elenco(ctx, banco.Feminino, 0)
	if err != nil {
		t.Fatal(err)
	}
	if e.Temporada != 2025 || len(e.Temporadas) != 2 || len(e.Elenco) != 2 {
		t.Fatalf("elenco mais recente: %+v", e)
	}
	if i := e.Elenco[0]; i.Nome != "Tamires" || i.Wikidata != "Q2" || *i.Numero != 37 || *i.Jogos != 30 || *i.Gols != 4 {
		t.Errorf("com número primeiro e ligada à pessoa: %+v", i)
	}
	if i := e.Elenco[1]; i.Wikidata != "" || i.Numero != nil {
		t.Errorf("sem item no Wikidata: %+v", i)
	}
	if _, err := ac.Elenco(ctx, banco.Feminino, 1990); !errors.Is(err, acervo.ErrNaoEncontrado) {
		t.Fatalf("temporada sem elenco: %v", err)
	}
	if _, err := ac.Elenco(ctx, banco.Masculino, 0); !errors.Is(err, acervo.ErrNaoEncontrado) {
		t.Fatalf("categoria sem elenco: %v", err)
	}

	p, err := ac.Pessoa(ctx, banco.Feminino, "Q2")
	if err != nil || len(p.Elencos) != 2 || p.Elencos[0].Temporada != 2024 || *p.Elencos[1].Numero != 37 {
		t.Fatalf("elencos da Tamires: %+v %v", p.Elencos, err)
	}
}
