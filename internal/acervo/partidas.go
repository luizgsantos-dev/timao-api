package acervo

import (
	"context"
	"database/sql"
	"errors"

	"timao-api/internal/banco"
)

type FiltroPartidas struct {
	Categoria  string
	Temporada  int
	Competicao string // busca parcial
	Adversario string // busca parcial, sem diferenciar acentos
	Mando      string // casa ou fora
	Resultado  string // vitoria, empate ou derrota
	De, Ate    string // AAAA-MM-DD, inclusive
	Crescente  bool   // padrão: mais recentes primeiro
	Paginacao
}

type ListaPartidas struct {
	Paginacao
	// Resumo considera todas as partidas do filtro já disputadas, não só as da página.
	Resumo   Resumo    `json:"resumo"`
	Partidas []Partida `json:"partidas"`
}

// resultadoSQL classifica pelo placar do tempo normal (e prorrogação); pênaltis não mudam o resultado.
const resultadoSQL = `CASE WHEN p.gols_pro IS NULL THEN NULL
	WHEN p.gols_pro > p.gols_contra THEN 'vitoria'
	WHEN p.gols_pro < p.gols_contra THEN 'derrota' ELSE 'empate' END`

const colunasPartida = `p.id, p.data, p.horario, p.temporada, p.competicao, p.fase, p.adversario, p.mando,
	p.gols_pro, p.gols_contra, p.penaltis_pro, p.penaltis_contra, ` + resultadoSQL + `,
	p.estadio, p.cidade, p.publico, p.arbitro, f.nome, f.url, f.licenca`

func (fp FiltroPartidas) filtro() filtro {
	var f filtro
	f.add("p.categoria = ?", fp.Categoria)
	if fp.Temporada > 0 {
		f.add("p.temporada = ?", fp.Temporada)
	}
	if fp.Competicao != "" {
		f.add(`p.competicao LIKE ? ESCAPE '\'`, contem(fp.Competicao))
	}
	if fp.Adversario != "" {
		f.add(`p.adversario_busca LIKE ? ESCAPE '\'`, contem(banco.Normalizar(fp.Adversario)))
	}
	if fp.Mando != "" {
		f.add("p.mando = ?", fp.Mando)
	}
	if fp.Resultado != "" {
		f.add("("+resultadoSQL+") = ?", fp.Resultado)
	}
	if fp.De != "" {
		f.add("p.data >= ?", fp.De)
	}
	if fp.Ate != "" {
		f.add("p.data <= ?", fp.Ate)
	}
	return f
}

// Partidas lista as partidas da categoria com o resumo de desempenho do filtro.
func (a *Acervo) Partidas(ctx context.Context, fp FiltroPartidas) (ListaPartidas, error) {
	fp.normalizar()
	f := fp.filtro()
	out := ListaPartidas{Paginacao: fp.Paginacao, Partidas: []Partida{}}

	var err error
	out.Total, out.Resumo, err = a.resumo(ctx, f)
	if err != nil {
		return out, err
	}

	ordem := "DESC"
	if fp.Crescente {
		ordem = "ASC"
	}
	rows, err := a.conn().QueryContext(ctx,
		`SELECT `+colunasPartida+` FROM partida p JOIN fonte f ON f.id = p.fonte_id`+f.where()+
			` ORDER BY p.data `+ordem+` LIMIT ? OFFSET ?`,
		append(f.args, fp.Limite, fp.offset())...)
	if err != nil {
		return out, erroConsulta("listando partidas", err)
	}
	defer rows.Close()
	for rows.Next() {
		p, err := lerPartida(rows)
		if err != nil {
			return out, erroConsulta("lendo partida", err)
		}
		out.Partidas = append(out.Partidas, p)
	}
	return out, rows.Err()
}

// resumo conta as partidas do filtro e soma o desempenho das já disputadas.
func (a *Acervo) resumo(ctx context.Context, f filtro) (int, Resumo, error) {
	var total int
	var r Resumo
	err := a.conn().QueryRowContext(ctx, `SELECT COUNT(*),
		COUNT(p.gols_pro),
		COALESCE(SUM(p.gols_pro > p.gols_contra), 0),
		COALESCE(SUM(p.gols_pro = p.gols_contra), 0),
		COALESCE(SUM(p.gols_pro < p.gols_contra), 0),
		COALESCE(SUM(p.gols_pro), 0), COALESCE(SUM(p.gols_contra), 0)
		FROM partida p`+f.where(), f.args...).
		Scan(&total, &r.Jogos, &r.Vitorias, &r.Empates, &r.Derrotas, &r.GolsPro, &r.GolsContra)
	if err != nil {
		return 0, r, erroConsulta("resumindo partidas", err)
	}
	return total, r, nil
}

// Partida devolve a partida da categoria na data (AAAA-MM-DD), com os gols.
func (a *Acervo) Partida(ctx context.Context, categoria, data string) (Partida, error) {
	row := a.conn().QueryRowContext(ctx, `SELECT `+colunasPartida+`
		FROM partida p JOIN fonte f ON f.id = p.fonte_id
		WHERE p.categoria = ? AND p.data = ? ORDER BY p.id LIMIT 1`, categoria, data)
	p, err := lerPartida(row)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNaoEncontrado
	}
	if err != nil {
		return p, erroConsulta("lendo partida", err)
	}

	rows, err := a.conn().QueryContext(ctx, `SELECT g.minuto, g.autor, g.tipo, g.lado,
		(SELECT pe.wikidata FROM pessoa pe WHERE pe.wiki_en = g.autor_wiki_en LIMIT 1)
		FROM gol g WHERE g.partida_id = ? ORDER BY g.id`, p.id)
	if err != nil {
		return p, erroConsulta("listando gols", err)
	}
	defer rows.Close()
	p.Gols = []Gol{}
	for rows.Next() {
		var minuto, autor, wd sql.NullString
		var g Gol
		var lado string
		if err := rows.Scan(&minuto, &autor, &g.Tipo, &lado, &wd); err != nil {
			return p, erroConsulta("lendo gol", err)
		}
		g.Minuto, g.Autor, g.Wikidata = minuto.String, autor.String, wd.String
		g.Time = "adversario"
		if lado == "pro" {
			g.Time = "corinthians"
		}
		if g.Tipo == "contra" {
			g.Wikidata = "" // o autor de gol contra é do outro time; não ligamos à pessoa
		}
		p.Gols = append(p.Gols, g)
	}
	return p, rows.Err()
}

