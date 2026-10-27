// Package acervo consulta o timao.db: pessoas, partidas, gols e títulos de cada
// categoria. Todas as consultas são somente leitura.
package acervo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"timao-api/internal/banco"
)

// ErrNaoEncontrado indica que o item pedido não existe na categoria.
var ErrNaoEncontrado = errors.New("acervo: não encontrado")

const (
	LimitePadrao = 50
	LimiteMaximo = 200
)

// Acervo é seguro para uso concorrente, e o banco pode ser trocado com a API no
// ar (veja Trocar).
type Acervo struct {
	atual atomic.Pointer[sql.DB]
}

func Novo(db *sql.DB) *Acervo {
	a := &Acervo{}
	a.atual.Store(db)
	return a
}

func (a *Acervo) conn() *sql.DB { return a.atual.Load() }

// Vazio devolve um acervo ainda sem banco: as consultas falham até Trocar.
func Vazio() *Acervo { return &Acervo{} }

// Disponivel diz se há um banco aberto.
func (a *Acervo) Disponivel() bool { return a != nil && a.conn() != nil }

// EsperaParaFechar é quanto o banco anterior fica aberto depois de uma troca,
// para que requisições em andamento terminem nele.
var EsperaParaFechar = time.Minute

// Trocar passa a usar o banco em path. O anterior é fechado depois de
// EsperaParaFechar, e as consultas que já começaram nele terminam normalmente.
func (a *Acervo) Trocar(path string) error {
	novo, err := banco.Abrir(path)
	if err != nil {
		return err
	}
	if antigo := a.atual.Swap(novo); antigo != nil {
		time.AfterFunc(EsperaParaFechar, func() { antigo.Close() })
	}
	return nil
}

// Abrir abre o arquivo do banco só para leitura.
func Abrir(path string) (*Acervo, error) {
	db, err := banco.Abrir(path)
	if err != nil {
		return nil, err
	}
	return Novo(db), nil
}

func (a *Acervo) Fechar() error {
	if db := a.conn(); db != nil {
		return db.Close()
	}
	return nil
}

// GeradoEm devolve quando o banco foi gerado (RFC 3339).
func (a *Acervo) GeradoEm(ctx context.Context) (string, error) {
	var v string
	err := a.conn().QueryRowContext(ctx, `SELECT valor FROM meta WHERE chave = 'gerado_em'`).Scan(&v)
	return v, err
}

type Fonte struct {
	Nome    string `json:"nome"`
	URL     string `json:"url"`
	Licenca string `json:"licenca"`
}

type Passagem struct {
	Funcao string `json:"funcao"`
	Inicio string `json:"inicio,omitempty"`
	Fim    string `json:"fim,omitempty"`
	Jogos  *int   `json:"jogos,omitempty"`
	Gols   *int   `json:"gols,omitempty"`
}

type Pessoa struct {
	Wikidata      string     `json:"wikidata"`
	Nome          string     `json:"nome"`                    // nome de uso
	NomeCompleto  string     `json:"nome_completo,omitempty"` // quando difere do nome de uso
	Genero        string     `json:"genero,omitempty"`
	Nascimento    string     `json:"nascimento,omitempty"`
	Falecimento   string     `json:"falecimento,omitempty"`
	Nacionalidade []string   `json:"nacionalidade,omitempty"`
	Posicao       []string   `json:"posicao,omitempty"`
	FotoURL       string     `json:"foto_url,omitempty"`
	WikipediaPT   string     `json:"wikipedia_pt,omitempty"`
	WikipediaEN   string     `json:"wikipedia_en,omitempty"`
	Passagens     []Passagem `json:"passagens"`
	// GolsEmPartidas conta os gols registrados nas partidas do acervo, amistosos e
	// temporadas sem elenco publicado incluídos; por isso pode passar da soma dos
	// elencos. Só vem no detalhe, e só quando a pessoa tem artigo na Wikipédia em
	// inglês para ligar.
	GolsEmPartidas *int             `json:"gols_em_partidas,omitempty"`
	Elencos        []ElencoDaPessoa `json:"elencos,omitempty"` // só no detalhe
	Fonte          Fonte            `json:"fonte"`

	id     int64
	wikiEN string
}

