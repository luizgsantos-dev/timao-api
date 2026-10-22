// Package jogos cuida do calendário de jogos do Corinthians.
//
// Hoje os jogos vêm de um arquivo JSON editado à mão. O servidor só depende da
// interface JogosProvider, então dá para trocar por uma API esportiva depois
// sem mexer nos handlers.
package jogos

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

//go:embed jogos.exemplo.json
var exemploJSON []byte

// Categorias aceitas no campo categoria do calendário.
const (
	Masculino = "masculino"
	Feminino  = "feminino"
)

// Jogo é uma partida do calendário.
type Jogo struct {
	Data       string `json:"data"`              // AAAA-MM-DD
	Horario    string `json:"horario,omitempty"` // HH:MM, horário de Brasília
	Adversario string `json:"adversario"`
	Competicao string `json:"competicao"`
	Mando      string `json:"mando"`               // "casa" ou "fora"
	Categoria  string `json:"categoria,omitempty"` // "masculino" (padrão) ou "feminino"
	Local      string `json:"local,omitempty"`
}

// DaCategoria diz se o jogo é da categoria c. Jogo sem categoria conta como masculino.
func (j Jogo) DaCategoria(c string) bool {
	if j.Categoria == "" {
		return c == Masculino
	}
	return j.Categoria == c
}

// Static é um calendário carregado uma vez e mantido em memória.
type Static struct {
	jogos []Jogo
}

// FromJSON lê e valida um calendário em JSON (uma lista de jogos).
func FromJSON(b []byte) (*Static, error) {
	var lista []Jogo
	if err := json.Unmarshal(b, &lista); err != nil {
		return nil, fmt.Errorf("jogos: JSON inválido: %w", err)
	}
	for i := range lista {
		if err := validar(lista[i]); err != nil {
			return nil, fmt.Errorf("jogos: item %d (%s): %w", i+1, lista[i].Adversario, err)
		}
		if lista[i].Categoria == "" {
			lista[i].Categoria = Masculino
		}
	}
	sort.SliceStable(lista, func(a, b int) bool {
		if lista[a].Data != lista[b].Data {
			return lista[a].Data < lista[b].Data
		}
		return lista[a].Horario < lista[b].Horario
	})
	return &Static{jogos: lista}, nil
}

// FromFile lê o calendário de um arquivo JSON.
func FromFile(path string) (*Static, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("jogos: lendo %s: %w", path, err)
	}
	return FromJSON(b)
}

// Exemplo devolve o calendário de exemplo embutido no binário.
// Os adversários são fictícios: serve só para testar a API.
func Exemplo() (*Static, error) {
	return FromJSON(exemploJSON)
}

// Jogos devolve uma cópia da lista, ordenada por data e horário.
func (s *Static) Jogos(ctx context.Context) ([]Jogo, error) {
	return append([]Jogo(nil), s.jogos...), nil
}

func validar(j Jogo) error {
	if _, err := time.Parse("2006-01-02", j.Data); err != nil {
		return fmt.Errorf("data %q inválida, use AAAA-MM-DD", j.Data)
	}
	if j.Horario != "" {
		if _, err := time.Parse("15:04", j.Horario); err != nil {
			return fmt.Errorf("horário %q inválido, use HH:MM", j.Horario)
		}
	}
	if strings.TrimSpace(j.Adversario) == "" {
		return fmt.Errorf("adversário vazio")
	}
	if strings.TrimSpace(j.Competicao) == "" {
		return fmt.Errorf("competição vazia")
	}
	if j.Mando != "casa" && j.Mando != "fora" {
		return fmt.Errorf("mando %q inválido, use \"casa\" ou \"fora\"", j.Mando)
	}
	if j.Categoria != "" && j.Categoria != Masculino && j.Categoria != Feminino {
		return fmt.Errorf("categoria %q inválida, use \"masculino\" ou \"feminino\"", j.Categoria)
	}
	return nil
}
