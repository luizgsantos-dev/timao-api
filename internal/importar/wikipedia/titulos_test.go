package wikipedia

import (
	"testing"

	"timao-api/internal/banco"
)

// Seção "Honours" sintética, no formato das páginas dos clubes na Wikipédia em inglês.
const honoursTeste = `
==History==
Texto.

==Honours==
===Official tournaments===
{| class="wikitable"
|-
! colspan="3" |Continental
|-
! style="width:260px"|Competitions
! style="width:80px"|Titles
! style="width:380px"|Seasons
|-
! style="text-align:center"|[[Copa Libertadores Femenina]]
| bgcolor="gold" style="text-align:center"|'''2'''
|align="left"|[[2017 Copa Libertadores Femenina|2017]]{{Cref2|a|1}}, [[2019 Copa Libertadores Femenina|2019]]
|-
! colspan="3"|National
|-
![[Supercopa do Brasil de Futebol Feminino|Supercopa do Brasil]]
| bgcolor="gold" style="text-align:center"|'''3'''{{sup|s}}
|align="left"|2022, 2023
|}

===Friendly tournaments===
*Rosario Cup (1): 2018

===Others tournaments===
====State====
*[[Taça Competência]] (2): 1922, [[1923 Taça|1923]]

===Youth team===
*[[Campeonato Brasileiro Feminino Sub-17]] (1): 2025

==See also==
{{Cnote2 Begin}}
{{Cnote2|a|In 2016, [[SC Corinthians]] and Audax formed a partnership.}}
{{Cnote2 End}}
`

func TestExtrairTitulos(t *testing.T) {
	ts, avisos := ExtrairTitulos(honoursTeste, banco.Feminino)
	if len(ts) != 7 {
		t.Fatalf("esperava 7 títulos (4 oficiais, 1 amistoso, 2 outros, sem a base), vieram %d: %+v", len(ts), ts)
	}
	if len(avisos) != 1 {
		t.Fatalf("esperava 1 aviso (Supercopa diz 3 e lista 2 anos), vieram %v", avisos)
	}

	lib := ts[0]
	if lib.Competicao != "Copa Libertadores" || lib.Slug != "libertadores" || lib.Escopo != "continental" ||
		!lib.Oficial || lib.Ano != 2017 || lib.EdicaoWikiEN != "2017 Copa Libertadores Femenina" {
		t.Errorf("Libertadores 2017: %+v", lib)
	}
	if lib.Nota != "In 2016, SC Corinthians and Audax formed a partnership." {
		t.Errorf("nota de rodapé: %q", lib.Nota)
	}
	if ts[1].Nota != "" {
		t.Errorf("2019 não tem nota: %q", ts[1].Nota)
	}
	if s := ts[2]; s.Slug != "supercopa" || s.Escopo != "nacional" || s.Ano != 2022 {
		t.Errorf("Supercopa: %+v", s)
	}
	if r := ts[4]; r.Competicao != "Rosario Cup" || r.Escopo != "amistoso" || r.Oficial || r.Ano != 2018 {
		t.Errorf("Rosario Cup: %+v", r)
	}
	if c := ts[6]; c.Slug != "taca-competencia" || c.Escopo != "estadual" || c.Ano != 1923 || c.EdicaoWikiEN != "1923 Taça" {
		t.Errorf("Taça Competência: %+v", c)
	}
}

func TestExtrairTitulosSemSecao(t *testing.T) {
	if ts, avisos := ExtrairTitulos("==History==\nnada", banco.Masculino); len(ts) != 0 || len(avisos) != 1 {
		t.Fatalf("sem Honours: %v %v", ts, avisos)
	}
}
