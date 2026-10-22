package wikipedia

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"timao-api/internal/banco"
)

// Páginas de edição das competições femininas na Wikipédia em inglês. Elas trazem
// jogos das Brabas de antes de 2022, que não têm página de temporada.
var competicoesFemininas = []struct {
	busca      string
	titulo     *regexp.Regexp
	competicao string
}{
	{`intitle:"Copa Libertadores Femenina"`, regexp.MustCompile(`^(\d{4}) Copa Libertadores Femenina$`), "Copa Libertadores"},
	{`intitle:"Campeonato Brasileiro de Futebol Feminino"`, regexp.MustCompile(`^(\d{4}) Campeonato Brasileiro de Futebol Feminino( Série A1?)?$`), "Campeonato Brasileiro"},
	{`intitle:"Copa do Brasil de Futebol Feminino"`, regexp.MustCompile(`^(\d{4}) Copa do Brasil de Futebol Feminino$`), "Copa do Brasil"},
	{`intitle:"Campeonato Paulista de Futebol Feminino"`, regexp.MustCompile(`^(\d{4}) Campeonato Paulista de Futebol Feminino$`), "Campeonato Paulista"},
	{`intitle:"Supercopa do Brasil de Futebol Feminino"`, regexp.MustCompile(`^(\d{4}) Supercopa do Brasil de Futebol Feminino$`), "Supercopa do Brasil"},
}

// CompeticoesFemininas lê as páginas de edição das competições femininas e
// devolve só as que têm jogos do Corinthians.
func (w *Wikipedia) CompeticoesFemininas(ctx context.Context) ([]Pagina, error) {
	var out []Pagina
	for _, c := range competicoesFemininas {
		titulos, err := w.buscar(ctx, c.busca, c.titulo)
		if err != nil {
			return nil, fmt.Errorf("wikipedia: buscando %s: %w", c.competicao, err)
		}
		for _, titulo := range titulos {
			if w.Pausa > 0 {
				select {
				case <-time.After(w.Pausa):
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			temporada, _ := strconv.Atoi(c.titulo.FindStringSubmatch(titulo)[1])
			texto, nome, rev, err := w.wikitexto(ctx, titulo)
			if err != nil {
				return nil, fmt.Errorf("wikipedia: %s: %w", titulo, err)
			}
			ps, avisos := ExtrairPartidasDaCompeticao(texto, banco.Feminino, temporada, c.competicao)
			if len(ps) == 0 {
				continue
			}
			OrdenarPartidas(ps)
			out = append(out, Pagina{
				Titulo: nome, Revisao: rev, URL: urlRevisao(rev),
				Categoria: banco.Feminino, Temporada: temporada, Partidas: ps, Avisos: avisos,
			})
		}
	}
	return out, nil
}
