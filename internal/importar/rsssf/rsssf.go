// Package rsssf importa jogos do Corinthians (masculino) das páginas da RSSSF
// (Rec.Sport.Soccer Statistics Foundation) e da RSSSF Brasil: Campeonato
// Brasileiro, Copa do Brasil e Campeonato Paulista, ano a ano. As páginas são
// texto de largura fixa dentro de HTML, com formatos que mudam de época para
// época; o pacote entende os formatos mais comuns e devolve, para cada página,
// a URL e o autor, que precisam ser creditados.
//
// Jogos sem data não entram (o banco exige data): são só contados em SemData.
package rsssf

import (
	"context"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"timao-api/internal/banco"
	"timao-api/internal/importar/coleta"
)

const (
	BaseRSSSF  = "https://www.rsssf.org/"
	BaseBrasil = "https://www.rsssfbrasil.com/"
	// AutorPadrao é o crédito usado quando a página não diz quem a escreveu.
	AutorPadrao = "RSSSF"
)

// Pagina é uma página do RSSSF já lida.
type Pagina struct {
	URL, Autor, Competicao string
	Temporada              int
	Partidas               []banco.Partida
	SemData                int // jogos do Corinthians achados sem data (não entram)
	Avisos                 []string
}

type RSSSF struct {
	Coleta     *coleta.Cliente
	Pausa      time.Duration // pausa entre requisições, para não sobrecarregar o servidor
	BaseRSSSF  string
	BaseBrasil string

	requisitou bool
}

func Novo(c *coleta.Cliente) *RSSSF {
	return &RSSSF{Coleta: c, Pausa: time.Second, BaseRSSSF: BaseRSSSF, BaseBrasil: BaseBrasil}
}

// Brasileirao lê as páginas da Série A (rsssf.org/tablesb/brazYY.html até 2009,
// brazYYYY.html depois). Anos sem página são pulados.
func (r *RSSSF) Brasileirao(ctx context.Context, anos []int) ([]Pagina, error) {
	var out []Pagina
	for _, ano := range anos {
		curto, longo := fmt.Sprintf("tablesb/braz%02d.html", ano%100), fmt.Sprintf("tablesb/braz%d.html", ano)
		nomes := []string{curto, longo}
		if ano >= 2010 {
			nomes = []string{longo, curto}
		}
		var url, texto string
		for _, nome := range nomes {
			t, err := r.baixar(ctx, r.BaseRSSSF+nome)
			if err != nil {
				return nil, err
			}
			if t != "" {
				url, texto = r.BaseRSSSF+nome, t
				break
			}
		}
		if url == "" {
			continue
		}
		p := Pagina{URL: url, Competicao: "Campeonato Brasileiro", Temporada: ano}
		p.Partidas, p.SemData, p.Avisos = extrairBrasileirao(texto, ano)
		creditar(&p, texto)
		out = append(out, p)
	}
	return out, nil
}

// CopaDoBrasil lê as páginas rsssfbrasil.com/tablesae/cbrYYYY.htm.
func (r *RSSSF) CopaDoBrasil(ctx context.Context, anos []int) ([]Pagina, error) {
	return r.porAno(ctx, anos, "tablesae/cbr%d.htm", "Copa do Brasil", ExtrairCopaDoBrasil)
}

// Paulista lê as páginas rsssfbrasil.com/tablessz/spYYYY.htm.
func (r *RSSSF) Paulista(ctx context.Context, anos []int) ([]Pagina, error) {
	return r.porAno(ctx, anos, "tablessz/sp%d.htm", "Campeonato Paulista", ExtrairPaulista)
}

func (r *RSSSF) porAno(ctx context.Context, anos []int, modelo, competicao string,
	extrair func(string, int) ([]banco.Partida, int, []string)) ([]Pagina, error) {
	var out []Pagina
	for _, ano := range anos {
		url := r.BaseBrasil + fmt.Sprintf(modelo, ano)
		texto, err := r.baixar(ctx, url)
		if err != nil {
			return nil, err
		}
		if texto == "" {
			continue
		}
		p := Pagina{URL: url, Competicao: competicao, Temporada: ano}
		p.Partidas, p.SemData, p.Avisos = extrair(texto, ano)
		creditar(&p, texto)
		out = append(out, p)
	}
	return out, nil
}

