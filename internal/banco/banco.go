// Package banco cria e abre o timao.db, o arquivo SQLite com os dados importados
// de fontes abertas. O banco é gerado do zero a cada importação e lido pela API
// em modo somente leitura.
package banco

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite" // driver SQLite em Go puro, sem CGO
)

//go:embed schema.sql
var schema string

// VersaoEsquema muda sempre que schema.sql mudar de forma incompatível.
const VersaoEsquema = "4"

const (
	Masculino = "masculino"
	Feminino  = "feminino"
)

type Fonte struct {
	Nome       string
	URL        string
	Licenca    string
	ColetadoEm time.Time
}

type Pessoa struct {
	Wikidata      string
	Nome          string
	Genero        string
	Nascimento    string
	Falecimento   string
	Nacionalidade string
	Posicao       string
	FotoURL       string
	WikiPT        string
	WikiEN        string
}

type Passagem struct {
	Categoria string
	Funcao    string // "jogador" ou "tecnico"
	Inicio    string
	Fim       string
	Jogos     *int
	Gols      *int
}

type Titulo struct {
	Categoria    string
	Competicao   string
	Slug         string
	Escopo       string
	Oficial      bool
	Ano          int
	EdicaoWikiEN string
	Nota         string
}

type Partida struct {
	Categoria      string
	Temporada      int
	Data           string
	Horario        string
	Competicao     string
	Fase           string
	Adversario     string
	Mando          string
	GolsPro        *int
	GolsContra     *int
	PenaltisPro    *int
	PenaltisContra *int
	Estadio        string
	Cidade         string
	Publico        *int
	Arbitro        string
	Gols           []Gol
}

// Elenco é um(a) integrante do elenco de uma temporada.
type Elenco struct {
	Categoria     string
	Temporada     int
	Nome          string
	WikiEN        string // artigo na Wikipédia em inglês, quando há link
	Numero        *int
	Posicao       string
	Nacionalidade string // código FIFA, ex.: BRA
}

type Gol struct {
	Lado        string // "pro" ou "contra"
	Autor       string
	AutorWikiEN string
	Minuto      string
	Tipo        string // "normal", "penalti" ou "contra"
}

// Escritor monta um banco novo num arquivo temporário. Concluir troca o arquivo
// final de uma vez, então uma importação que falha não estraga o banco anterior.
type Escritor struct {
	db      *sql.DB
	tx      *sql.Tx
	destino string
	tmp     string
}

// Criar começa um banco novo que, ao concluir, substitui destino.
func Criar(ctx context.Context, destino string) (*Escritor, error) {
	tmp := destino + ".tmp"
	if err := os.Remove(tmp); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	db, err := sql.Open("sqlite", "file:"+tmp+"?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("banco: criando esquema: %w", err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		db.Close()
		return nil, err
	}
	return &Escritor{db: db, tx: tx, destino: destino, tmp: tmp}, nil
}

// Concluir grava os metadados, confirma a transação e põe o banco no lugar de destino.
func (e *Escritor) Concluir(ctx context.Context, geradoEm time.Time) error {
	for k, v := range map[string]string{
		"versao_esquema": VersaoEsquema,
		"gerado_em":      geradoEm.UTC().Format(time.RFC3339),
	} {
		if _, err := e.tx.ExecContext(ctx, `INSERT INTO meta (chave, valor) VALUES (?, ?)`, k, v); err != nil {
			return err
		}
	}
	if err := e.tx.Commit(); err != nil {
		return err
	}
	if _, err := e.db.ExecContext(ctx, `VACUUM`); err != nil {
		return err
	}
	if err := e.db.Close(); err != nil {
		return err
	}
	return os.Rename(e.tmp, e.destino)
}

// Descartar desfaz tudo e apaga o arquivo temporário.
func (e *Escritor) Descartar() {
	_ = e.tx.Rollback()
	_ = e.db.Close()
	_ = os.Remove(e.tmp)
}

func (e *Escritor) Fonte(ctx context.Context, f Fonte) (int64, error) {
	res, err := e.tx.ExecContext(ctx,
		`INSERT INTO fonte (nome, url, licenca, coletado_em) VALUES (?, ?, ?, ?)`,
		f.Nome, f.URL, f.Licenca, f.ColetadoEm.UTC().Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("banco: fonte %s: %w", f.URL, err)
	}
	return res.LastInsertId()
}

