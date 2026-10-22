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

func TestElencoEmSubsecao(t *testing.T) {
	// Formato das páginas do masculino de 2015–2019: "===Squad===" dentro de "==Background==",
	// e uma "==Squad statistics==" de nível 2 que não é o elenco.
	texto := `==Background==
===Kit===
Texto.
===Squad===
As of 15 September 2015
{{Fs start}}
{{Fs player|no=1|nat=BRA|pos=GK|name=[[Cássio Ramos|Cássio]]}}
{{Fs player|no=9|nat=BRA|pos=FW|name=[[Vágner Love]]|other=on loan from [[Monaco]]}}
{{Fs end}}
==Squad statistics==
{| class="wikitable"
|}
==Players==
===First team squad===
{{fs player|no=99|nat=BRA|pos=MF|name=[[Outra Seção]]}}
`
	el := ExtrairElenco(texto, banco.Masculino)
	if len(el) != 2 || el[0].Nome != "Cássio" || el[1].WikiEN != "Vágner Love" {
		t.Fatalf("elenco da subseção ===Squad===: %+v", el)
	}

	// Formato de 2013: "===First team squad===" dentro de "==Players==", com emprestados à parte.
	texto = `==Players==
===First team squad===
{{fs start}}
{{Fs player |no=1  |nat=BRA |pos=GK |name=[[Júlio César de Souza Santos|Júlio César]]}}
{{fs end}}
===Out on Loan===
{{Fs player|no=30|nat=BRA|pos=DF|name=[[Emprestado]]}}
`
	el = ExtrairElenco(texto, banco.Masculino)
	if len(el) != 1 || el[0].Nome != "Júlio César" || *el[0].Numero != 1 {
		t.Fatalf("elenco de First team squad: %+v", el)
	}
}

const estatisticasTeste = `===Statistics===
{| class="wikitable"
|-
! rowspan="2"|No.
! rowspan="2"|Pos.
! rowspan="2"|Name
! colspan="2"|[[2008 Campeonato Brasileiro Série B|Brasileiro Série B]]
! colspan="2"|Copa do Brasil
! colspan="2"|Total
! colspan="2"|Discipline
|-
!Apps
!Goals
!Apps
!Goals
!Apps
!Goals
![[File:Yellow card.svg|13px]]
![[File:Red card.svg|13px]]
|-
|align="left"|1||align="left"|GK||align="left"|{{Flag icon|BRA}} [[Luiz Felipe Ventura dos Santos|Felipe]]
|27||-17||11||-12||38||-29||1||0
|-
|align="left"|2||align="left"|DF||align="left"|{{flagicon|PER}} [[Edu Dracena]]
|9 (2)||1||1 (3)||0||10 (5)||1||3||0
|}
`

func TestExtrairEstatisticas(t *testing.T) {
	es := ExtrairEstatisticas(estatisticasTeste, banco.Masculino)
	if len(es) != 2 {
		t.Fatalf("esperava 2 jogadores, vieram %+v", es)
	}
	goleiro := es[0]
	if goleiro.Nome != "Felipe" || goleiro.WikiEN != "Luiz Felipe Ventura dos Santos" || *goleiro.Jogos != 38 || goleiro.Gols != nil ||
		goleiro.Posicao != "goleiro" || goleiro.Nacionalidade != "BRA" || *goleiro.Numero != 1 {
		t.Errorf("goleiro (gols sofridos negativos ficam nulos): %+v", goleiro)
	}
	if e := es[1]; *e.Jogos != 15 || *e.Gols != 1 || e.Nacionalidade != "PER" {
		t.Errorf("titular e reserva somados pelo Total: %+v jogos=%v", e, *e.Jogos)
	}
}

func TestMesclarElenco(t *testing.T) {
	n := func(v int) *int { return &v }
	elenco := []banco.Elenco{{Nome: "Felipe", WikiEN: "Luiz Felipe Ventura dos Santos", Numero: n(1)}}
	est := []banco.Elenco{
		{Nome: "Felipe", WikiEN: "Luiz Felipe Ventura dos Santos", Jogos: n(38), Nacionalidade: "BRA"},
		{Nome: "Saiu no Meio", Jogos: n(3)},
	}
	m := MesclarElenco(elenco, est)
	if len(m) != 2 || *m[0].Jogos != 38 || m[0].Nacionalidade != "BRA" || *m[0].Numero != 1 || m[1].Nome != "Saiu no Meio" {
		t.Fatalf("mescla: %+v", m)
	}
}
