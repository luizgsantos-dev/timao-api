package rsssf

import (
	"strings"
	"testing"

	"timao-api/internal/banco"
)

// Trechos reais de braz05.html (rsssf.org), encurtados.
const brasileiraoTeste = `<h4><a name="sa">Série A</a></h4>
<pre>
Round 1
[Apr 24]
Santos         4-1  Paysandu
  [Deivid 4', 90', Robinho 20'p, Edmílson 70'; Leonardo 20'og]
Corinthians    2-2  Juventude
  [Carlos Alberto Tevez 9', Marcelo Mattos 43'; Enilton 6', Naldo 78']

Round 2
[Apr 30]
Botafogo       3-1  Corinthians
  [Alex Alves 51'p, 88'p, Ramón Hubner 74'; Gil 33']

Round 8
[Jun 12]
Corinthians    4-2  Flamengo           [played in Mogi-Mirim]
  [Jô 23', 83', Abuda 52', Roger Flores 77'; Fabiano Oliveira 73', Jean Ferreira 81']

Round 15
[Jul 27]
Corinthians    4-3  Cruzeiro
  [Roger Flores 11', Rosinei 60', 62', Carlos Alberto Tevez 73'p; Marinho 4'og, Fred 46', Moisés 68']

Round 16
[Jul 31]
Santos         4-2  Corinthians            [annulled - refereed by Edílson P.Carvalho]
  [Giovanni 1', 60', Ricardinho 53', Wendell 69'; Roger Flores 31', Rosiniei 62']

Round 20
[Aug 14]
Ponte Preta    3-5  Corinthians
  [Kahê 30', Danilo Sacramento 67', Izaías 79'; Carlos Alberto Tevez 14'p, 68', Roger Flores 47', 49',
   Sebá Domínguez 74']

Replayed matches
[Oct 13]
Santos         2-3  Corinthians
  [Cláudio Pitbull 12min', Luciano Henrique 69'; Betão 35', Nilmar 72', Carlos Alberto Jesus 87'p]

Round 42 [Dec 4]
Goiás          3-2  Corinthians
  [Paulo Baier 45', Souza 69', Romerito 84'; Carlos Alberto Tevez 50', Coelho 57']
</pre>
<h4><a name="cb">Copa do Brasil</a></h4>
<pre>
 4 Corinthians - Juventude (RS)            1-0 (10/02)     1-0 (24/02)
</pre>`

func TestExtrairBrasileirao(t *testing.T) {
	ps, avisos := ExtrairBrasileirao(Texto([]byte(brasileiraoTeste)), 2005)
	if len(ps) != 7 {
		t.Fatalf("esperava 7 jogos (sem o anulado e sem a Copa do Brasil), vieram %d: %+v", len(ps), ps)
	}
	confere := func(nome string, p banco.Partida, data, adv, mando string, pro, contra int, fase string) {
		t.Helper()
		if p.Data != data || p.Adversario != adv || p.Mando != mando || *p.GolsPro != pro || *p.GolsContra != contra || p.Fase != fase {
			t.Errorf("%s: %+v", nome, p)
		}
		if p.Categoria != banco.Masculino || p.Temporada != 2005 || p.Competicao != "Campeonato Brasileiro" {
			t.Errorf("%s: metadados %+v", nome, p)
		}
	}
	confere("em casa", ps[0], "2005-04-24", "Juventude", "casa", 2, 2, "Rodada 1")
	confere("fora", ps[1], "2005-04-30", "Botafogo", "fora", 1, 3, "Rodada 2")
	confere("remarcado", ps[5], "2005-10-13", "Santos", "fora", 3, 2, "Jogo remarcado")
	confere("rodada com data na linha", ps[6], "2005-12-04", "Goiás", "fora", 2, 3, "Rodada 42")

	g := ps[1].Gols
	if len(g) != 4 || g[0] != (banco.Gol{Lado: "contra", Autor: "Alex Alves", Minuto: "51", Tipo: "penalti"}) ||
		g[1].Autor != "Alex Alves" || g[1].Minuto != "88" || g[3] != (banco.Gol{Lado: "pro", Autor: "Gil", Minuto: "33", Tipo: "normal"}) {
		t.Errorf("gols com pênalti e nome repetido: %+v", g)
	}
	if ps[2].Cidade != "Mogi-Mirim" {
		t.Errorf("cidade da nota: %+v", ps[2])
	}
	if g := ps[3].Gols; len(g) != 7 || g[4] != (banco.Gol{Lado: "contra", Autor: "Marinho", Minuto: "4", Tipo: "contra"}) {
		t.Errorf("gol contra fica do lado beneficiado: %+v", g)
	}
	if g := ps[4].Gols; len(g) != 8 || g[7].Autor != "Sebá Domínguez" || g[3].Lado != "pro" || g[3].Tipo != "penalti" {
		t.Errorf("linha de gols quebrada em duas: %+v", g)
	}
	if g := ps[5].Gols; len(g) != 5 || g[0].Minuto != "12" {
		t.Errorf("minuto escrito 12min': %+v", g)
	}
	if len(avisos) != 1 || !strings.Contains(avisos[0], "anulado") {
		t.Errorf("avisos: %v", avisos)
	}
}

