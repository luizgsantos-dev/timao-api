package acervo

import (
	"context"
	"database/sql"
)

type Integrante struct {
	Nome          string `json:"nome"`
	Wikidata      string `json:"wikidata,omitempty"` // quando a pessoa está no acervo
	Numero        *int   `json:"numero,omitempty"`
	Posicao       string `json:"posicao,omitempty"`
	Nacionalidade string `json:"nacionalidade,omitempty"` // código FIFA, ex.: BRA
	Jogos         *int   `json:"jogos,omitempty"`         // na temporada, quando a fonte tem estatísticas
	Gols          *int   `json:"gols,omitempty"`
}

type Elenco struct {
	Temporada  int          `json:"temporada"`
	Temporadas []int        `json:"temporadas_disponiveis"`
	Elenco     []Integrante `json:"elenco"`
	Fonte      *Fonte       `json:"fonte,omitempty"`
}

// ElencoDaPessoa é a presença da pessoa no elenco de uma temporada.
type ElencoDaPessoa struct {
	Temporada int    `json:"temporada"`
	Numero    *int   `json:"numero,omitempty"`
	Posicao   string `json:"posicao,omitempty"`
	Jogos     *int   `json:"jogos,omitempty"`
	Gols      *int   `json:"gols,omitempty"`
}

// Elenco devolve o elenco da categoria numa temporada, ordenado pelo número da
// camisa. Com temporada 0, devolve a mais recente.
func (a *Acervo) Elenco(ctx context.Context, categoria string, temporada int) (Elenco, error) {
	out := Elenco{Temporadas: []int{}, Elenco: []Integrante{}}
	rows, err := a.conn().QueryContext(ctx, `SELECT DISTINCT temporada FROM elenco WHERE categoria = ? ORDER BY temporada`, categoria)
	if err != nil {
		return out, erroConsulta("listando temporadas do elenco", err)
	}
	for rows.Next() {
		var t int
		if err := rows.Scan(&t); err != nil {
			rows.Close()
			return out, erroConsulta("lendo temporada", err)
		}
		out.Temporadas = append(out.Temporadas, t)
	}
	rows.Close()
	if len(out.Temporadas) == 0 {
		return out, ErrNaoEncontrado
	}
	if temporada == 0 {
		temporada = out.Temporadas[len(out.Temporadas)-1]
	}
	out.Temporada = temporada

	rows, err = a.conn().QueryContext(ctx, `SELECT e.nome, pe.wikidata, e.numero, e.posicao, e.nacionalidade, e.jogos, e.gols, f.nome, f.url, f.licenca
		FROM elenco e JOIN fonte f ON f.id = e.fonte_id LEFT JOIN pessoa pe ON pe.id = e.pessoa_id
		WHERE e.categoria = ? AND e.temporada = ?
		ORDER BY e.numero IS NULL, e.numero, e.nome`, categoria, temporada)
	if err != nil {
		return out, erroConsulta("listando elenco", err)
	}
	defer rows.Close()
	for rows.Next() {
		var i Integrante
		var wd, pos, nac sql.NullString
		var num, jogos, gols sql.NullInt64
		var f Fonte
		if err := rows.Scan(&i.Nome, &wd, &num, &pos, &nac, &jogos, &gols, &f.Nome, &f.URL, &f.Licenca); err != nil {
			return out, erroConsulta("lendo integrante", err)
		}
		i.Wikidata, i.Numero, i.Posicao, i.Nacionalidade = wd.String, inteiro(num), pos.String, nac.String
		i.Jogos, i.Gols = inteiro(jogos), inteiro(gols)
		out.Elenco = append(out.Elenco, i)
		out.Fonte = &f
	}
	if err := rows.Err(); err != nil {
		return out, erroConsulta("listando elenco", err)
	}
	if len(out.Elenco) == 0 {
		return out, ErrNaoEncontrado
	}
	return out, nil
}

// elencosDaPessoa lista as temporadas em que a pessoa esteve no elenco da categoria.
func (a *Acervo) elencosDaPessoa(ctx context.Context, categoria string, pessoaID int64) ([]ElencoDaPessoa, error) {
	rows, err := a.conn().QueryContext(ctx, `SELECT temporada, numero, posicao, jogos, gols FROM elenco
		WHERE categoria = ? AND pessoa_id = ? ORDER BY temporada`, categoria, pessoaID)
	if err != nil {
		return nil, erroConsulta("listando elencos da pessoa", err)
	}
	defer rows.Close()
	var out []ElencoDaPessoa
	for rows.Next() {
		var e ElencoDaPessoa
		var num, jogos, gols sql.NullInt64
		var pos sql.NullString
		if err := rows.Scan(&e.Temporada, &num, &pos, &jogos, &gols); err != nil {
			return nil, erroConsulta("lendo elenco da pessoa", err)
		}
		e.Numero, e.Posicao, e.Jogos, e.Gols = inteiro(num), pos.String, inteiro(jogos), inteiro(gols)
		out = append(out, e)
	}
	return out, rows.Err()
}
