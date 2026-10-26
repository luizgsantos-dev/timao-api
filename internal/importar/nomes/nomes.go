// Package nomes traduz para o português os nomes de competições e fases que as
// fontes trazem em inglês, para que todas as fontes falem a mesma língua.
package nomes

import (
	"regexp"
	"strings"
)

// Nomes de competição como aparecem nas fontes em inglês -> nome em português.
var nomesCompeticao = map[string]string{
	"libertadores":               "Copa Libertadores",
	"copa libertadores":          "Copa Libertadores",
	"sudamericana":               "Copa Sul-Americana",
	"copa sudamericana":          "Copa Sul-Americana",
	"recopa sudamericana":        "Recopa Sul-Americana",
	"fifa club world cup":        "Mundial de Clubes da FIFA",
	"fifa women's champions cup": "Copa dos Campeões Feminina da FIFA",
	"friendlies":                 "Amistoso",
}

// Competicao traduz o nome de uma competição. O que não reconhece fica como veio.
func Competicao(s string) string {
	if n, ok := nomesCompeticao[strings.ToLower(s)]; ok {
		return n
	}
	return s
}

var (
	reRodada      = regexp.MustCompile(`(?i)^(?:match\s*|matchday\s*|round\s*)?(\d{1,2})$`)
	reGrupo       = regexp.MustCompile(`(?i)^(?:group|gruppe|grupo)\s+(\w+)$`)
	reOrdinalFase = regexp.MustCompile(`(?i)^(first|second|third|fourth|fifth|sixth) (round|stage|phase)$`)
	ordinais      = map[string]string{
		"first": "Primeira", "second": "Segunda", "third": "Terceira",
		"fourth": "Quarta", "fifth": "Quinta", "sixth": "Sexta",
	}
	nomesFase = map[string]string{
		"final": "Final", "finals": "Final",
		"semi-final": "Semifinal", "semi-finals": "Semifinal", "semifinal": "Semifinal", "semifinals": "Semifinal",
		"quarter-final": "Quartas de final", "quarterfinal": "Quartas de final", "quarter-finals": "Quartas de final", "quarterfinals": "Quartas de final",
		"round of 16": "Oitavas de final", "round of 32": "Primeira fase eliminatória",
		"group stage": "Fase de grupos", "league stage": "Fase de liga",
		"first leg": "Ida", "second leg": "Volta",
		"knockout round play-offs": "Playoffs", "knockout stages": "Mata-mata", "knockout stage": "Mata-mata",
		"third place": "Disputa de 3º lugar", "3rd place match": "Disputa de 3º lugar", "match 3rd place": "Disputa de 3º lugar",
		"match 5th place": "Disputa de 5º lugar", "match for fifth place": "Disputa de 5º lugar",
		"friendly": "", "pre-season & friendlies": "",
	}
)

// Fase traduz as fases mais comuns. O que não reconhece fica como veio.
func Fase(s string) string {
	if m := reRodada.FindStringSubmatch(s); m != nil {
		return "Rodada " + m[1]
	}
	if m := reGrupo.FindStringSubmatch(s); m != nil {
		return "Grupo " + strings.ToUpper(m[1])
	}
	if m := reOrdinalFase.FindStringSubmatch(s); m != nil {
		return ordinais[strings.ToLower(m[1])] + " fase"
	}
	if n, ok := nomesFase[strings.ToLower(s)]; ok {
		return n
	}
	return s
}
