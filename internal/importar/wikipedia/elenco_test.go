package wikipedia

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"timao-api/internal/banco"
	"timao-api/internal/importar/coleta"
)

const temporadaTeste = `{{Infobox football club season
| club    = [[SC Corinthians Paulista (women)|Corinthians]]
| manager = [[Lucas Piccinato]]<br />[[Emily Lima]]{{small|(from June)}}
}}
==Squad==
{{Fs start}}
{{Fs player|no=1|nat=BRA|pos=GK|name=[[Nicole Ramos|Nicole]]}}
{{Fs player|no=10|nat=BRA|pos=MF|name=[[Gabi Zanotti]]|other=[[Captain (association football)|captain]]}}
{{Fs player|nat=COL|pos=FW|name=Sem Artigo}}
{{Fs player|no=10|nat=BRA|pos=MF|name=[[Gabi Zanotti]]}}
{{Fs end}}
===Out on loan===
{{Fs player|no=30|nat=BRA|pos=DF|name=[[Emprestada]]}}
==Squad statistics==
{{Fs player|no=99|nat=BRA|pos=DF|name=[[Outra Seção]]}}
`

func TestExtrairElenco(t *testing.T) {
	el := ExtrairElenco(temporadaTeste, banco.Feminino)
	if len(el) != 3 {
		t.Fatalf("esperava 3 integrantes (sem repetida, emprestada e outra seção), vieram %d: %+v", len(el), el)
	}
	if e := el[0]; e.Nome != "Nicole" || e.WikiEN != "Nicole Ramos" || *e.Numero != 1 || e.Posicao != "goleira" || e.Nacionalidade != "BRA" {
		t.Errorf("Nicole: %+v", e)
	}
	if e := el[2]; e.Nome != "Sem Artigo" || e.WikiEN != "" || e.Numero != nil || e.Posicao != "atacante" {
		t.Errorf("sem artigo nem número: %+v", e)
	}
	if e := ExtrairElenco(temporadaTeste, banco.Masculino)[0]; e.Posicao != "goleiro" {
		t.Errorf("posição no masculino: %q", e.Posicao)
	}
}

func TestExtrairTecnicos(t *testing.T) {
	ts := ExtrairTecnicos(temporadaTeste)
	if len(ts) != 2 || ts[0].WikiEN != "Lucas Piccinato" || ts[1].Nome != "Emily Lima" {
		t.Fatalf("técnicos em ordem: %+v", ts)
	}
	if ts := ExtrairTecnicos("sem quadro"); len(ts) != 0 {
		t.Fatalf("sem quadro: %+v", ts)
	}
}

const competicaoTeste = `
==Group stage==
===Group C===
{{football box
|date  = {{Start date|2019|10|14}}
|team1 = [[SC Corinthians Paulista (women)|Corinthians]]
|score = 3–1
|team2 = Ñañas
}}
{{football box
|date  = 15 October 2019
|team1 = Outro
|score = 1–0
|team2 = Mais Um
}}
==Knockout stage==
{{football box
|round = Final
|date  = 28 October 2019
|team1 = Corinthians
|score = 2–0
|team2 = Ferroviária
}}
`

func TestExtrairPartidasDaCompeticao(t *testing.T) {
	ps, _ := ExtrairPartidasDaCompeticao(competicaoTeste, banco.Feminino, 2019, "Copa Libertadores")
	if len(ps) != 2 {
		t.Fatalf("esperava 2 jogos do Corinthians, vieram %d", len(ps))
	}
	if p := ps[0]; p.Competicao != "Copa Libertadores" || p.Fase != "Grupo C" || p.Data != "2019-10-14" || p.Temporada != 2019 {
		t.Errorf("fase de grupos: %+v", p)
	}
	if p := ps[1]; p.Fase != "Final" || p.Competicao != "Copa Libertadores" {
		t.Errorf("final (fase do campo round, competição da página): %+v", p)
	}
}

func TestItensWikidata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"query":{
			"normalized":[{"from":"gabi zanotti","to":"Gabi zanotti"}],
			"redirects":[{"from":"Gabi zanotti","to":"Gabi Zanotti"}],
			"pages":{"1":{"title":"Gabi Zanotti","pageprops":{"wikibase_item":"Q1"}},
			         "2":{"title":"Tamires","pageprops":{"wikibase_item":"Q2"}},
			         "-1":{"title":"Inexistente","missing":""}}}}`))
	}))
	defer srv.Close()

	w := Novo(coleta.Novo())
	w.API = srv.URL
	itens, err := w.ItensWikidata(context.Background(), []string{"gabi zanotti", "Tamires", "Inexistente"})
	if err != nil {
		t.Fatal(err)
	}
	if len(itens) != 2 || itens["gabi zanotti"] != "Q1" || itens["Tamires"] != "Q2" {
		t.Fatalf("itens (seguindo normalização e redirecionamento): %v", itens)
	}
}

func TestMembrosDaCategoria(t *testing.T) {
	chamadas := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chamadas++
		if r.URL.Query().Get("cmcontinue") == "" {
			_, _ = w.Write([]byte(`{"continue":{"cmcontinue":"pagina2"},"query":{"categorymembers":[{"title":"A"}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"query":{"categorymembers":[{"title":"B"}]}}`))
	}))
	defer srv.Close()

	w := Novo(coleta.Novo())
	w.API = srv.URL
	membros, err := w.MembrosDaCategoria(context.Background(), "Category:Teste")
	if err != nil || len(membros) != 2 || membros[1] != "B" || chamadas != 2 {
		t.Fatalf("membros com continuação: %v %v (%d chamadas)", membros, err, chamadas)
	}
}
