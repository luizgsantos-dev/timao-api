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
const VersaoEsquema = "6"

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
	Jogos         *int   // na temporada, da tabela de estatísticas
	Gols          *int
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

// Pessoa grava uma pessoa. O nome gravado é o de uso, tirado do título do artigo
// na Wikipédia em inglês ("Dida", e não "Nelson de Jesus Silva"), que é o mesmo
// que as páginas de temporada usam nos gols; o nome recebido, quando difere, fica
// como nome completo.
func (e *Escritor) Pessoa(ctx context.Context, p Pessoa, fonteID int64) (int64, error) {
	nome, completo := p.Nome, ""
	if uso := NomeDeUso(p.WikiEN); uso != "" && uso != p.Nome {
		nome, completo = uso, p.Nome
	}
	res, err := e.tx.ExecContext(ctx, `
		INSERT INTO pessoa (wikidata, nome, nome_completo, nome_busca, genero, nascimento, falecimento, nacionalidade,
		                    posicao, foto_url, wiki_pt, wiki_en, fonte_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nulo(p.Wikidata), nome, nulo(completo), Normalizar(strings.TrimSpace(nome+" "+completo)), nulo(p.Genero),
		nulo(p.Nascimento), nulo(p.Falecimento),
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
		INSERT INTO elenco (categoria, temporada, pessoa_id, nome, numero, posicao, nacionalidade, jogos, gols, fonte_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (categoria, temporada, nome) DO NOTHING`,
		el.Categoria, el.Temporada, pessoa, el.Nome, el.Numero, nulo(el.Posicao), nulo(el.Nacionalidade), el.Jogos, el.Gols, fonteID)
	if err != nil {
		return fmt.Errorf("banco: elenco %d %s: %w", el.Temporada, el.Nome, err)
	}
	return nil
}

// Partida grava a partida e seus gols. Se já existe uma partida da mesma categoria,
// data e adversário (a mesma página às vezes repete o jogo), devolve false e não grava.
// Partida grava uma partida e seus gols. O nome do adversário passa pela tabela
// de nomes (NomeDoAdversario) e, nas competições disputadas em sede única, o mando
// vira "neutro". Se já existe partida da categoria com o mesmo adversário no mesmo
// dia, devolve false e não grava.
func (e *Escritor) Partida(ctx context.Context, p Partida, fonteID int64) (bool, error) {
	p.Adversario = NomeDoAdversario(p.Adversario)
	if SedeUnica(p.Categoria, p.Competicao) {
		p.Mando = "neutro"
	}
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
	ID         int64
	Data       string
	Adversario string
	GolsPro    *int
	GolsContra *int
}

