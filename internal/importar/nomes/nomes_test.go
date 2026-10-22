package nomes

import "testing"

func TestFase(t *testing.T) {
	casos := map[string]string{
		"Match 7":         "Rodada 7",
		"Matchday 12":     "Rodada 12",
		"12":              "Rodada 12",
		"Second stage":    "Segunda fase",
		"Semi-finals":     "Semifinal",
		"Quarterfinals":   "Quartas de final",
		"Round of 16":     "Oitavas de final",
		"Gruppe 6":        "Grupo 6",
		"Group b":         "Grupo B",
		"Teal Rising Cup": "Teal Rising Cup",
	}
	for in, want := range casos {
		if got := Fase(in); got != want {
			t.Errorf("Fase(%q) = %q, esperava %q", in, got, want)
		}
	}
}

func TestCompeticao(t *testing.T) {
	if got := Competicao("Sudamericana"); got != "Copa Sul-Americana" {
		t.Errorf("Sudamericana: %q", got)
	}
	if got := Competicao("Campeonato Paulista"); got != "Campeonato Paulista" {
		t.Errorf("nome desconhecido deveria ficar como veio: %q", got)
	}
}
