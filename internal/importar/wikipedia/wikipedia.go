// Package wikipedia importa as partidas das páginas de temporada da Wikipédia em
// inglês ("2025 Sport Club Corinthians Paulista season" e "(women) season"), que
// trazem cada jogo numa predefinição {{football box}}. Conteúdo sob CC BY-SA 4.0.
package wikipedia

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"timao-api/internal/banco"
	"timao-api/internal/importar/coleta"
)

const (
	APIIngles = "https://en.wikipedia.org/w/api.php"
	Licenca   = "CC BY-SA 4.0"
)

var reTituloTemporada = regexp.MustCompile(`^(\d{4}) (?:Sport Club|SC) Corinthians Paulista (\(women\) )?season$`)

type Wikipedia struct {
	API    string // Wikipédia em inglês
	APIpt  string // Wikipédia em português
	Coleta *coleta.Cliente
	Pausa  time.Duration // entre páginas, para não pesar nos servidores da Wikimedia
}

func Novo(c *coleta.Cliente) *Wikipedia {
	return &Wikipedia{API: APIIngles, APIpt: APIPortugues, Coleta: c, Pausa: 500 * time.Millisecond}
}

// Pagina é uma página de temporada já lida.
type Pagina struct {
	Titulo    string
	Revisao   int64
	URL       string // link fixo para a revisão lida
	Lingua    string // en ou pt; vazio é en
	Categoria string
	Temporada int
	Partidas  []banco.Partida
	Elenco    []banco.Elenco // só nas páginas de temporada
	Tecnicos  []Tecnico      // idem
	Avisos    []Aviso
}

