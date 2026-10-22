// Package brasileirao importa os jogos do Campeonato Brasileiro (Série A, desde
// 2003) do pacote R "brasileirao", de William Amorim, publicado sob a licença MIT.
// Traz data, mandante, visitante e placar; não traz fase, horário nem gols.
package brasileirao

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"timao-api/internal/banco"
	"timao-api/internal/importar/coleta"
)

const (
	URL     = "https://raw.githubusercontent.com/williamorim/brasileirao/master/data-raw/csv/matches.csv"
	Pagina  = "https://github.com/williamorim/brasileirao"
	Licenca = "MIT"
)

type Brasileirao struct {
	URL    string
	Coleta *coleta.Cliente
}

func Novo(c *coleta.Cliente) *Brasileirao { return &Brasileirao{URL: URL, Coleta: c} }

// Partidas baixa o CSV e devolve os jogos do Corinthians.
func (b *Brasileirao) Partidas(ctx context.Context) ([]banco.Partida, error) {
	body, err := b.Coleta.Get(ctx, b.URL, "text/csv")
	if err != nil {
		return nil, fmt.Errorf("brasileirao: %w", err)
	}
	ps, err := Extrair(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("brasileirao: %w", err)
	}
	return ps, nil
}

var (
	rePlacar      = regexp.MustCompile(`^\s*(\d+)\s*x\s*(\d+)\s*$`)
	reCorinthians = regexp.MustCompile(`(?i)^corinthians$`)
)

// Extrair lê o CSV (id_match, season, date, home, score, away).
func Extrair(r io.Reader) ([]banco.Partida, error) {
	leitor := csv.NewReader(r)
	cab, err := leitor.Read()
	if err != nil {
		return nil, fmt.Errorf("lendo cabeçalho: %w", err)
	}
	col := map[string]int{}
	for i, c := range cab {
		col[strings.TrimSpace(c)] = i
	}
	for _, c := range []string{"season", "date", "home", "score", "away"} {
		if _, ok := col[c]; !ok {
			return nil, fmt.Errorf("coluna %q ausente no CSV", c)
		}
	}

	var out []banco.Partida
	for {
		reg, err := leitor.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("lendo linha: %w", err)
		}
		mandante, visitante := strings.TrimSpace(reg[col["home"]]), strings.TrimSpace(reg[col["away"]])
		casa, fora := reCorinthians.MatchString(mandante), reCorinthians.MatchString(visitante)
		if casa == fora {
			continue
		}
		data := strings.TrimSpace(reg[col["date"]])
		if _, err := time.Parse("2006-01-02", data); err != nil {
			continue
		}
		temporada, err := strconv.Atoi(strings.TrimSpace(reg[col["season"]]))
		if err != nil {
			continue
		}
		p := banco.Partida{
			Categoria:  banco.Masculino,
			Temporada:  temporada,
			Data:       data,
			Competicao: "Campeonato Brasileiro",
			Mando:      "casa",
			Adversario: visitante,
		}
		if fora {
			p.Mando, p.Adversario = "fora", mandante
		}
		if m := rePlacar.FindStringSubmatch(reg[col["score"]]); m != nil {
			a, _ := strconv.Atoi(m[1])
			b, _ := strconv.Atoi(m[2])
			if fora {
				a, b = b, a
			}
			p.GolsPro, p.GolsContra = &a, &b
		}
		out = append(out, p)
	}
	return out, nil
}
