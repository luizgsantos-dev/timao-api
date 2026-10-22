package acervo

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"timao-api/internal/banco"
)

type FiltroPessoas struct {
	Categoria string
	Funcao    string // jogador, tecnico ou vazio para os dois
	Nome      string // busca parcial, sem diferenciar acentos
	Posicao   string // busca parcial
	Paginacao
}

type ListaPessoas struct {
	Paginacao
	Pessoas []Pessoa `json:"pessoas"`
}

const colunasPessoa = `pe.id, pe.wikidata, pe.nome, pe.genero, pe.nascimento, pe.falecimento,
	pe.nacionalidade, pe.posicao, pe.foto_url, pe.wiki_pt, pe.wiki_en, f.nome, f.url, f.licenca`

// Pessoas lista quem passou pela categoria, em ordem alfabética.
func (a *Acervo) Pessoas(ctx context.Context, fp FiltroPessoas) (ListaPessoas, error) {
	fp.normalizar()

	var f filtro
	passagem := "pa.pessoa_id = pe.id AND pa.categoria = ?"
	args := []any{fp.Categoria}
	if fp.Funcao != "" {
		passagem += " AND pa.funcao = ?"
		args = append(args, fp.Funcao)
	}
	f.add("EXISTS (SELECT 1 FROM passagem pa WHERE "+passagem+")", args...)
	if fp.Nome != "" {
		f.add(`pe.nome_busca LIKE ? ESCAPE '\'`, contem(banco.Normalizar(fp.Nome)))
	}
	if fp.Posicao != "" {
		f.add(`lower(pe.posicao) LIKE ? ESCAPE '\'`, contem(strings.ToLower(fp.Posicao)))
	}

	out := ListaPessoas{Paginacao: fp.Paginacao, Pessoas: []Pessoa{}}
	if err := a.conn().QueryRowContext(ctx, `SELECT COUNT(*) FROM pessoa pe`+f.where(), f.args...).Scan(&out.Total); err != nil {
		return out, erroConsulta("contando pessoas", err)
	}

	rows, err := a.conn().QueryContext(ctx,
		`SELECT `+colunasPessoa+` FROM pessoa pe JOIN fonte f ON f.id = pe.fonte_id`+f.where()+
			` ORDER BY pe.nome_busca LIMIT ? OFFSET ?`,
		append(f.args, fp.Limite, fp.offset())...)
	if err != nil {
		return out, erroConsulta("listando pessoas", err)
	}
	defer rows.Close()
	for rows.Next() {
		p, err := lerPessoa(rows)
		if err != nil {
			return out, erroConsulta("lendo pessoa", err)
		}
		out.Pessoas = append(out.Pessoas, p)
	}
	if err := rows.Err(); err != nil {
		return out, erroConsulta("listando pessoas", err)
	}
	return out, a.preencherPassagens(ctx, fp.Categoria, out.Pessoas)
}

// Pessoa devolve uma pessoa pelo QID do Wikidata, se ela passou pela categoria.
func (a *Acervo) Pessoa(ctx context.Context, categoria, wikidata string) (Pessoa, error) {
	row := a.conn().QueryRowContext(ctx, `SELECT `+colunasPessoa+`
		FROM pessoa pe JOIN fonte f ON f.id = pe.fonte_id
		WHERE pe.wikidata = ? AND EXISTS (SELECT 1 FROM passagem pa WHERE pa.pessoa_id = pe.id AND pa.categoria = ?)`,
		strings.ToUpper(wikidata), categoria)
	p, err := lerPessoa(row)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNaoEncontrado
	}
	if err != nil {
		return p, erroConsulta("lendo pessoa", err)
	}
	ps := []Pessoa{p}
	if err := a.preencherPassagens(ctx, categoria, ps); err != nil {
		return p, err
	}
	p = ps[0]

	if p.wikiEN != "" {
		var n int
		if err := a.conn().QueryRowContext(ctx, `SELECT COUNT(*) FROM gol g JOIN partida p ON p.id = g.partida_id
			WHERE g.autor_wiki_en = ? AND g.lado = 'pro' AND g.tipo <> 'contra' AND p.categoria = ?`,
			p.wikiEN, categoria).Scan(&n); err != nil {
			return p, erroConsulta("contando gols", err)
		}
		p.GolsEmPartidas = &n
	}
	if p.Elencos, err = a.elencosDaPessoa(ctx, categoria, p.id); err != nil {
		return p, err
	}
	return p, nil
}

