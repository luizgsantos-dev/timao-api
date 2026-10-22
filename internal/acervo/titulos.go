package acervo

import (
	"context"
	"database/sql"
	"sort"
	"strings"
)

// Conquista reúne os títulos de uma competição.
type Conquista struct {
	Competicao string   `json:"competicao"`
	Slug       string   `json:"slug"`
	Escopo     string   `json:"escopo"`
	Oficial    bool     `json:"oficial"`
	Quantidade int      `json:"quantidade"`
	Anos       []int    `json:"anos"`
	Notas      []Nota   `json:"notas,omitempty"`
	Edicoes    []Edicao `json:"edicoes,omitempty"` // só no detalhe
	Fonte      Fonte    `json:"fonte"`
}

type Nota struct {
	Ano   int    `json:"ano"`
	Texto string `json:"texto"`
}

type Edicao struct {
	Ano         int    `json:"ano"`
	WikipediaEN string `json:"wikipedia_en,omitempty"`
	Nota        string `json:"nota,omitempty"`
}

type FiltroTitulos struct {
	Categoria string
	Escopo    string
	Tipo      string // oficial (padrão), outros ou todos
}

// Ordem de exibição dos escopos, do mais importante ao menos.
var ordemEscopo = map[string]int{
	"mundial": 0, "continental": 1, "nacional": 2, "interestadual": 3, "estadual": 4,
	"internacional": 5, "amistoso": 6, "outros": 7,
}

// Titulos lista as conquistas da categoria por competição: primeiro as oficiais,
// do escopo mundial ao estadual, na ordem em que a fonte as apresenta.
func (a *Acervo) Titulos(ctx context.Context, ft FiltroTitulos) ([]Conquista, error) {
	var f filtro
	f.add("t.categoria = ?", ft.Categoria)
	switch ft.Tipo {
	case "", "oficial":
		f.add("t.oficial = 1")
	case "outros":
		f.add("t.oficial = 0")
	}
	if ft.Escopo != "" {
		f.add("t.escopo = ?", strings.ToLower(ft.Escopo))
	}
	return a.conquistas(ctx, f, false)
}

// Titulo devolve a conquista de uma competição pelo slug, com as edições.
func (a *Acervo) Titulo(ctx context.Context, categoria, slug string) (Conquista, error) {
	var f filtro
	f.add("t.categoria = ?", categoria)
	f.add("t.slug = ?", strings.ToLower(slug))
	cs, err := a.conquistas(ctx, f, true)
	if err != nil {
		return Conquista{}, err
	}
	if len(cs) == 0 {
		return Conquista{}, ErrNaoEncontrado
	}
	return cs[0], nil
}

func (a *Acervo) conquistas(ctx context.Context, f filtro, comEdicoes bool) ([]Conquista, error) {
	rows, err := a.conn().QueryContext(ctx, `SELECT t.competicao, t.slug, t.escopo, t.oficial, t.ano, t.edicao_wiki_en, t.nota,
		f.nome, f.url, f.licenca
		FROM titulo t JOIN fonte f ON f.id = t.fonte_id`+f.where()+` ORDER BY t.id`, f.args...)
	if err != nil {
		return nil, erroConsulta("listando títulos", err)
	}
	defer rows.Close()

	porSlug := map[string]*Conquista{}
	var ordem []string
	for rows.Next() {
		var c Conquista
		var ano int
		var wiki, nota sql.NullString
		if err := rows.Scan(&c.Competicao, &c.Slug, &c.Escopo, &c.Oficial, &ano, &wiki, &nota,
			&c.Fonte.Nome, &c.Fonte.URL, &c.Fonte.Licenca); err != nil {
			return nil, erroConsulta("lendo título", err)
		}
		atual, ok := porSlug[c.Slug]
		if !ok {
			c.Anos = []int{}
			atual = &c
			porSlug[c.Slug] = atual
			ordem = append(ordem, c.Slug)
		}
		atual.Quantidade++
		atual.Anos = append(atual.Anos, ano)
		if nota.String != "" {
			atual.Notas = append(atual.Notas, Nota{Ano: ano, Texto: nota.String})
		}
		if comEdicoes {
			atual.Edicoes = append(atual.Edicoes, Edicao{Ano: ano, WikipediaEN: linkWiki("en", wiki), Nota: nota.String})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, erroConsulta("listando títulos", err)
	}

	out := make([]Conquista, 0, len(ordem))
	for _, s := range ordem {
		c := porSlug[s]
		sort.Ints(c.Anos)
		sort.Slice(c.Edicoes, func(i, j int) bool { return c.Edicoes[i].Ano < c.Edicoes[j].Ano })
		out = append(out, *c)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Oficial != out[j].Oficial {
			return out[i].Oficial
		}
		return ordemEscopo[out[i].Escopo] < ordemEscopo[out[j].Escopo]
	})
	return out, nil
}