// Temporadas descobre as páginas de temporada e extrai as partidas de cada uma.
func (w *Wikipedia) Temporadas(ctx context.Context) ([]Pagina, error) {
	titulos, err := w.descobrir(ctx)
	if err != nil {
		return nil, fmt.Errorf("wikipedia: buscando páginas de temporada: %w", err)
	}
	var out []Pagina
	for i, t := range titulos {
		if i > 0 && w.Pausa > 0 {
			select {
			case <-time.After(w.Pausa):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		p, err := w.pagina(ctx, t)
		if err != nil {
			return nil, fmt.Errorf("wikipedia: %s: %w", t, err)
		}
		out = append(out, p)
	}
	return out, nil
}

// descobrir procura os títulos das páginas de temporada, masculinas e femininas.
func (w *Wikipedia) descobrir(ctx context.Context) ([]string, error) {
	return w.buscar(ctx, `intitle:"Corinthians Paulista" intitle:season`, reTituloTemporada)
}

// buscar devolve, em ordem alfabética, os títulos da busca que casam com padrao.
func (w *Wikipedia) buscar(ctx context.Context, consulta string, padrao *regexp.Regexp) ([]string, error) {
	return w.buscarEm(ctx, w.API, consulta, padrao)
}

// buscarEm é buscar na Wikipédia cuja API é api (outra língua).
func (w *Wikipedia) buscarEm(ctx context.Context, api, consulta string, padrao *regexp.Regexp) ([]string, error) {
	q := url.Values{
		"action":   {"query"},
		"list":     {"search"},
		"srsearch": {consulta},
		"srlimit":  {"500"},
		"format":   {"json"},
	}
	body, err := w.Coleta.Get(ctx, api+"?"+q.Encode(), "application/json")
	if err != nil {
		return nil, err
	}
	var r struct {
		Query struct {
			Search []struct {
				Title string `json:"title"`
			} `json:"search"`
		} `json:"query"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("resposta inválida: %w", err)
	}
	var titulos []string
	for _, s := range r.Query.Search {
		if padrao.MatchString(s.Title) {
			titulos = append(titulos, s.Title)
		}
	}
	sort.Strings(titulos)
	return titulos, nil
}

// MembrosDaCategoria lista os artigos de uma categoria da Wikipédia.
func (w *Wikipedia) MembrosDaCategoria(ctx context.Context, categoria string) ([]string, error) {
	var out []string
	continuar := ""
	for {
		q := url.Values{
			"action":  {"query"},
			"list":    {"categorymembers"},
			"cmtitle": {categoria},
			"cmtype":  {"page"},
			"cmlimit": {"500"},
			"format":  {"json"},
		}
		if continuar != "" {
			q.Set("cmcontinue", continuar)
		}
		body, err := w.Coleta.Get(ctx, w.API+"?"+q.Encode(), "application/json")
		if err != nil {
			return nil, fmt.Errorf("wikipedia: %s: %w", categoria, err)
		}
		var r struct {
			Query struct {
				Membros []struct {
					Title string `json:"title"`
				} `json:"categorymembers"`
			} `json:"query"`
			Continue struct {
				CM string `json:"cmcontinue"`
			} `json:"continue"`
		}
		if err := json.Unmarshal(body, &r); err != nil {
			return nil, fmt.Errorf("wikipedia: %s: resposta inválida: %w", categoria, err)
		}
		for _, m := range r.Query.Membros {
			out = append(out, m.Title)
		}
		if r.Continue.CM == "" {
			return out, nil
		}
		continuar = r.Continue.CM
	}
}

func (w *Wikipedia) pagina(ctx context.Context, titulo string) (Pagina, error) {
	m := reTituloTemporada.FindStringSubmatch(titulo)
	if m == nil {
		return Pagina{}, fmt.Errorf("título fora do padrão de temporada")
	}
	temporada, _ := strconv.Atoi(m[1])
	categoria := banco.Masculino
	if m[2] != "" {
		categoria = banco.Feminino
	}

	texto, nome, rev, err := w.wikitexto(ctx, titulo)
	if err != nil {
		return Pagina{}, err
	}

	partidas, avisos := ExtrairPartidas(texto, categoria, temporada)
	OrdenarPartidas(partidas)
	elenco := MesclarElenco(ExtrairElenco(texto, categoria), ExtrairEstatisticas(texto, categoria))
	for i := range elenco {
		elenco[i].Temporada = temporada
	}
	return Pagina{
		Titulo:    nome,
		Revisao:   rev,
		URL:       urlRevisao(rev),
		Categoria: categoria,
		Temporada: temporada,
		Partidas:  partidas,
		Elenco:    elenco,
		Tecnicos:  ExtrairTecnicos(texto),
		Avisos:    avisos,
	}, nil
}

// wikitexto busca o wikitexto atual de uma página, com o título final (após
// redirecionamentos) e o número da revisão lida.
func (w *Wikipedia) wikitexto(ctx context.Context, titulo string) (texto, nome string, rev int64, err error) {
	q := url.Values{
		"action":        {"parse"},
		"page":          {titulo},
		"prop":          {"wikitext|revid"},
		"redirects":     {"1"},
		"format":        {"json"},
		"formatversion": {"2"},
	}
	body, err := w.Coleta.Get(ctx, w.API+"?"+q.Encode(), "application/json")
	if err != nil {
		return "", "", 0, err
	}
	var r struct {
		Parse struct {
			Title    string `json:"title"`
			RevID    int64  `json:"revid"`
			Wikitext string `json:"wikitext"`
		} `json:"parse"`
		Error *struct {
			Info string `json:"info"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", "", 0, fmt.Errorf("resposta inválida: %w", err)
	}
	if r.Error != nil {
		return "", "", 0, fmt.Errorf("API: %s", r.Error.Info)
	}
	return r.Parse.Wikitext, r.Parse.Title, r.Parse.RevID, nil
}

// urlRevisao é o link fixo para a revisão lida, usado como fonte.
func urlRevisao(rev int64) string {
	return "https://en.wikipedia.org/w/index.php?oldid=" + strconv.FormatInt(rev, 10)
}

// ItensWikidata devolve o item do Wikidata (QID) de cada artigo pedido, seguindo
// redirecionamentos. Artigos sem item ou inexistentes ficam de fora do mapa.
func (w *Wikipedia) ItensWikidata(ctx context.Context, titulos []string) (map[string]string, error) {
	out := map[string]string{}
	const lote = 50 // limite da API por consulta
	for i := 0; i < len(titulos); i += lote {
		pedidos := titulos[i:min(i+lote, len(titulos))]
		q := url.Values{
			"action":    {"query"},
			"titles":    {strings.Join(pedidos, "|")},
			"prop":      {"pageprops"},
			"ppprop":    {"wikibase_item"},
			"redirects": {"1"},
			"format":    {"json"},
		}
		body, err := w.Coleta.Get(ctx, w.API+"?"+q.Encode(), "application/json")
		if err != nil {
			return nil, fmt.Errorf("wikipedia: itens do Wikidata: %w", err)
		}
		var r struct {
			Query struct {
				Normalized []struct{ From, To string } `json:"normalized"`
				Redirects  []struct{ From, To string } `json:"redirects"`
				Pages      map[string]struct {
					Title     string `json:"title"`
					PageProps struct {
						Item string `json:"wikibase_item"`
					} `json:"pageprops"`
				} `json:"pages"`
			} `json:"query"`
		}
		if err := json.Unmarshal(body, &r); err != nil {
			return nil, fmt.Errorf("wikipedia: itens do Wikidata: resposta inválida: %w", err)
		}
		porTitulo := map[string]string{}
		for _, p := range r.Query.Pages {
			if p.PageProps.Item != "" {
				porTitulo[p.Title] = p.PageProps.Item
			}
		}
		destino := func(t string) string {
			for _, n := range r.Query.Normalized {
				if n.From == t {
					t = n.To
				}
			}
			for _, n := range r.Query.Redirects {
				if n.From == t {
					t = n.To
				}
			}
			return t
		}
		for _, t := range pedidos {
			if qid, ok := porTitulo[destino(t)]; ok {
				out[t] = qid
			}
		}
	}
	return out, nil
}

// URLArtigo é o link de um artigo (ou categoria) da Wikipédia em inglês.
func URLArtigo(titulo string) string {
	return "https://en.wikipedia.org/wiki/" + url.PathEscape(strings.ReplaceAll(titulo, " ", "_"))
}