// GolsDaPessoa lista os gols da pessoa pelo Corinthians nas partidas do acervo, do mais antigo ao mais recente.
func (a *Acervo) GolsDaPessoa(ctx context.Context, categoria, wikidata string) ([]Gol, error) {
	p, err := a.Pessoa(ctx, categoria, wikidata)
	if err != nil {
		return nil, err
	}
	gols := []Gol{}
	if p.wikiEN == "" {
		return gols, nil
	}
	rows, err := a.conn().QueryContext(ctx, `SELECT g.minuto, g.autor, g.tipo, p.data, p.adversario, p.competicao
		FROM gol g JOIN partida p ON p.id = g.partida_id
		WHERE g.autor_wiki_en = ? AND g.lado = 'pro' AND g.tipo <> 'contra' AND p.categoria = ?
		ORDER BY p.data, g.id`, p.wikiEN, categoria)
	if err != nil {
		return nil, erroConsulta("listando gols", err)
	}
	defer rows.Close()
	for rows.Next() {
		var minuto, autor, comp sql.NullString
		g := Gol{Wikidata: p.Wikidata, Time: "corinthians", Partida: &Jogo{}}
		if err := rows.Scan(&minuto, &autor, &g.Tipo, &g.Partida.Data, &g.Partida.Adversario, &comp); err != nil {
			return nil, erroConsulta("lendo gol", err)
		}
		g.Minuto, g.Autor, g.Partida.Competicao = minuto.String, autor.String, comp.String
		gols = append(gols, g)
	}
	return gols, rows.Err()
}

type linha interface{ Scan(...any) error }

func lerPessoa(r linha) (Pessoa, error) {
	var p Pessoa
	var genero, nasc, morte, nac, pos, foto, wikiPT, wikiEN sql.NullString
	err := r.Scan(&p.id, &p.Wikidata, &p.Nome, &genero, &nasc, &morte, &nac, &pos, &foto, &wikiPT, &wikiEN,
		&p.Fonte.Nome, &p.Fonte.URL, &p.Fonte.Licenca)
	if err != nil {
		return p, err
	}
	p.Genero, p.Nascimento, p.Falecimento, p.FotoURL = genero.String, nasc.String, morte.String, foto.String
	p.Nacionalidade, p.Posicao = lista(nac), lista(pos)
	p.WikipediaPT, p.WikipediaEN = linkWiki("pt", wikiPT), linkWiki("en", wikiEN)
	p.wikiEN = wikiEN.String
	p.Passagens = []Passagem{}
	return p, nil
}

// preencherPassagens carrega, de uma vez, as passagens das pessoas na categoria.
func (a *Acervo) preencherPassagens(ctx context.Context, categoria string, ps []Pessoa) error {
	if len(ps) == 0 {
		return nil
	}
	idx := make(map[int64]int, len(ps))
	args := []any{categoria}
	for i, p := range ps {
		idx[p.id] = i
		args = append(args, p.id)
	}
	rows, err := a.conn().QueryContext(ctx, `SELECT pessoa_id, funcao, inicio, fim, jogos, gols FROM passagem
		WHERE categoria = ? AND pessoa_id IN (`+placeholders(len(ps))+`)
		ORDER BY pessoa_id, inicio IS NULL, inicio, id`, args...)
	if err != nil {
		return erroConsulta("listando passagens", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var pa Passagem
		var inicio, fim sql.NullString
		var jogos, gols sql.NullInt64
		if err := rows.Scan(&id, &pa.Funcao, &inicio, &fim, &jogos, &gols); err != nil {
			return erroConsulta("lendo passagem", err)
		}
		pa.Inicio, pa.Fim, pa.Jogos, pa.Gols = inicio.String, fim.String, inteiro(jogos), inteiro(gols)
		i := idx[id]
		ps[i].Passagens = append(ps[i].Passagens, pa)
	}
	return rows.Err()
}