// Trechos reais de cbr2009.htm, cbr2014.htm e da seção de Copa de braz2010.html
// (rsssfbrasil.com). O confronto com pênaltis usa a linha real do ICASA x
// Portuguesa, com o Corinthians no lugar da Portuguesa.
const copaTeste = `<pre>First Round (18/02 to 19/03)
[if the away team wins the 1st leg by 2 or more goals, no 2nd leg is played]</pre>

<pre> 7 Corinthians (AL) - Vitória (BA)              3-1 (24/02)     0-4 (10/03)
                                                [Yanick 74, Catanha 81, Tozin 88 ; Ramon Menezes 79]
 9 Itumbiara (GO) - Corinthians (SP)            0-2 (04/03)     n/p (19/03)
                                                [Chicão 44<em>pen</em>, André Santos 49]
 3 ICASA (CE) - Corinthians (SP)                1-1 (18/02)     1-1 (04/03)     [4-3<em>pen</em>]  [<a
href="#a">a</a>]
                                                [Serginho 12 ; Bruno Rodrigo 90]
                                                [Moré 49 ; Bruno Rodrigo 84]</pre>

<pre>Quarterfinals</pre>

<pre>66 Corinthians - Bahia                          3-0 (23/07)     0-1 (06/08)
                                                [Elias 18, Romero 77, Renato Augusto 89<em>pen</em>]
                                                [Guilherme Andrade 33<em>og</em>]</pre>

<pre>FINAL</pre>

<pre>   Corinthians - Internacional                  2-0 (17/06)     2-2 (01/07)
                                                [Jorge Henrique 26, Ronaldo 53]
                                                [Jorge Henrique 20, André Santos 28 ; Alecsandro 70, 74]</pre>`

// Trechos reais de cbr1995.htm: ida e volta sem data e a súmula da final
// (gravados em Latin-1 no teste, como a página original).
const copa1995Teste = `<P><PRE>First Round <I>[ From Feb 14 to Apr 12 ]</I>
Operário(MT)1-1 0-4 Corinthians(SP)
Rio Branco(AC)  2-0 2-2 Palmares(RO)

Quarter-Finals <I>[ From May 3 to May 17 ]</I>

[May 03, 16]
Paraná(PR)    0-0 1-2   Corinthians(SP)

Final

14/06/1995 Corinthians 2-1 Grêmio
                   Viola               41'    (1-0)
                   Luiz Carlos Goiano  65'    (1-1)
                   Marcelinho Carioca  71'    (2-1)
Referee: Antônio Pereira da Silva
Att: 25 281
Corinthians: Ronaldo, Vítor, Célio Silva, Henrique, Silvinho - Bernardo (Ezequiel),
</PRE>`

