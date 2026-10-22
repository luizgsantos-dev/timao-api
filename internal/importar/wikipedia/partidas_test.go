package wikipedia

import (
	"testing"

	"timao-api/internal/banco"
)

// Página sintética no formato das páginas de temporada da Wikipédia em inglês.
const paginaTeste = `
== Squad ==
Texto qualquer.

== Campeonato Paulista ==
=== First stage ===
{{football box collapsible
|round      = 3
|date       = {{Start date|2025|1|22}}
|time       = {{UTZ|21:30|-3}}
|team1      = [[Sport Club Corinthians Paulista|Corinthians]]
|score      = 2–1
|team2      = [[Mirassol Futebol Clube|Mirassol]] {{flagicon|BRA}}
|goals1     = [[Yuri Alberto]] {{goal|12|pen|80}}<ref>nota</ref>
|goals2     = [[Fulano de Tal|Fulano]] {{goal|45+2}}
|stadium    = [[Neo Química Arena]]
|location   = [[São Paulo]]
|attendance = 41,234
|referee    = Ciclano
}}
=== Copa Paulista ===
{{football box collapsible
|date       = 22 April
|team1      = Santos
|score      = 0–3
|team2      = Corinthians
|goals1     =
|goals2     = {{goal|10}} [[Dentinho]]<br>[[Beltrano]] {{goal|20}}<br>[[Zagueiro do Santos|Zagueiro]] {{goal|30|o.g.}}
|attendance = Undisclosed
}}

== Matches ==
=== Copa Libertadores ===
{{Football box
|round      = Quarter-finals
|date       = 10 August 2025
|team1      = [[River Plate]]
|score      = 1–1
|penaltyscore = 4–5
|team2      = Corinthians
}}

== Friendlies ==
{{football box collapsible
|date       = 5 January 2025
|team1      = Time A
|score      = 1–0
|team2      = Time B
}}
{{football box collapsible
|date       = 6 January 2025
|team1      = Corinthians
|score      = v
|team2      = Futuro FC
}}
`

func TestExtrairPartidas(t *testing.T) {
	partidas, avisos := ExtrairPartidas(paginaTeste, banco.Feminino, 2025)
	if len(partidas) != 4 {
		t.Fatalf("esperava 4 partidas, vieram %d (avisos: %v)", len(partidas), avisos)
	}
	if len(avisos) != 1 {
		t.Fatalf("esperava 1 aviso (jogo sem o Corinthians), vieram %v", avisos)
	}

	p := partidas[0]
	confere(t, "data", p.Data, "2025-01-22")
	confere(t, "horário", p.Horario, "21:30")
	confere(t, "competição", p.Competicao, "Campeonato Paulista")
	confere(t, "fase", p.Fase, "Rodada 3")
	confere(t, "adversário", p.Adversario, "Mirassol")
	confere(t, "mando", p.Mando, "casa")
	confere(t, "estádio", p.Estadio, "Neo Química Arena")
	confere(t, "categoria", p.Categoria, banco.Feminino)
	placarIgual(t, p, 2, 1)
	if p.Publico == nil || *p.Publico != 41234 {
		t.Errorf("público: %v", p.Publico)
	}
	if len(p.Gols) != 3 {
		t.Fatalf("esperava 3 gols, vieram %+v", p.Gols)
	}
	confere(t, "autor", p.Gols[0].Autor, "Yuri Alberto")
	confere(t, "tipo do 1º gol", p.Gols[0].Tipo, "penalti")
	confere(t, "tipo do 2º gol", p.Gols[1].Tipo, "normal")
	confere(t, "lado do gol do Mirassol", p.Gols[2].Lado, "contra")
	confere(t, "wiki do autor", p.Gols[2].AutorWikiEN, "Fulano de Tal")
	confere(t, "minuto", p.Gols[2].Minuto, "45+2")

	p = partidas[1]
	confere(t, "data sem ano", p.Data, "2025-04-22")
	confere(t, "competição dentro de outra seção", p.Competicao, "Copa Paulista")
	confere(t, "mando", p.Mando, "fora")
	placarIgual(t, p, 3, 0)
	if p.Publico != nil {
		t.Errorf("público não informado deveria ser nulo: %v", *p.Publico)
	}
	confere(t, "autor depois do minuto", p.Gols[0].Autor, "Dentinho")
	// O gol contra do Santos foi listado do lado do Santos; a conciliação move para o Corinthians.
	for _, g := range p.Gols {
		if g.Lado != "pro" {
			t.Errorf("todos os gols deveriam ser do Corinthians: %+v", g)
		}
	}
	confere(t, "tipo gol contra", p.Gols[2].Tipo, "contra")

	p = partidas[2]
	confere(t, "competição sob título genérico", p.Competicao, "Copa Libertadores")
	confere(t, "fase traduzida", p.Fase, "Quartas de final")
	if p.PenaltisPro == nil || *p.PenaltisPro != 5 || *p.PenaltisContra != 4 {
		t.Errorf("pênaltis: %v x %v", p.PenaltisPro, p.PenaltisContra)
	}

	p = partidas[3]
	confere(t, "amistoso", p.Competicao, "Amistoso")
	if p.GolsPro != nil {
		t.Errorf("jogo futuro não deveria ter placar")
	}
}