type Placar struct {
	Corinthians         int  `json:"corinthians"`
	Adversario          int  `json:"adversario"`
	PenaltisCorinthians *int `json:"penaltis_corinthians,omitempty"`
	PenaltisAdversario  *int `json:"penaltis_adversario,omitempty"`
}

type Partida struct {
	Data       string  `json:"data"`
	Horario    string  `json:"horario,omitempty"`
	Temporada  int     `json:"temporada"`
	Competicao string  `json:"competicao,omitempty"`
	Fase       string  `json:"fase,omitempty"`
	Adversario string  `json:"adversario"`
	Mando      string  `json:"mando"`
	Placar     *Placar `json:"placar,omitempty"` // nulo quando o jogo ainda não aconteceu
	Resultado  string  `json:"resultado,omitempty"`
	Estadio    string  `json:"estadio,omitempty"`
	Cidade     string  `json:"cidade,omitempty"`
	Publico    *int    `json:"publico,omitempty"`
	Arbitro    string  `json:"arbitro,omitempty"`
	Gols       []Gol   `json:"gols,omitempty"` // só no detalhe
	Fonte      Fonte   `json:"fonte"`

	id int64
}

type Gol struct {
	Minuto   string `json:"minuto,omitempty"`
	Autor    string `json:"autor,omitempty"`    // nome de uso da pessoa, quando ligada; senão, como a fonte escreveu
	Wikidata string `json:"wikidata,omitempty"` // da pessoa autora, quando ligada
	Tipo     string `json:"tipo"`               // normal, penalti ou contra
	Time     string `json:"time"`               // quem ganhou o gol: corinthians ou adversario
	Partida  *Jogo  `json:"partida,omitempty"`  // só na lista de gols de uma pessoa
}

// Jogo é o resumo de uma partida, usado dentro de outros itens.
type Jogo struct {
	Data       string `json:"data"`
	Adversario string `json:"adversario"`
	Competicao string `json:"competicao,omitempty"`
}

type Resumo struct {
	Jogos      int `json:"jogos"`
	Vitorias   int `json:"vitorias"`
	Empates    int `json:"empates"`
	Derrotas   int `json:"derrotas"`
	GolsPro    int `json:"gols_pro"`
	GolsContra int `json:"gols_contra"`
}

type Artilheiro struct {
	Nome     string `json:"nome"`
	Wikidata string `json:"wikidata,omitempty"`
	Gols     int    `json:"gols"`
	Penaltis int    `json:"penaltis"`
}

type Temporada struct {
	Temporada int     `json:"temporada"`
	Resumo    Resumo  `json:"resumo"`
	Fontes    []Fonte `json:"fontes"` // de onde vieram as partidas da temporada
}

// Paginacao é a parte comum das listas paginadas.
type Paginacao struct {
	Total  int `json:"total"`
	Pagina int `json:"pagina"`
	Limite int `json:"limite"`
}

func (p *Paginacao) normalizar() {
	if p.Pagina < 1 {
		p.Pagina = 1
	}
	if p.Limite < 1 {
		p.Limite = LimitePadrao
	}
	if p.Limite > LimiteMaximo {
		p.Limite = LimiteMaximo
	}
}

func (p Paginacao) offset() int { return (p.Pagina - 1) * p.Limite }

// filtro junta condições SQL e seus argumentos.
type filtro struct {
	conds []string
	args  []any
}

func (f *filtro) add(cond string, args ...any) {
	f.conds = append(f.conds, cond)
	f.args = append(f.args, args...)
}

func (f *filtro) where() string {
	if len(f.conds) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(f.conds, " AND ")
}

func contem(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(s) + "%"
}

func lista(s sql.NullString) []string {
	if !s.Valid || s.String == "" {
		return nil
	}
	return strings.Split(s.String, ", ")
}

func linkWiki(lang string, titulo sql.NullString) string {
	if !titulo.Valid {
		return ""
	}
	return "https://" + lang + ".wikipedia.org/wiki/" + url.PathEscape(strings.ReplaceAll(titulo.String, " ", "_"))
}

func inteiro(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func erroConsulta(op string, err error) error {
	return fmt.Errorf("acervo: %s: %w", op, err)
}

// Contagens conta as linhas das tabelas principais do banco em uso.
func (a *Acervo) Contagens(ctx context.Context) (map[string]int, error) {
	return banco.Contagens(ctx, a.conn())
}
