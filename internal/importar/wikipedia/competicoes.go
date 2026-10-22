package wikipedia

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"timao-api/internal/banco"
)

// Páginas de edição de competições (ex.: "2010 Copa Libertadores second stage",
// "Campeonato Paulista de Futebol de 2003"). Elas completam as partidas dos anos
// sem página de temporada: jogos das Brabas antes de 2022 e do masculino em
// 2003–2007, 2009–2011 e 2014, além de anos anteriores. Em geral só o mata-mata e
// as fases de grupos das copas continentais vêm em caixas de jogo.

const APIPortugues = "https://pt.wikipedia.org/w/api.php"

type paginaDeCompeticao struct {
	categoria  string
	lingua     string // en ou pt
	busca      string
	titulo     *regexp.Regexp // o grupo 1 é o ano
	competicao string
}

var paginasDeCompeticao = []paginaDeCompeticao{
	// Brabas, em inglês.
	{banco.Feminino, "en", `intitle:"Copa Libertadores Femenina"`, regexp.MustCompile(`^(\d{4}) Copa Libertadores Femenina$`), "Copa Libertadores"},
	{banco.Feminino, "en", `intitle:"Campeonato Brasileiro de Futebol Feminino"`, regexp.MustCompile(`^(\d{4}) Campeonato Brasileiro de Futebol Feminino( Série A1?)?$`), "Campeonato Brasileiro"},
	{banco.Feminino, "en", `intitle:"Copa do Brasil de Futebol Feminino"`, regexp.MustCompile(`^(\d{4}) Copa do Brasil de Futebol Feminino$`), "Copa do Brasil"},
	{banco.Feminino, "en", `intitle:"Campeonato Paulista de Futebol Feminino"`, regexp.MustCompile(`^(\d{4}) Campeonato Paulista de Futebol Feminino$`), "Campeonato Paulista"},
	{banco.Feminino, "en", `intitle:"Supercopa do Brasil de Futebol Feminino"`, regexp.MustCompile(`^(\d{4}) Supercopa do Brasil de Futebol Feminino$`), "Supercopa do Brasil"},
	// Brabas, em português (mata-mata com gols, e edições que não existem em inglês).
	{banco.Feminino, "pt", `intitle:"Campeonato Paulista de Futebol Feminino"`, regexp.MustCompile(`^Campeonato Paulista de Futebol Feminino de (\d{4})$`), "Campeonato Paulista"},
	{banco.Feminino, "pt", `intitle:"Campeonato Brasileiro de Futebol Feminino"`, regexp.MustCompile(`^Campeonato Brasileiro de Futebol Feminino de (\d{4})( - Série A1?)?$`), "Campeonato Brasileiro"},
	{banco.Feminino, "pt", `intitle:"Copa do Brasil de Futebol Feminino"`, regexp.MustCompile(`^Copa do Brasil de Futebol Feminino de (\d{4})$`), "Copa do Brasil"},

	// Masculino, em inglês: copas continentais e Mundial (com as subpáginas de fase).
	{banco.Masculino, "en", `intitle:"Copa Libertadores" -Femenina`, regexp.MustCompile(`(?i)^(\d{4}) Copa Libertadores(?: (?:first|second|third|group|knockout|final|qualifying) stages?| finals?)?$`), "Copa Libertadores"},
	{banco.Masculino, "en", `intitle:"Copa Sudamericana"`, regexp.MustCompile(`(?i)^(\d{4}) Copa Sudamericana(?: (?:first|second|group|knockout|final) stages?| finals?)?$`), "Copa Sul-Americana"},
	{banco.Masculino, "en", `intitle:"Recopa Sudamericana"`, regexp.MustCompile(`^(\d{4}) Recopa Sudamericana$`), "Recopa Sul-Americana"},
	{banco.Masculino, "en", `intitle:"FIFA Club World"`, regexp.MustCompile(`^(\d{4}) FIFA Club World (?:Cup|Championship)(?: [Ff]inal)?$`), "Mundial de Clubes da FIFA"},
	// Masculino, em português: Paulista e Copa do Brasil (finais e mata-mata).
	{banco.Masculino, "pt", `intitle:"Campeonato Paulista de Futebol de"`, regexp.MustCompile(`^Campeonato Paulista de Futebol de (\d{4})( - Série A1)?$`), "Campeonato Paulista"},
	{banco.Masculino, "pt", `intitle:"Copa do Brasil de Futebol de"`, regexp.MustCompile(`^Copa do Brasil de Futebol de (\d{4})$`), "Copa do Brasil"},
}

