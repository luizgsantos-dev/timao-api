// Package acervotest monta um timao.db pequeno e conhecido para os testes.
package acervotest

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"timao-api/internal/acervo"
	"timao-api/internal/banco"
)

func n(v int) *int { return &v }

// Novo gera o banco de teste num diretório temporário e devolve o acervo aberto.
//
// Conteúdo:
//   - Masculino: Sócrates (Q1, jogador), Tite (Q3, técnico); partidas em
//     2025-03-01 (2 x 1 Palmeiras, casa), 2025-04-10 (1 x 1 São Paulo, fora,
//     4 x 3 nos pênaltis) e 2024-11-20 (0 x 2 Santos, casa).
//   - Feminino: Tamires (Q2, jogadora); partidas em 2025-05-05 (3 x 0 Palmeiras)
//     e 2026-12-01 (jogo futuro, sem placar).
//   - Títulos oficiais: Mundial 2000 e 2012 e Libertadores 2012 (masculino);
//     Libertadores 2017 (com nota) e 2019 (feminino). Não oficial: Torneio Início 1919 (masculino).
func Novo(t testing.TB) *acervo.Acervo {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "timao.db")

	esc, err := banco.Criar(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			esc.Descartar()
			t.Fatal(err)
		}
	}
	fonte, err := esc.Fonte(ctx, banco.Fonte{Nome: "Teste", URL: "https://exemplo/wd", Licenca: "CC0", ColetadoEm: time.Now()})
	must(err)
	wiki, err := esc.Fonte(ctx, banco.Fonte{Nome: "Wikipédia (en): teste", URL: "https://exemplo/wp", Licenca: "CC BY-SA 4.0", ColetadoEm: time.Now()})
	must(err)

	pessoa := func(p banco.Pessoa, pas ...banco.Passagem) int64 {
		id, err := esc.Pessoa(ctx, p, fonte)
		must(err)
		for _, pa := range pas {
			must(esc.Passagem(ctx, id, pa, fonte))
		}
		return id
	}
	pessoa(banco.Pessoa{Wikidata: "Q1", Nome: "Sócrates", Genero: banco.Masculino, Posicao: "meia",
		Nacionalidade: "Brasil", WikiEN: "Sócrates", WikiPT: "Sócrates"},
		banco.Passagem{Categoria: banco.Masculino, Funcao: "jogador", Inicio: "1978", Fim: "1984", Jogos: n(297), Gols: n(172)})
	idTamires := pessoa(banco.Pessoa{Wikidata: "Q2", Nome: "Tamires", Genero: banco.Feminino, Posicao: "lateral", WikiEN: "Tamires"},
		banco.Passagem{Categoria: banco.Feminino, Funcao: "jogador", Inicio: "2017"})
	pessoa(banco.Pessoa{Wikidata: "Q3", Nome: "Tite", Genero: banco.Masculino},
		banco.Passagem{Categoria: banco.Masculino, Funcao: "tecnico", Inicio: "2010", Fim: "2013"})

	partida := func(p banco.Partida) {
		_, err := esc.Partida(ctx, p, wiki)
		must(err)
	}
	partida(banco.Partida{Categoria: banco.Masculino, Temporada: 2025, Data: "2025-03-01", Competicao: "Campeonato Paulista",
		Fase: "Final", Adversario: "Palmeiras", Mando: "casa", GolsPro: n(2), GolsContra: n(1), Estadio: "Neo Química Arena", Publico: n(45000),
		Gols: []banco.Gol{
			{Lado: "pro", Autor: "Sócrates", AutorWikiEN: "Sócrates", Minuto: "10", Tipo: "penalti"},
			{Lado: "pro", Autor: "Zagueiro", Minuto: "50", Tipo: "contra"},
			{Lado: "contra", Autor: "Fulano", Minuto: "80", Tipo: "normal"},
		}})
	partida(banco.Partida{Categoria: banco.Masculino, Temporada: 2025, Data: "2025-04-10", Competicao: "Copa do Brasil",
		Adversario: "São Paulo", Mando: "fora", GolsPro: n(1), GolsContra: n(1), PenaltisPro: n(4), PenaltisContra: n(3),
		Gols: []banco.Gol{
			{Lado: "pro", Autor: "Sócrates", AutorWikiEN: "Sócrates", Minuto: "30", Tipo: "normal"},
			{Lado: "contra", Autor: "Beltrano", Minuto: "60", Tipo: "normal"},
		}})
	partida(banco.Partida{Categoria: banco.Masculino, Temporada: 2024, Data: "2024-11-20", Competicao: "Campeonato Brasileiro",
		Adversario: "Santos", Mando: "casa", GolsPro: n(0), GolsContra: n(2)})
	partida(banco.Partida{Categoria: banco.Feminino, Temporada: 2025, Data: "2025-05-05", Competicao: "Campeonato Brasileiro",
		Adversario: "Palmeiras", Mando: "casa", GolsPro: n(3), GolsContra: n(0),
		Gols: []banco.Gol{
			// Fora de ordem de propósito: a API devolve por minuto.
			{Lado: "pro", Autor: "Gabi Zanotti", Minuto: "90+1", Tipo: "normal"},
			{Lado: "pro", Autor: "Tamires", AutorWikiEN: "Tamires", Minuto: "15", Tipo: "normal"},
			{Lado: "pro", Autor: "Tamires", AutorWikiEN: "Tamires", Minuto: "5", Tipo: "normal"},
		}})
	partida(banco.Partida{Categoria: banco.Feminino, Temporada: 2026, Data: "2026-12-01", Competicao: "Copa Libertadores",
		Adversario: "Boca Juniors", Mando: "fora"})

	titulo := func(t banco.Titulo) {
		_, err := esc.Titulo(ctx, t, wiki)
		must(err)
	}
	for _, ano := range []int{2000, 2012} {
		titulo(banco.Titulo{Categoria: banco.Masculino, Competicao: "Mundial de Clubes da FIFA", Slug: "mundial-de-clubes",
			Escopo: "mundial", Oficial: true, Ano: ano})
	}
	titulo(banco.Titulo{Categoria: banco.Masculino, Competicao: "Copa Libertadores", Slug: "libertadores",
		Escopo: "continental", Oficial: true, Ano: 2012, EdicaoWikiEN: "2012 Copa Libertadores"})
	titulo(banco.Titulo{Categoria: banco.Masculino, Competicao: "Torneio Início", Slug: "torneio-inicio",
		Escopo: "estadual", Oficial: false, Ano: 1919})
	titulo(banco.Titulo{Categoria: banco.Feminino, Competicao: "Copa Libertadores", Slug: "libertadores",
		Escopo: "continental", Oficial: true, Ano: 2017, Nota: "Conquistada na parceria com o Audax."})
	titulo(banco.Titulo{Categoria: banco.Feminino, Competicao: "Copa Libertadores", Slug: "libertadores",
		Escopo: "continental", Oficial: true, Ano: 2019})
	// Elenco: Tamires (ligada à pessoa Q2) e uma jogadora sem item no Wikidata.
	must(esc.Elenco(ctx, banco.Elenco{Categoria: banco.Feminino, Temporada: 2025, Nome: "Sem Item", Posicao: "atacante"}, 0, wiki))
	must(esc.Elenco(ctx, banco.Elenco{Categoria: banco.Feminino, Temporada: 2025, Nome: "Tamires", Numero: n(37), Posicao: "defensora", Nacionalidade: "BRA", Jogos: n(30), Gols: n(4)}, idTamires, wiki))
	must(esc.Elenco(ctx, banco.Elenco{Categoria: banco.Feminino, Temporada: 2024, Nome: "Tamires", Numero: n(6), Posicao: "defensora"}, idTamires, wiki))

	must(esc.Concluir(ctx, time.Now()))

	ac, err := acervo.Abrir(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ac.Fechar() })
	return ac
}
