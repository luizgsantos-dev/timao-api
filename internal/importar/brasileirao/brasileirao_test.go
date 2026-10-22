package brasileirao

import (
	"strings"
	"testing"
)

const csvTeste = `id_match,season,date,home,score,away
1,2003,2003-03-29,Athletico PR,2x0,Grêmio
3,2003,2003-03-30,Corinthians,0x3,Atlético MG
4,2003,2003-04-06,Santos,1x2,Corinthians
5,2026,2026-12-02,Remo,x,Corinthians
6,2026,data-ruim,Corinthians,1x0,Bahia
`

func TestExtrair(t *testing.T) {
	ps, err := Extrair(strings.NewReader(csvTeste))
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 3 {
		t.Fatalf("esperava 3 jogos do Corinthians (sem a linha de data ruim), vieram %d: %+v", len(ps), ps)
	}
	if p := ps[0]; p.Mando != "casa" || p.Adversario != "Atlético MG" || *p.GolsPro != 0 || *p.GolsContra != 3 {
		t.Errorf("jogo em casa: %+v", p)
	}
	if p := ps[1]; p.Mando != "fora" || p.Adversario != "Santos" || *p.GolsPro != 2 || *p.GolsContra != 1 {
		t.Errorf("placar visto pelo Corinthians fora de casa: %+v", p)
	}
	if p := ps[2]; p.GolsPro != nil || p.Temporada != 2026 {
		t.Errorf("jogo futuro sem placar: %+v", p)
	}
}

func TestColunaAusente(t *testing.T) {
	if _, err := Extrair(strings.NewReader("a,b\n1,2\n")); err == nil {
		t.Fatal("esperava erro de coluna ausente")
	}
}