func TestExtrairCopaDoBrasil(t *testing.T) {
	ps, sem, avisos := ExtrairCopaDoBrasil(Texto([]byte(copaTeste)), 2009)
	if len(ps) != 7 || sem != 0 || len(avisos) != 0 {
		t.Fatalf("esperava 7 jogos (sem o Corinthians-AL e sem o n/p), vieram %d, sem data %d, avisos %v: %+v", len(ps), sem, avisos, ps)
	}
	if p := ps[0]; p.Data != "2009-03-04" || p.Adversario != "Itumbiara" || p.Mando != "fora" || *p.GolsPro != 2 ||
		p.Fase != "Primeira fase" || len(p.Gols) != 2 || p.Gols[0] != (banco.Gol{Lado: "pro", Autor: "Chicão", Minuto: "44", Tipo: "penalti"}) {
		t.Errorf("ida fora, volta não jogada: %+v", p)
	}
	if p := ps[1]; p.Mando != "fora" || p.PenaltisPro != nil || len(p.Gols) != 2 || p.Gols[0].Lado != "contra" || p.Gols[1].Autor != "Bruno Rodrigo" {
		t.Errorf("ida do confronto com pênaltis: %+v", p)
	}
	if p := ps[2]; p.Data != "2009-03-04" || p.Mando != "casa" || p.PenaltisPro == nil || *p.PenaltisPro != 3 || *p.PenaltisContra != 4 {
		t.Errorf("volta em casa, decidida nos pênaltis (4-3 para o ICASA): %+v", p)
	}
	if p := ps[4]; p.Mando != "fora" || *p.GolsContra != 1 || len(p.Gols) != 1 ||
		p.Gols[0] != (banco.Gol{Lado: "contra", Autor: "Guilherme Andrade", Minuto: "33", Tipo: "contra"}) || p.Fase != "Quartas de final" {
		t.Errorf("gol contra listado sozinho, do lado do Bahia: %+v", p)
	}
	if p := ps[6]; p.Fase != "Final" || p.Mando != "fora" || len(p.Gols) != 4 || p.Gols[3].Lado != "contra" || p.Gols[3].Minuto != "74" {
		t.Errorf("volta da final: %+v", p)
	}

	ps, sem, _ = ExtrairCopaDoBrasil(Texto(latin1(copa1995Teste)), 1995)
	if sem != 2 || len(ps) != 3 {
		t.Fatalf("1995: esperava 3 jogos e 2 sem data, vieram %d e %d: %+v", len(ps), sem, ps)
	}
	if p := ps[1]; p.Data != "1995-05-16" || p.Mando != "casa" || *p.GolsPro != 2 || p.Adversario != "Paraná" {
		t.Errorf("volta com data de \"[May 03, 16]\": %+v", p)
	}
	p := ps[2]
	if p.Data != "1995-06-14" || p.Adversario != "Grêmio" || p.Arbitro != "Antônio Pereira da Silva" || p.Publico == nil || *p.Publico != 25281 {
		t.Errorf("súmula da final: %+v", p)
	}
	if len(p.Gols) != 3 || p.Gols[1] != (banco.Gol{Lado: "contra", Autor: "Luiz Carlos Goiano", Minuto: "65", Tipo: "normal"}) || p.Gols[2].Lado != "pro" {
		t.Errorf("gols pelo placar corrido: %+v", p.Gols)
	}
}