func lerPartida(r linha) (Partida, error) {
	var p Partida
	var horario, comp, fase, resultado, estadio, cidade, arbitro sql.NullString
	var gp, gc, pp, pc, publico sql.NullInt64
	err := r.Scan(&p.id, &p.Data, &horario, &p.Temporada, &comp, &fase, &p.Adversario, &p.Mando,
		&gp, &gc, &pp, &pc, &resultado, &estadio, &cidade, &publico, &arbitro,
		&p.Fonte.Nome, &p.Fonte.URL, &p.Fonte.Licenca)
	if err != nil {
		return p, err
	}
	p.Horario, p.Competicao, p.Fase, p.Resultado = horario.String, comp.String, fase.String, resultado.String
	p.Estadio, p.Cidade, p.Arbitro, p.Publico = estadio.String, cidade.String, arbitro.String, inteiro(publico)
	if gp.Valid && gc.Valid {
		p.Placar = &Placar{Corinthians: int(gp.Int64), Adversario: int(gc.Int64),
			PenaltisCorinthians: inteiro(pp), PenaltisAdversario: inteiro(pc)}
	}
	return p, nil
}

type FiltroArtilharia struct {
	Categoria  string
	Temporada  int
	Competicao string
	Limite     int
}

// Artilharia soma os gols pelo Corinthians nas partidas do acervo. Gols contra
// não entram. Autores com artigo na Wikipédia são agrupados pelo artigo, o que
// junta grafias diferentes do mesmo nome.
func (a *Acervo) Artilharia(ctx context.Context, fa FiltroArtilharia) ([]Artilheiro, error) {
	if fa.Limite < 1 {
		fa.Limite = 20
	}
	if fa.Limite > LimiteMaximo {
		fa.Limite = LimiteMaximo
	}
	f := FiltroPartidas{Categoria: fa.Categoria, Temporada: fa.Temporada, Competicao: fa.Competicao}.filtro()
	f.add("g.lado = 'pro'")
	f.add("g.tipo <> 'contra'")
	f.add("g.autor IS NOT NULL")

	rows, err := a.conn().QueryContext(ctx, `SELECT
		COALESCE((SELECT pe.nome FROM pessoa pe WHERE pe.wiki_en = g.autor_wiki_en LIMIT 1), MIN(g.autor)),
		(SELECT pe.wikidata FROM pessoa pe WHERE pe.wiki_en = g.autor_wiki_en LIMIT 1),
		COUNT(*), SUM(g.tipo = 'penalti')
		FROM gol g JOIN partida p ON p.id = g.partida_id`+f.where()+`
		GROUP BY COALESCE(g.autor_wiki_en, g.autor)
		ORDER BY COUNT(*) DESC, MIN(g.autor) LIMIT ?`, append(f.args, fa.Limite)...)
	if err != nil {
		return nil, erroConsulta("artilharia", err)
	}
	defer rows.Close()
	out := []Artilheiro{}
	for rows.Next() {
		var x Artilheiro
		var wd sql.NullString
		if err := rows.Scan(&x.Nome, &wd, &x.Gols, &x.Penaltis); err != nil {
			return nil, erroConsulta("lendo artilheiro", err)
		}
		x.Wikidata = wd.String
		out = append(out, x)
	}
	return out, rows.Err()
}

// Temporadas lista as temporadas com partidas no acervo, com o resumo de cada uma.
func (a *Acervo) Temporadas(ctx context.Context, categoria string) ([]Temporada, error) {
	rows, err := a.conn().QueryContext(ctx, `SELECT p.temporada, f.nome, f.url, f.licenca, COUNT(*)
		FROM partida p JOIN fonte f ON f.id = p.fonte_id
		WHERE p.categoria = ? GROUP BY p.temporada, f.id ORDER BY p.temporada, COUNT(*) DESC, f.id`, categoria)
	if err != nil {
		return nil, erroConsulta("listando temporadas", err)
	}
	var out []Temporada
	for rows.Next() {
		var temporada, n int
		var f Fonte
		if err := rows.Scan(&temporada, &f.Nome, &f.URL, &f.Licenca, &n); err != nil {
			rows.Close()
			return nil, erroConsulta("lendo temporada", err)
		}
		if len(out) == 0 || out[len(out)-1].Temporada != temporada {
			out = append(out, Temporada{Temporada: temporada})
		}
		out[len(out)-1].Fontes = append(out[len(out)-1].Fontes, f) // a fonte com mais partidas vem primeiro
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, erroConsulta("listando temporadas", err)
	}
	for i := range out {
		_, r, err := a.resumo(ctx, FiltroPartidas{Categoria: categoria, Temporada: out[i].Temporada}.filtro())
		if err != nil {
			return nil, err
		}
		out[i].Resumo = r
	}
	if out == nil {
		out = []Temporada{}
	}
	return out, nil
}
