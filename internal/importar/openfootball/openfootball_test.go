package openfootball

import (
	"testing"
)

const arquivoTeste = `= Copa Libertadores 2012

# Date       Tue Jan 24 - Wed Jul 4 2012 (162d)

▪ Gruppe 6
  Wed Feb 15 2012
    22:00  Deportivo Táchira (VEN) v Corinthians SP (BRA)     1-1 (1-0)
  Thu Feb 16
    21:00  Outro (ARG)             v Mais Um (PER)            3-0 (2-0)
           Corinthians SP (BRA)    v Club Nacional (PAR)      2-0

▪ Round of 16
  Wed May 2
    22:00  Corinthians SP (BRA)    v Emelec (ECU)             4-5 pen. 2-1 a.e.t. (2-1, 0-1)

▪ Final
  Wed Jul 4
    21:50  Corinthians SP (BRA)    v Boca Juniors (ARG)       5-6 pen. (1-1, 0-1)
  Sun Dec 30
           Futuro FC RJ            v Corinthians SP
  Mon Jan 7
    16:00  Corinthians SP          v Virada FC SP             1-0

▪ Gruppe 7
  Tue Feb 14
    22:00  Corinthians SP (BRA)    v Volta no Tempo (PAR)     1-0
  Fri Feb 14
           Corinthians SP (BRA)    v Dia Errado (PAR)         1-0
`

func TestExtrair(t *testing.T) {
	ps := Extrair(arquivoTeste, "Copa Libertadores", 2012)
	if len(ps) != 7 {
		t.Fatalf("esperava 7 partidas do Corinthians (a de dia da semana impossível fica de fora), vieram %d: %+v", len(ps), ps)
	}

	p := ps[0]
	if p.Data != "2012-02-15" || p.Horario != "22:00" || p.Fase != "Grupo 6" || p.Mando != "fora" ||
		p.Adversario != "Deportivo Táchira" || *p.GolsPro != 1 || *p.GolsContra != 1 {
		t.Errorf("1ª partida: %+v", p)
	}
	if p := ps[1]; p.Data != "2012-02-16" || p.Horario != "" || p.Adversario != "Club Nacional" || *p.GolsPro != 2 {
		t.Errorf("data herdada da linha anterior: %+v", p)
	}
	if p := ps[2]; *p.GolsPro != 2 || *p.GolsContra != 1 || *p.PenaltisPro != 4 || *p.PenaltisContra != 5 || p.Fase != "Oitavas de final" {
		t.Errorf("prorrogação e pênaltis: %+v", p)
	}
	if p := ps[3]; *p.GolsPro != 1 || *p.GolsContra != 1 || *p.PenaltisPro != 5 {
		t.Errorf("pênaltis sem prorrogação: %+v", p)
	}
	if p := ps[4]; p.GolsPro != nil || p.Adversario != "Futuro FC" || p.Mando != "fora" {
		t.Errorf("jogo sem placar, sem sigla do estado: %+v", p)
	}
	if p := ps[4]; p.Data != "2012-12-30" {
		t.Errorf("data de dezembro: %+v", p)
	}
	if p := ps[5]; p.Data != "2013-01-07" || p.Adversario != "Virada FC" {
		t.Errorf("virada de ano sem o ano na linha: %+v", p)
	}
	if p := ps[6]; p.Data != "2012-02-14" {
		t.Errorf("grupo listado depois da final, voltando no tempo: %+v", p)
	}
	for _, p := range ps {
		if p.Temporada != 2012 || p.Competicao != "Copa Libertadores" || p.Categoria != "masculino" {
			t.Errorf("metadados: %+v", p)
		}
	}
}