// Trechos reais de sp1922.htm (HTML sem <pre>, com quebra no meio do jogo),
// sp1993.htm, sp2003.htm e sp1977.htm (a final sem a linha de data).
const paulistaTeste = `<p><font size="2" face="Courier New">23/04/1922 Germânia 1 x 3
Santos<br>
23/04/1922 Palestra Itália 2 x 2 Corinthians<br>
18/06/1922 AA S&atilde;o Bento 0 x 1 Corinthians<br></font></p>
<pre>24/Jan Corinthians 0-1 Bragantino
28/Jan Ituano 0-1 Corinthians</pre>
<pre>
Semi-finals

[Mar 5 and 8]
Palmeiras       2-2    2-4  Corinthians
  [Adãozinho 15'p, Neném 34'; Ânderson 68', Liédson 77']
  [Fabrício 21'og, Darío Muñoz 61'; Liédson 9', Gil 11', 15', Rogério Régis 41'p]

Final

Corinthians       1-0  Ponte Preta
</pre>`

func TestExtrairPaulista(t *testing.T) {
	ps, sem, _ := ExtrairPaulista(Texto(latin1(paulistaTeste)), 1922)
	if sem != 1 || len(ps) != 6 {
		t.Fatalf("esperava 6 jogos e 1 sem data, vieram %d e %d: %+v", len(ps), sem, ps)
	}
	if p := ps[0]; p.Data != "1922-04-23" || p.Adversario != "Palestra Itália" || p.Mando != "fora" || *p.GolsPro != 2 {
		t.Errorf("data na linha: %+v", p)
	}
	if p := ps[1]; p.Adversario != "AA São Bento" || p.Mando != "fora" || *p.GolsPro != 1 {
		t.Errorf("entidade HTML no nome: %+v", p)
	}
	if p := ps[2]; p.Data != "1922-01-24" || p.Mando != "casa" || *p.GolsContra != 1 {
		t.Errorf("dia/mês abreviado: %+v", p)
	}
	if p := ps[4]; p.Data != "1922-03-05" || p.Fase != "Semifinal" || len(p.Gols) != 4 || p.Gols[2].Autor != "Ânderson" {
		t.Errorf("ida e volta com datas \"[Mar 5 and 8]\": %+v", p)
	}
	if p := ps[5]; p.Data != "1922-03-08" || p.Mando != "casa" || *p.GolsPro != 4 ||
		p.Gols[0] != (banco.Gol{Lado: "contra", Autor: "Fabrício", Minuto: "21", Tipo: "contra"}) {
		t.Errorf("volta em casa, gol contra listado do lado do Palmeiras: %+v", p)
	}
}

func TestCorinthiansHomonimos(t *testing.T) {
	for _, n := range []string{"Corinthians", "Corinthians (SP)", "Corinthians-SP", "Corinthians(SP)", "SC Corinthians Paulista"} {
		if !ehCorinthians(n) {
			t.Errorf("%q devia ser o Corinthians", n)
		}
	}
	for _, n := range []string{"Corinthians (AL)", "Corinthians-AL", "Corinthians Alagoano", "SC Corinthians Alagoano",
		"Corinthians (PR)", "Corinthians/PP", "Corinthians de Presidente Prudente", "Coríntians-RN", "Corinthians Paranaense"} {
		if ehCorinthians(n) {
			t.Errorf("%q não é o Corinthians paulista", n)
		}
	}
	for n, quer := range map[string]string{"Atlético/PR": "Atlético-PR", "Atlético MG": "Atlético-MG", "Inter (RS)": "Internacional",
		"Náutico-PE": "Náutico", "Flamengo(PI)": "Flamengo-PI", "Flamengo (RJ)": "Flamengo", "Portuguesa/San": "Portuguesa Santista",
		"São Bento (SCS)": "São Bento (SCS)", "Remo (PA)": "Remo"} {
		if got := nomeAdversario(n); got != quer {
			t.Errorf("nomeAdversario(%q) = %q, quer %q", n, got, quer)
		}
	}
}

// latin1 grava o texto em ISO-8859-1, como as páginas antigas do RSSSF.
func latin1(s string) []byte {
	var b []byte
	for _, r := range s {
		b = append(b, byte(r))
	}
	return b
}