func TestCorrigirAno(t *testing.T) {
	casos := []struct {
		data, want string
		corrigiu   bool
	}{
		{"2011-05-23", "2012-05-23", true},  // Libertadores de 2012 digitada como 2011
		{"2011-12-20", "2011-12-20", false}, // dezembro anterior: pode ser pré-temporada
		{"2011-01-20", "2012-01-20", true},
		{"2004-01-13", "2012-01-13", true},  // ano bem distante
		{"2013-02-10", "2013-02-10", false}, // ano seguinte (temporada que atravessa o ano)
		{"2015-03-01", "2012-03-01", true},
		{"2012-03-01", "2012-03-01", false},
	}
	for _, c := range casos {
		got, ok := corrigirAno(c.data, 2012)
		if got != c.want || ok != c.corrigiu {
			t.Errorf("corrigirAno(%s) = %s, %v; esperava %s, %v", c.data, got, ok, c.want, c.corrigiu)
		}
	}
}

func TestLimpar(t *testing.T) {
	casos := map[string]string{
		"[[Sport Club Corinthians Paulista|Corinthians]] {{flagicon|BRA}}": "Corinthians",
		"{{nowrap|[[Neo Química Arena]]}}<ref>x</ref>":                     "Neo Química Arena",
		"'''Fulano'''<!-- comentário -->":                                  "Fulano",
		"{{sortname|Cássio|Ramos}}":                                        "Cássio Ramos",
	}
	for in, want := range casos {
		confere(t, in, limpar(in), want)
	}
}

func TestSepararParametros(t *testing.T) {
	got := separarParametros("goal|12|{{a|b}}|[[x|y]]")
	if len(got) != 4 || got[2] != "{{a|b}}" || got[3] != "[[x|y]]" {
		t.Fatalf("separação errada: %q", got)
	}
}

func confere(t *testing.T, campo, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: veio %q, esperava %q", campo, got, want)
	}
}

func placarIgual(t *testing.T, p banco.Partida, pro, contra int) {
	t.Helper()
	if p.GolsPro == nil || p.GolsContra == nil || *p.GolsPro != pro || *p.GolsContra != contra {
		t.Errorf("placar %s: veio %v x %v, esperava %d x %d", p.Data, p.GolsPro, p.GolsContra, pro, contra)
	}
}

// Caixa da Wikipédia em português, no formato das páginas de Copa do Brasil e Paulista.
const caixaPT = `== Final ==
{{footballbox
|data=[[17 de junho]]
|hora   =21:50
|time1  ={{Futebol Corinthians|estado=depois}}
|placar =2 – 0
|time2  ={{Futebol Internacional-RS|estado=antes}}
|gols1  =[[Jorge Henrique de Souza|Jorge Henrique]] {{gol|26}}<br />[[Ronaldo Luís Nazário de Lima|Ronaldo]] {{gol|53}}
|gols2  =
|estadio=[[Estádio Municipal Paulo Machado de Carvalho|Estádio do Pacaembu]], [[São Paulo]]
|publico=36.614
|arbitro={{Arbitro|BR-PR|PR|[[Heber Roberto Lopes]]}}
}}
{{footballbox
|data = 1º de julho de 2009
|time1 = {{Futebol Internacional-RS|estado=depois}}
|placar = 2 — 2
|time2 = {{Futebol Corinthians|estado=antes}}
|gols1 = [[Alecsandro]] {{gol|69}}, {{gol|74}}
|gols2 = [[Jorge Henrique de Souza|Jorge Henrique]] {{gol|29}}
}}`

func TestCaixaEmPortugues(t *testing.T) {
	ps, avisos := ExtrairPartidasDaCompeticao(caixaPT, banco.Masculino, 2009, "Copa do Brasil")
	if len(ps) != 2 {
		t.Fatalf("esperava 2 jogos, vieram %d (avisos: %v)", len(ps), avisos)
	}
	p := ps[0]
	confere(t, "data sem ano", p.Data, "2009-06-17")
	confere(t, "adversário da predefinição Futebol", p.Adversario, "Internacional-RS")
	confere(t, "mando", p.Mando, "casa")
	confere(t, "árbitro", p.Arbitro, "Heber Roberto Lopes")
	confere(t, "fase", p.Fase, "Final")
	placarIgual(t, p, 2, 0)
	if p.Publico == nil || *p.Publico != 36614 || len(p.Gols) != 2 || p.Gols[1].Autor != "Ronaldo" || p.Gols[1].Minuto != "53" {
		t.Errorf("público e gols: %v %+v", p.Publico, p.Gols)
	}
	p = ps[1]
	confere(t, "data com 1º", p.Data, "2009-07-01")
	placarIgual(t, p, 2, 2)
	if len(p.Gols) != 3 {
		t.Errorf("dois gols do Alecsandro na mesma linha e um do Corinthians: %+v", p.Gols)
	}
}