// Competicoes lê as páginas de edição de competição dos anos que não estão em
// cobertos (os anos com página de temporada, por categoria, que já trazem todos os
// jogos) e devolve só as páginas com jogos do Corinthians.
func (w *Wikipedia) Competicoes(ctx context.Context, cobertos map[string]map[int]bool) ([]Pagina, error) {
	var out []Pagina
	for _, c := range paginasDeCompeticao {
		api := w.API
		if c.lingua == "pt" {
			api = w.APIpt
		}
		titulos, err := w.buscarEm(ctx, api, c.busca, c.titulo)
		if err != nil {
			return nil, fmt.Errorf("wikipedia: buscando %s (%s): %w", c.competicao, c.lingua, err)
		}
		var pedidos []string
		for _, t := range titulos {
			ano, _ := strconv.Atoi(c.titulo.FindStringSubmatch(t)[1])
			if !cobertos[c.categoria][ano] {
				pedidos = append(pedidos, t)
			}
		}
		paginas, err := w.wikitextos(ctx, api, pedidos)
		if err != nil {
			return nil, fmt.Errorf("wikipedia: lendo %s (%s): %w", c.competicao, c.lingua, err)
		}
		for _, t := range pedidos {
			pg, ok := paginas[t]
			if !ok {
				continue
			}
			ano, _ := strconv.Atoi(c.titulo.FindStringSubmatch(t)[1])
			ps, avisos := ExtrairPartidasDaCompeticao(pg.texto, c.categoria, ano, c.competicao)
			if len(ps) == 0 {
				continue
			}
			OrdenarPartidas(ps)
			out = append(out, Pagina{
				Titulo: pg.titulo, Revisao: pg.rev, URL: urlRevisaoEm(c.lingua, pg.rev), Lingua: c.lingua,
				Categoria: c.categoria, Temporada: ano, Partidas: ps, Avisos: avisos,
			})
		}
	}
	return out, nil
}

type textoDePagina struct {
	titulo, texto string
	rev           int64
}

// wikitextos busca o wikitexto atual de várias páginas, em lotes de 20 por
// requisição. O mapa é indexado pelo título pedido (seguindo redirecionamentos).
func (w *Wikipedia) wikitextos(ctx context.Context, api string, titulos []string) (map[string]textoDePagina, error) {
	out := map[string]textoDePagina{}
	const lote = 20
	for i := 0; i < len(titulos); i += lote {
		if i > 0 && w.Pausa > 0 {
			select {
			case <-time.After(w.Pausa):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		pedidos := titulos[i:min(i+lote, len(titulos))]
		q := url.Values{
			"action":        {"query"},
			"titles":        {strings.Join(pedidos, "|")},
			"prop":          {"revisions"},
			"rvprop":        {"content|ids"},
			"rvslots":       {"main"},
			"redirects":     {"1"},
			"format":        {"json"},
			"formatversion": {"2"},
		}
		body, err := w.Coleta.Get(ctx, api+"?"+q.Encode(), "application/json")
		if err != nil {
			return nil, err
		}
		var r struct {
			Query struct {
				Redirects []struct{ From, To string } `json:"redirects"`
				Pages     []struct {
					Title     string `json:"title"`
					Missing   bool   `json:"missing"`
					Revisions []struct {
						RevID int64 `json:"revid"`
						Slots struct {
							Main struct {
								Content string `json:"content"`
							} `json:"main"`
						} `json:"slots"`
					} `json:"revisions"`
				} `json:"pages"`
			} `json:"query"`
		}
		if err := json.Unmarshal(body, &r); err != nil {
			return nil, fmt.Errorf("resposta inválida: %w", err)
		}
		porTitulo := map[string]textoDePagina{}
		for _, p := range r.Query.Pages {
			if p.Missing || len(p.Revisions) == 0 {
				continue
			}
			porTitulo[p.Title] = textoDePagina{titulo: p.Title, texto: p.Revisions[0].Slots.Main.Content, rev: p.Revisions[0].RevID}
		}
		for _, t := range pedidos {
			destino := t
			for _, rd := range r.Query.Redirects {
				if rd.From == t {
					destino = rd.To
				}
			}
			if pg, ok := porTitulo[destino]; ok {
				out[t] = pg
			}
		}
	}
	return out, nil
}

// urlRevisaoEm é o link fixo de uma revisão na Wikipédia da língua dada.
func urlRevisaoEm(lingua string, rev int64) string {
	return "https://" + lingua + ".wikipedia.org/w/index.php?oldid=" + strconv.FormatInt(rev, 10)
}
