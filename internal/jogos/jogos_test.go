package jogos

import (
	"context"
	"strings"
	"testing"
)

func TestExemploCarrega(t *testing.T) {
	s, err := Exemplo()
	if err != nil {
		t.Fatalf("Exemplo: %v", err)
	}
	lista, _ := s.Jogos(context.Background())
	if len(lista) == 0 {
		t.Fatal("calendário de exemplo vazio")
	}
}

func TestOrdenaPorDataEHorario(t *testing.T) {
	s, err := FromJSON([]byte(`[
		{"data":"2026-11-02","horario":"20:00","adversario":"B","competicao":"X","mando":"fora"},
		{"data":"2026-10-25","horario":"16:00","adversario":"A","competicao":"X","mando":"casa"},
		{"data":"2026-11-02","horario":"18:00","adversario":"C","competicao":"X","mando":"casa"}
	]`))
	if err != nil {
		t.Fatalf("FromJSON: %v", err)
	}
	lista, _ := s.Jogos(context.Background())
	got := lista[0].Adversario + lista[1].Adversario + lista[2].Adversario
	if got != "ACB" {
		t.Fatalf("ordem esperada ACB, veio %s", got)
	}
}

func TestValidacao(t *testing.T) {
	casos := []struct {
		nome   string
		json   string
		contem string
	}{
		{"json quebrado", `{`, "JSON inválido"},
		{"data inválida", `[{"data":"02/11/2026","adversario":"A","competicao":"X","mando":"casa"}]`, "AAAA-MM-DD"},
		{"horário inválido", `[{"data":"2026-11-02","horario":"25:00","adversario":"A","competicao":"X","mando":"casa"}]`, "HH:MM"},
		{"adversário vazio", `[{"data":"2026-11-02","adversario":" ","competicao":"X","mando":"casa"}]`, "adversário vazio"},
		{"competição vazia", `[{"data":"2026-11-02","adversario":"A","competicao":"","mando":"casa"}]`, "competição vazia"},
		{"mando inválido", `[{"data":"2026-11-02","adversario":"A","competicao":"X","mando":"neutro"}]`, "mando"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			_, err := FromJSON([]byte(c.json))
			if err == nil {
				t.Fatal("esperava erro")
			}
			if !strings.Contains(err.Error(), c.contem) {
				t.Fatalf("erro %q deveria conter %q", err, c.contem)
			}
		})
	}
}

func TestFromFileInexistente(t *testing.T) {
	if _, err := FromFile("/caminho/que/nao/existe.json"); err == nil {
		t.Fatal("esperava erro")
	}
}