// baixar busca a página e devolve o texto já sem HTML. Página inexistente (404)
// devolve "" sem erro.
func (r *RSSSF) baixar(ctx context.Context, url string) (string, error) {
	if r.requisitou && r.Pausa > 0 {
		select {
		case <-time.After(r.Pausa):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	r.requisitou = true
	body, err := r.Coleta.Get(ctx, url, "text/html")
	if err != nil {
		if errors.Is(err, coleta.ErrNaoEncontrado) {
			return "", nil
		}
		return "", fmt.Errorf("rsssf: %w", err)
	}
	return Texto(body), nil
}

func creditar(p *Pagina, texto string) {
	p.Autor = Autor(texto)
	if p.Autor == "" {
		p.Autor = AutorPadrao
		p.Avisos = append(p.Avisos, "autor não encontrado na página; creditado como "+AutorPadrao)
	}
	if p.SemData > 0 {
		p.Avisos = append(p.Avisos, fmt.Sprintf("%d jogo(s) do Corinthians sem data ficaram de fora", p.SemData))
	}
}

// ParaUTF8 devolve o conteúdo como texto UTF-8. Se não for UTF-8 válido, é
// tratado como Latin-1 (ISO-8859-1): cada byte vira a runa de mesmo valor.
func ParaUTF8(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	var sb strings.Builder
	sb.Grow(len(b) + len(b)/8)
	for _, c := range b {
		sb.WriteRune(rune(c))
	}
	return sb.String()
}

var (
	reBlocoInvisivel = regexp.MustCompile(`(?is)<script\b.*?</script\s*>|<style\b.*?</style\s*>|<!--.*?-->`)
	reTag            = regexp.MustCompile(`(?s)<(/?)([a-zA-Z][a-zA-Z0-9]*)\b[^>]*>`)
	tagsDeLinha      = map[string]bool{"br": true, "p": true, "div": true, "tr": true, "li": true, "table": true,
		"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true, "hr": true, "pre": true}
)

// Texto converte a página em texto: Latin-1 vira UTF-8, as tags saem e as
// entidades (&aacute;) são decodificadas. Dentro de <pre> as quebras de linha
// valem; fora dele, como no navegador, só <br>, <p> e afins quebram a linha.
func Texto(b []byte) string {
	s := strings.ReplaceAll(ParaUTF8(b), "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = reBlocoInvisivel.ReplaceAllString(s, "")

	var sb strings.Builder
	pre, pos := 0, 0
	trecho := func(t string) {
		if pre == 0 {
			t = strings.ReplaceAll(t, "\n", " ")
		}
		sb.WriteString(t)
	}
	for _, m := range reTag.FindAllStringSubmatchIndex(s, -1) {
		trecho(s[pos:m[0]])
		pos = m[1]
		fecha, nome := s[m[2]:m[3]] == "/", strings.ToLower(s[m[4]:m[5]])
		if nome == "pre" {
			if fecha {
				pre = max(pre-1, 0)
			} else {
				pre++
			}
		}
		if tagsDeLinha[nome] {
			sb.WriteByte('\n')
		}
	}
	trecho(s[pos:])
	return consertarTexto(strings.ReplaceAll(html.UnescapeString(sb.String()), " ", " "))
}

// Algumas páginas já vêm do servidor com acentos perdidos (o caractere de
// substituição U+FFFD no lugar da letra) ou convertidos de uma codificação do DOS
// ("Andr‚"). Estes são os casos achados nos nomes de clubes.
var consertos = strings.NewReplacer(
	"S�o ", "São ", "S�o\n", "São\n",
	"It�lia", "Itália",
	"Germ�nia", "Germânia",
	"Andr‚", "André",
)

func consertarTexto(s string) string { return consertos.Replace(s) }

var (
	reEspacos      = regexp.MustCompile(`\s+`)
	reAutorLinha   = regexp.MustCompile(`(?i)\bAut(?:hor|or)(?:s|es)?\s*:\s*(.+?)(?:\s*(?:Last updated|Última atualização|Atualizado|\(C\)|Copyright|You are free)|$)`)
	reAutorPrep    = regexp.MustCompile(`(?i)Prepared and maintained by\s+(.+?)\s+for\s+the\b`)
	reAutorCopy    = regexp.MustCompile(`(?i)\(C\)\s*Copyright\s+(.+?)(?:,?\s+RSSSF\b|\s+\d{4}|$)`)
	reEmailParen   = regexp.MustCompile(`\(\s*[^()]*@[^()]*\)`)
	reEmail        = regexp.MustCompile(`\S+@\S+`)
	reParenVazio   = regexp.MustCompile(`\(\s*\)`)
	reFimDoCredito = regexp.MustCompile(`[\s,;.(]+$`)
)

// Autor procura o crédito da página: "Author(s): ...", senão "Prepared and
// maintained by ... for the", senão "(C) Copyright ...". Devolve "" se não achar.
func Autor(texto string) string {
	t := reEspacos.ReplaceAllString(texto, " ")
	for _, re := range []*regexp.Regexp{reAutorLinha, reAutorPrep, reAutorCopy} {
		ms := re.FindAllStringSubmatch(t, -1)
		if len(ms) == 0 {
			continue
		}
		a := ms[len(ms)-1][1] // o crédito fica no fim da página
		a = reEmailParen.ReplaceAllString(a, "")
		a = reEmail.ReplaceAllString(a, "")
		a = reParenVazio.ReplaceAllString(a, "")
		a = reFimDoCredito.ReplaceAllString(reEspacos.ReplaceAllString(a, " "), "")
		a = strings.TrimSpace(a)
		if a != "" && len(a) < 200 {
			return a
		}
	}
	return ""
}