// PartidasPerto lista as partidas da categoria a até dias de distância de data,
// da mais próxima para a mais distante.
func (e *Escritor) PartidasPerto(ctx context.Context, categoria, data string, dias int) ([]Vizinha, error) {
	antes, depois := fmt.Sprintf("-%d days", dias), fmt.Sprintf("+%d days", dias)
	rows, err := e.tx.QueryContext(ctx, `SELECT id, data, adversario, gols_pro, gols_contra FROM partida
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
		if err := rows.Scan(&v.ID, &v.Data, &v.Adversario, &gp, &gc); err != nil {
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

// Enriquecer completa uma partida já gravada com os dados de outra fonte, sem
// sobrescrever nada: preenche horário, fase, estádio, cidade, público e árbitro
// vazios e, se a partida ainda não tem gols e o placar das duas fontes bate, grava
// os gols da outra fonte (só quando eles somam o placar). Devolve se mudou algo.
func (e *Escritor) Enriquecer(ctx context.Context, partidaID int64, p Partida) (bool, error) {
	res, err := e.tx.ExecContext(ctx, `UPDATE partida SET
		horario = COALESCE(horario, ?), fase = COALESCE(fase, ?), estadio = COALESCE(estadio, ?),
		cidade = COALESCE(cidade, ?), publico = COALESCE(publico, ?), arbitro = COALESCE(arbitro, ?)
		WHERE id = ? AND (
			(horario IS NULL AND ? IS NOT NULL) OR (fase IS NULL AND ? IS NOT NULL) OR
			(estadio IS NULL AND ? IS NOT NULL) OR (cidade IS NULL AND ? IS NOT NULL) OR
			(publico IS NULL AND ? IS NOT NULL) OR (arbitro IS NULL AND ? IS NOT NULL))`,
		nulo(p.Horario), nulo(p.Fase), nulo(p.Estadio), nulo(p.Cidade), p.Publico, nulo(p.Arbitro), partidaID,
		nulo(p.Horario), nulo(p.Fase), nulo(p.Estadio), nulo(p.Cidade), p.Publico, nulo(p.Arbitro))
	if err != nil {
		return false, fmt.Errorf("banco: enriquecendo partida %d: %w", partidaID, err)
	}
	n, _ := res.RowsAffected()
	mudou := n > 0

	if len(p.Gols) == 0 || p.GolsPro == nil || p.GolsContra == nil || len(p.Gols) != *p.GolsPro+*p.GolsContra {
		return mudou, nil
	}
	var temGols int
	var gp, gc sql.NullInt64
	if err := e.tx.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM gol WHERE partida_id = ?), gols_pro, gols_contra
		FROM partida WHERE id = ?`, partidaID, partidaID).Scan(&temGols, &gp, &gc); err != nil {
		return mudou, fmt.Errorf("banco: conferindo gols da partida %d: %w", partidaID, err)
	}
	if temGols > 0 || !gp.Valid || int(gp.Int64) != *p.GolsPro || int(gc.Int64) != *p.GolsContra {
		return mudou, nil
	}
	for _, g := range p.Gols {
		if _, err := e.tx.ExecContext(ctx, `INSERT INTO gol (partida_id, lado, autor, autor_wiki_en, minuto, tipo)
			VALUES (?, ?, ?, ?, ?, ?)`, partidaID, g.Lado, nulo(g.Autor), nulo(g.AutorWikiEN), nulo(g.Minuto), g.Tipo); err != nil {
			return mudou, fmt.Errorf("banco: gol da partida %d: %w", partidaID, err)
		}
	}
	return true, nil
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

// AjustarPassagensPelosGols recua o início das passagens de jogadores e jogadoras
// deduzidas das páginas da Wikipédia (elencos e categorias) até a primeira
// temporada em que a pessoa marcou pelo clube. Essas passagens nascem da primeira
// temporada com elenco publicado, e as páginas de competição trazem gols de anos
// sem elenco. Passagens do Wikidata, que têm datas próprias, não mudam. Devolve
// quantas passagens mudaram.
func (e *Escritor) AjustarPassagensPelosGols(ctx context.Context) (int64, error) {
	res, err := e.tx.ExecContext(ctx, `
		WITH primeiro_gol AS (
			SELECT pe.id AS pessoa_id, p.categoria, min(p.temporada) AS temporada
			FROM gol g JOIN partida p ON p.id = g.partida_id JOIN pessoa pe ON pe.wiki_en = g.autor_wiki_en
			WHERE g.lado = 'pro' AND g.tipo <> 'contra'
			GROUP BY pe.id, p.categoria
		)
		UPDATE passagem SET inicio = (
			SELECT CAST(pg.temporada AS TEXT) FROM primeiro_gol pg
			WHERE pg.pessoa_id = passagem.pessoa_id AND pg.categoria = passagem.categoria)
		WHERE funcao = 'jogador' AND length(inicio) = 4
		  AND fonte_id IN (SELECT id FROM fonte WHERE nome LIKE 'Wikipédia%')
		  AND (SELECT pg.temporada FROM primeiro_gol pg
		       WHERE pg.pessoa_id = passagem.pessoa_id AND pg.categoria = passagem.categoria) < CAST(inicio AS INTEGER)`)
	if err != nil {
		return 0, fmt.Errorf("banco: ajustando passagens pelos gols: %w", err)
	}
	return res.RowsAffected()
}

// NomeDeUso tira do título de um artigo da Wikipédia em inglês a desambiguação
// entre parênteses: "Dida (footballer, born 1973)" vira "Dida".
func NomeDeUso(tituloWikiEN string) string {
	if i := strings.Index(tituloWikiEN, " ("); i > 0 {
		tituloWikiEN = tituloWikiEN[:i]
	}
	return strings.TrimSpace(tituloWikiEN)
}

// Nomes de adversário que as fontes abreviam ou grafam de outro jeito -> o nome
// usado no resto do banco. Só abreviações, siglas de estado e erros de grafia:
// nomes históricos de verdade (Atlético Paranaense até 2018, Athletico depois;
// Bragantino antes de 2020) ficam como a fonte escreveu. Nomes iguais que podem
// ser de clubes diferentes ("Comercial", "América", "SC Internacional", o paulista
// de 1913) também ficam.
var nomesDeAdversario = map[string]string{
	"Atlético-PR":                 "Atlético Paranaense",
	"Atlético-MG":                 "Atlético Mineiro",
	"Atlético-GO":                 "Atlético Goianiense",
	"Atlético Goianense":          "Atlético Goianiense",
	"América-MG":                  "América Mineiro",
	"Sport":                       "Sport Recife",
	"Sport-PE":                    "Sport Recife",
	"Grêmio FBPA":                 "Grêmio",
	"Internacional-RS":            "Internacional",
	"Sao Paulo":                   "São Paulo",
	"Botafogo-RJ":                 "Botafogo",
	"Botafogo -SP":                "Botafogo-SP",
	"Botafogo (SP)":               "Botafogo-SP",
	"Fortaleza-CE":                "Fortaleza",
	"Goiás-GO":                    "Goiás",
	"Vila Nova-GO":                "Vila Nova",
	"São Caetano-SP":              "São Caetano",
	"ABC-RN":                      "ABC",
	"Juventus-SP":                 "Juventus",
	"Port. Santista":              "Portuguesa Santista",
	"Portuguesa Sa":               "Portuguesa Santista",
	"Mogi-Mirim":                  "Mogi Mirim",
	"Aracatuba":                   "Araçatuba",
	"Ferroviaria":                 "Ferroviária",
	"XV de Novembro (Jaú)":        "XV de Jaú",
	"XV de Novembro (Piracicaba)": "XV de Piracicaba",
	"Paulista (JUN)":              "Paulista",
	"Paulista (Jundiaí)":          "Paulista",
	"União S. João":               "União São João",
	"Avaí Kindermann":             "Avaí/Kindermann",
	"Kindermann/Avaí":             "Avaí/Kindermann",
}

// NomeDoAdversario devolve o nome do adversário como o banco o grava.
func NomeDoAdversario(nome string) string {
	if n, ok := nomesDeAdversario[strings.TrimSpace(nome)]; ok {
		return n
	}
	return strings.TrimSpace(nome)
}

// Competições disputadas inteiras numa sede, por categoria: nelas ninguém joga
// em casa, ainda que a fonte liste um mandante.
var competicoesEmSedeUnica = map[string]map[string]bool{
	Masculino: {"Mundial de Clubes da FIFA": true},
	Feminino:  {"Copa Libertadores": true, "Copa dos Campeões Feminina da FIFA": true},
}

// SedeUnica diz se a competição, na categoria, é disputada em sede única.
func SedeUnica(categoria, competicao string) bool {
	return competicoesEmSedeUnica[categoria][competicao]
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