func (e *Escritor) Pessoa(ctx context.Context, p Pessoa, fonteID int64) (int64, error) {
	res, err := e.tx.ExecContext(ctx, `
		INSERT INTO pessoa (wikidata, nome, nome_busca, genero, nascimento, falecimento, nacionalidade,
		                    posicao, foto_url, wiki_pt, wiki_en, fonte_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nulo(p.Wikidata), p.Nome, Normalizar(p.Nome), nulo(p.Genero), nulo(p.Nascimento), nulo(p.Falecimento),
		nulo(p.Nacionalidade), nulo(p.Posicao), nulo(p.FotoURL), nulo(p.WikiPT), nulo(p.WikiEN), fonteID)
	if err != nil {
		return 0, fmt.Errorf("banco: pessoa %s: %w", p.Wikidata, err)
	}
	return res.LastInsertId()
}

func (e *Escritor) Passagem(ctx context.Context, pessoaID int64, p Passagem, fonteID int64) error {
	_, err := e.tx.ExecContext(ctx, `
		INSERT INTO passagem (pessoa_id, categoria, funcao, inicio, fim, jogos, gols, fonte_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		pessoaID, p.Categoria, p.Funcao, nulo(p.Inicio), nulo(p.Fim), p.Jogos, p.Gols, fonteID)
	if err != nil {
		return fmt.Errorf("banco: passagem: %w", err)
	}
	return nil
}

// Titulo grava uma conquista. Se a mesma competição já tem título no mesmo ano
// na categoria, devolve false e não grava.
func (e *Escritor) Titulo(ctx context.Context, t Titulo, fonteID int64) (bool, error) {
	oficial := 0
	if t.Oficial {
		oficial = 1
	}
	res, err := e.tx.ExecContext(ctx, `
		INSERT INTO titulo (categoria, competicao, slug, escopo, oficial, ano, edicao_wiki_en, nota, fonte_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (categoria, slug, ano) DO NOTHING`,
		t.Categoria, t.Competicao, t.Slug, t.Escopo, oficial, t.Ano, nulo(t.EdicaoWikiEN), nulo(t.Nota), fonteID)
	if err != nil {
		return false, fmt.Errorf("banco: título %s %d: %w", t.Slug, t.Ano, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// Elenco grava um(a) integrante do elenco de uma temporada. pessoaID é 0 quando
// a pessoa não está no banco. Nomes repetidos na mesma temporada são ignorados.
func (e *Escritor) Elenco(ctx context.Context, el Elenco, pessoaID, fonteID int64) error {
	pessoa := any(nil)
	if pessoaID > 0 {
		pessoa = pessoaID
	}
	_, err := e.tx.ExecContext(ctx, `
		INSERT INTO elenco (categoria, temporada, pessoa_id, nome, numero, posicao, nacionalidade, fonte_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (categoria, temporada, nome) DO NOTHING`,
		el.Categoria, el.Temporada, pessoa, el.Nome, el.Numero, nulo(el.Posicao), nulo(el.Nacionalidade), fonteID)
	if err != nil {
		return fmt.Errorf("banco: elenco %d %s: %w", el.Temporada, el.Nome, err)
	}
	return nil
}

// Partida grava a partida e seus gols. Se já existe uma partida da mesma categoria,
// data e adversário (a mesma página às vezes repete o jogo), devolve false e não grava.
func (e *Escritor) Partida(ctx context.Context, p Partida, fonteID int64) (bool, error) {
	res, err := e.tx.ExecContext(ctx, `
		INSERT INTO partida (categoria, temporada, data, horario, competicao, fase, adversario, adversario_busca, mando,
		                     gols_pro, gols_contra, penaltis_pro, penaltis_contra,
		                     estadio, cidade, publico, arbitro, fonte_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (categoria, data, adversario) DO NOTHING`,
		p.Categoria, p.Temporada, p.Data, nulo(p.Horario), nulo(p.Competicao), nulo(p.Fase), p.Adversario, Normalizar(p.Adversario), p.Mando,
		p.GolsPro, p.GolsContra, p.PenaltisPro, p.PenaltisContra,
		nulo(p.Estadio), nulo(p.Cidade), p.Publico, nulo(p.Arbitro), fonteID)
	if err != nil {
		return false, fmt.Errorf("banco: partida %s %s: %w", p.Data, p.Adversario, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return false, nil
	}
	id, err := res.LastInsertId()
	if err != nil {
		return false, err
	}
	for _, g := range p.Gols {
		if _, err := e.tx.ExecContext(ctx, `
			INSERT INTO gol (partida_id, lado, autor, autor_wiki_en, minuto, tipo)
			VALUES (?, ?, ?, ?, ?, ?)`,
			id, g.Lado, nulo(g.Autor), nulo(g.AutorWikiEN), nulo(g.Minuto), g.Tipo); err != nil {
			return false, fmt.Errorf("banco: gol em %s: %w", p.Data, err)
		}
	}
	return true, nil
}

// Vizinha é uma partida já gravada perto de uma data, usada para unir fontes.
type Vizinha struct {
	Data       string
	Adversario string
	GolsPro    *int
	GolsContra *int
}

// PartidasPerto lista as partidas da categoria a até dias de distância de data,
// da mais próxima para a mais distante.
func (e *Escritor) PartidasPerto(ctx context.Context, categoria, data string, dias int) ([]Vizinha, error) {
	antes, depois := fmt.Sprintf("-%d days", dias), fmt.Sprintf("+%d days", dias)
	rows, err := e.tx.QueryContext(ctx, `SELECT data, adversario, gols_pro, gols_contra FROM partida
		WHERE categoria = ? AND data BETWEEN date(?, ?) AND date(?, ?)
		ORDER BY abs(julianday(data) - julianday(?))`, categoria, data, antes, data, depois, data)
	if err != nil {
		return nil, fmt.Errorf("banco: procurando partidas perto de %s: %w", data, err)
	}
	defer rows.Close()
	var out []Vizinha
	for rows.Next() {
		var v Vizinha
		var gp, gc sql.NullInt64
		if err := rows.Scan(&v.Data, &v.Adversario, &gp, &gc); err != nil {
			return nil, err
		}
		if gp.Valid && gc.Valid {
			a, b := int(gp.Int64), int(gc.Int64)
			v.GolsPro, v.GolsContra = &a, &b
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// MesmoAdversario diz se dois nomes parecem do mesmo time, comparando as palavras
// que distinguem ("CA Mineiro" e "Atlético Mineiro", "Grêmio FBPA" e "Grêmio").
func MesmoAdversario(a, b string) bool {
	pa := palavrasDistintivas(a)
	for p := range palavrasDistintivas(b) {
		if pa[p] {
			return true
		}
	}
	return false
}

// Palavras comuns a muitos nomes de clube, que não servem para distinguir times.
var palavrasComuns = map[string]bool{
	"club": true, "clube": true, "esporte": true, "sport": true, "futebol": true, "football": true,
	"atletico": true, "athletico": true, "associacao": true, "sociedade": true, "recreativo": true,
	"esportiva": true, "deportivo": true, "atletica": true, "real": true, "uniao": true,
}

func palavrasDistintivas(nome string) map[string]bool {
	out := map[string]bool{}
	for _, p := range strings.FieldsFunc(Normalizar(nome), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	}) {
		if len(p) >= 4 && !palavrasComuns[p] {
			out[p] = true
		}
	}
	return out
}

// Abrir abre um timao.db existente só para leitura.
func Abrir(path string) (*sql.DB, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("banco: %w", err)
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=query_only(1)")
	if err != nil {
		return nil, err
	}
	var versao string
	if err := db.QueryRow(`SELECT valor FROM meta WHERE chave = 'versao_esquema'`).Scan(&versao); err != nil {
		db.Close()
		return nil, fmt.Errorf("banco: lendo versão do esquema: %w", err)
	}
	if versao != VersaoEsquema {
		db.Close()
		return nil, fmt.Errorf("banco: esquema na versão %s, esperava %s; rode a importação de novo", versao, VersaoEsquema)
	}
	return db, nil
}

// Normalizar deixa o texto em minúsculas e sem acentos, para buscas como
// "socrates" acharem "Sócrates".
func Normalizar(s string) string {
	return strings.Map(func(r rune) rune {
		if b, ok := semAcento[r]; ok {
			return b
		}
		return r
	}, strings.ToLower(s))
}

var semAcento = map[rune]rune{
	'á': 'a', 'à': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a', 'å': 'a',
	'é': 'e', 'è': 'e', 'ê': 'e', 'ë': 'e',
	'í': 'i', 'ì': 'i', 'î': 'i', 'ï': 'i',
	'ó': 'o', 'ò': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o',
	'ú': 'u', 'ù': 'u', 'û': 'u', 'ü': 'u',
	'ç': 'c', 'ñ': 'n', 'ý': 'y', 'ÿ': 'y',
}

// Contagens conta as linhas das tabelas principais, para comparar dois bancos.
func Contagens(ctx context.Context, db *sql.DB) (map[string]int, error) {
	out := map[string]int{}
	for _, consulta := range []struct{ nome, sql string }{
		{"pessoas", `SELECT COUNT(*) FROM pessoa`},
		{"passagens", `SELECT COUNT(*) FROM passagem`},
		{"titulos", `SELECT COUNT(*) FROM titulo`},
		{"partidas masculino", `SELECT COUNT(*) FROM partida WHERE categoria = 'masculino'`},
		{"partidas feminino", `SELECT COUNT(*) FROM partida WHERE categoria = 'feminino'`},
		{"gols", `SELECT COUNT(*) FROM gol`},
		{"elenco", `SELECT COUNT(*) FROM elenco`},
	} {
		var n int
		if err := db.QueryRowContext(ctx, consulta.sql).Scan(&n); err != nil {
			return nil, fmt.Errorf("banco: contando %s: %w", consulta.nome, err)
		}
		out[consulta.nome] = n
	}
	return out, nil
}

// GeradoEm lê quando o banco foi gerado.
func GeradoEm(ctx context.Context, db *sql.DB) (time.Time, error) {
	var v string
	if err := db.QueryRowContext(ctx, `SELECT valor FROM meta WHERE chave = 'gerado_em'`).Scan(&v); err != nil {
		return time.Time{}, fmt.Errorf("banco: lendo gerado_em: %w", err)
	}
	return time.Parse(time.RFC3339, v)
}

// nulo converte string vazia em NULL.
func nulo(s string) any {
	if s == "" {
		return nil
	}
	return s
}
