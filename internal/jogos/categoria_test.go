package jogos

import (
	"context"
	"strings"
	"testing"
)

func TestSemCategoriaViraMasculino(t *testing.T) {
	s, err := FromJSON([]byte(`[{"data":"2026-11-02","adversario":"A","competicao":"X","mando":"casa"}]`))
	if err != nil {
		t.Fatalf("FromJSON: %v", err)
	}
	lista, _ := s.Jogos(context.Background())
	if lista[0].Categoria != Masculino {
		t.Fatalf("categoria esperada %q, veio %q", Masculino, lista[0].Categoria)
	}
}

func TestCategoriaInvalida(t *testing.T) {
	_, err := FromJSON([]byte(`[{"data":"2026-11-02","adversario":"A","competicao":"X","mando":"casa","categoria":"sub-20"}]`))
	if err == nil || !strings.Contains(err.Error(), "categoria") {
		t.Fatalf("esperava erro de categoria, veio %v", err)
	}
}

func TestDaCategoria(t *testing.T) {
	casos := []struct {
		nome string
		jogo Jogo
		cat  string
		want bool
	}{
		{"vazio é masculino", Jogo{}, Masculino, true},
		{"vazio não é feminino", Jogo{}, Feminino, false},
		{"feminino explícito", Jogo{Categoria: Feminino}, Feminino, true},
		{"feminino não é masculino", Jogo{Categoria: Feminino}, Masculino, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := c.jogo.DaCategoria(c.cat); got != c.want {
				t.Fatalf("DaCategoria(%q) = %v, esperava %v", c.cat, got, c.want)
			}
		})
	}
}

func TestExemploTemAsDuasCategorias(t *testing.T) {
	s, err := Exemplo()
	if err != nil {
		t.Fatalf("Exemplo: %v", err)
	}
	lista, _ := s.Jogos(context.Background())
	var masc, fem int
	for _, j := range lista {
		switch j.Categoria {
		case Masculino:
			masc++
		case Feminino:
			fem++
		}
	}
	if masc == 0 || fem == 0 {
		t.Fatalf("o exemplo deveria ter jogos das duas categorias, veio masculino=%d feminino=%d", masc, fem)
	}
}