func TestTextoLatin1(t *testing.T) {
	if got := Texto([]byte("<pre>S\xe3o Paulo\nGr\xeamio</pre>")); strings.TrimSpace(got) != "São Paulo\nGrêmio" {
		t.Errorf("Latin-1: %q", got)
	}
	if got := Texto([]byte("<pre>São Paulo</pre>")); strings.TrimSpace(got) != "São Paulo" {
		t.Errorf("UTF-8 válido não pode ser reconvertido: %q", got)
	}
}

func TestAutor(t *testing.T) {
	casos := map[string]string{
		// sp2002/cbr2002: crédito quebrado em várias linhas, com e-mails
		`<p><i>Author: Daniel Dalence Garcia (</i><a
href="ddgarcia@terra.com.br"><i>ddgarcia@terra.com.br</i></a><i>)
and Marcelo Leme de Arruda (</i><a
href="mailto:marcelo.leme.arruda@gmail.com"><i>marcelo.leme.arruda@gmail.com</i></a><i>)<br>
Last updated: 16 May 2002</i> </p>`: "Daniel Dalence Garcia and Marcelo Leme de Arruda",
		`<P><I>Author: RSSSF Brazil </I><BR><I>Last updated: 29 Apr 2006</I>`:               "RSSSF Brazil",
		`<P>Prepared and maintained by <I>Ricardo Pontes</I> for the <A href="x">RSSSF</A>`: "Ricardo Pontes",
		`<P><B>(C) Copyright Julio Bovi Diogo, RSSSF and RSSSF Brazil 2000/2020</B>`:        "Julio Bovi Diogo",
		`<pre>sem crédito</pre>`: "",
	}
	for html, quer := range casos {
		if got := Autor(Texto([]byte(html))); got != quer {
			t.Errorf("Autor(%q) = %q, quer %q", html, got, quer)
		}
	}
}

func TestTraduzirFase(t *testing.T) {
	for in, quer := range map[string]string{
		"First Stage - First Phase": "Primeira etapa - Primeira fase",
		"Second Stage - Final":      "Segunda etapa - Final",
		"Quarter-Finals":            "Quartas de final",
		"FINAL":                     "Final",
		"Preliminary Round":         "Fase preliminar",
	} {
		if got := traduzirFase(in); got != quer {
			t.Errorf("traduzirFase(%q) = %q, quer %q", in, got, quer)
		}
	}
	if ehCabecalho("Botafogo qualified to final due to better first phase record") {
		t.Error("frase comum não é linha de fase")
	}
}

func TestPaulistaParaNaDivisaoInferior(t *testing.T) {
	texto := `First Level

First Stage

Round 1 [Mar 3]

Corinthians       2-0  Santos

Second Level

First Phase

Round 1 [Mar 10]

Corinthians       2-0  Dracena
`
	ps, _, _ := ExtrairPaulista(texto, 1985)
	if len(ps) != 1 || ps[0].Adversario != "Santos" {
		t.Fatalf("só o jogo da primeira divisão deveria entrar: %+v", ps)
	}
}

func TestNomesConsertados(t *testing.T) {
	casos := map[string]string{
		"Araçatuba (*)":                "Araçatuba",
		"Paulista (played in Barueri)": "Paulista",
		"Sírio (Corinthians abandoned the league and joined APEA)": "Sírio",
		"XV de Novembro (Jaú)": "XV de Novembro (Jaú)",
		"Corinthians (SBC)":    "Corinthians (SBC)",
	}
	for in, want := range casos {
		if got := nomeAdversario(in); got != want {
			t.Errorf("nomeAdversario(%q) = %q, esperava %q", in, got, want)
		}
	}
	if got := Texto([]byte("<pre>Germ\xef\xbf\xbdnia 1-0 Santo Andr\xe2\x80\x9a\nPalestra It\xef\xbf\xbdlia</pre>")); !strings.Contains(got, "Germânia 1-0 Santo André") || !strings.Contains(got, "Palestra Itália") {
		t.Errorf("acentos consertados: %q", got)
	}
}
