package wikipedia

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"timao-api/internal/banco"
)

// Páginas dos clubes na Wikipédia em inglês, de onde vem a seção "Honours".
var PaginasDeTitulos = map[string]string{
	banco.Masculino: "SC Corinthians Paulista",
	banco.Feminino:  "SC Corinthians Paulista (women)",
}

var (
	reAnoTitulo    = regexp.MustCompile(`\b(18|19|20)\d{2}\b`)
	reCref         = regexp.MustCompile(`(?i)\{\{\s*cref2?\s*\|\s*([^|}]+)`)
	reNotaPredef   = regexp.MustCompile(`(?i)^cnote2?$`)
	reLinhaOutros  = regexp.MustCompile(`^(.+?)\s*\((\d+)\)\s*:\s*(.*)$`)
	reFimDeNome    = regexp.MustCompile(`\)\s*:`)
	reNaoAlfanum   = regexp.MustCompile(`[^a-z0-9]+`)
	reLinhaTabela  = regexp.MustCompile(`\n\|-[^\n]*`)
	reCelulaInline = regexp.MustCompile(`!!|\|\|`)
)

// Seções de "Honours" que não são do time principal.
var reSecaoIgnorada = regexp.MustCompile(`(?i)youth|runners?-?up|awards|individual|base`)

// Escopos das tabelas e listas, em inglês -> como a API expõe.
var escopos = map[string]string{
	"worldwide": "mundial", "world": "mundial",
	"continental":              "continental",
	"national":                 "nacional",
	"inter-state":              "interestadual",
	"interstate":               "interestadual",
	"national and inter-state": "nacional",
	"state":                    "estadual",
	"international":            "internacional",
	"friendly tournaments":     "amistoso",
}

// Competições conhecidas: nome na Wikipédia em inglês -> nome em português e slug.
// Masculino e feminino compartilham o slug porque a categoria já os separa.
var competicoes = map[string][2]string{
	"fifa club world cup":            {"Mundial de Clubes da FIFA", "mundial-de-clubes"},
	"copa libertadores":              {"Copa Libertadores", "libertadores"},
	"copa libertadores femenina":     {"Copa Libertadores", "libertadores"},
	"recopa sudamericana":            {"Recopa Sul-Americana", "recopa-sul-americana"},
	"copa sudamericana":              {"Copa Sul-Americana", "sul-americana"},
	"campeonato brasileiro série a":  {"Campeonato Brasileiro", "brasileirao"},
	"campeonato brasileiro série a1": {"Campeonato Brasileiro", "brasileirao"},
	"campeonato brasileiro série b":  {"Campeonato Brasileiro Série B", "brasileirao-serie-b"},
	"copa do brasil":                 {"Copa do Brasil", "copa-do-brasil"},
	"supercopa do brasil":            {"Supercopa do Brasil", "supercopa"},
	"torneio rio-são paulo":          {"Torneio Rio-São Paulo", "rio-sao-paulo"},
	"campeonato paulista":            {"Campeonato Paulista", "paulista"},
	"copa paulista":                  {"Copa Paulista", "copa-paulista"},
	"small club world cup":           {"Pequena Taça do Mundo", "pequena-taca-do-mundo"},
	"ramón de carranza trophy":       {"Troféu Ramón de Carranza", "ramon-de-carranza"},
}

// PaginaTitulos é o resultado da leitura da seção "Honours" de uma página.
type PaginaTitulos struct {
	Titulo    string
	URL       string
	Categoria string
	Titulos   []banco.Titulo
	Avisos    []Aviso
}

// Titulos lê os títulos das páginas dos dois times.
func (w *Wikipedia) Titulos(ctx context.Context) ([]PaginaTitulos, error) {
	var out []PaginaTitulos
	for _, cat := range []string{banco.Masculino, banco.Feminino} {
		titulo := PaginasDeTitulos[cat]
		texto, nome, rev, err := w.wikitexto(ctx, titulo)
		if err != nil {
			return nil, fmt.Errorf("wikipedia: %s: %w", titulo, err)
		}
		ts, avisos := ExtrairTitulos(texto, cat)
		if len(ts) == 0 {
			return nil, fmt.Errorf("wikipedia: %s: nenhum título encontrado na seção Honours", titulo)
		}
		out = append(out, PaginaTitulos{Titulo: nome, URL: urlRevisao(rev), Categoria: cat, Titulos: ts, Avisos: avisos})
	}
	return out, nil
}

// ExtrairTitulos lê a seção "Honours": a tabela de "Official tournaments" vira
// títulos oficiais, e as listas das demais subseções ("Others tournaments",
// "Friendly tournaments") viram títulos não oficiais. Base e vices ficam de fora.
func ExtrairTitulos(wikitexto, categoria string) ([]banco.Titulo, []Aviso) {
	secao, inicio := secaoNivel2(wikitexto, "honours")
	if secao == "" {
		return nil, []Aviso{{0, "seção Honours não encontrada"}}
	}
	notas := notasDeRodape(wikitexto)

	var out []banco.Titulo
	var avisos []Aviso
	titulos := reTitulo.FindAllStringSubmatchIndex(secao, -1)
	for i, t := range titulos {
		if t[3]-t[2] != 3 {
			continue
		}
		fim := len(secao)
		if i+1 < len(titulos) {
			// a subseção vai até o próximo título de nível 3 (os de nível 4 ficam dentro)
			for _, prox := range titulos[i+1:] {
				if prox[3]-prox[2] <= 3 {
					fim = prox[0]
					break
				}
			}
		}
		nome := strings.ToLower(limpar(secao[t[4]:t[5]]))
		corpo := secao[t[1]:fim]
		switch {
		case reSecaoIgnorada.MatchString(nome):
			continue
		case strings.Contains(nome, "official"):
			ts, as := lerTabelaTitulos(corpo, categoria, notas)
			out = append(out, ts...)
			for _, a := range as {
				avisos = append(avisos, Aviso{inicio + t[1] + a.Posicao, a.Motivo})
			}
		default:
			out = append(out, lerListaTitulos(corpo, nome, categoria)...)
		}
	}
	return out, avisos
}

// secaoNivel2 devolve o texto da seção de nível 2 cujo título contém nome, e onde ela começa.
func secaoNivel2(texto, nome string) (string, int) {
	titulos := reTitulo.FindAllStringSubmatchIndex(texto, -1)
	for i, t := range titulos {
		if t[3]-t[2] != 2 || !strings.Contains(strings.ToLower(texto[t[4]:t[5]]), nome) {
			continue
		}
		fim := len(texto)
		for _, prox := range titulos[i+1:] {
			if prox[3]-prox[2] == 2 {
				fim = prox[0]
				break
			}
		}
		return texto[t[1]:fim], t[1]
	}
	return "", 0
}

// notasDeRodape lê as notas {{Cnote2|rótulo|texto}} da página.
func notasDeRodape(texto string) map[string]string {
	out := map[string]string{}
	for _, p := range acharPredefinicoes(texto, reNotaPredef) {
		partes := separarParametros(p.corpo)
		if len(partes) >= 3 {
			out[strings.TrimSpace(partes[1])] = limpar(partes[2])
		}
	}
	return out
}

// lerTabelaTitulos lê uma wikitable de títulos: linhas de escopo ("Continental")
// e linhas de competição (competição, quantidade, temporadas).
func lerTabelaTitulos(corpo, categoria string, notas map[string]string) ([]banco.Titulo, []Aviso) {
	ini := strings.Index(corpo, "{|")
	if ini < 0 {
		return nil, []Aviso{{0, "tabela de títulos oficiais não encontrada"}}
	}
	fim := strings.Index(corpo[ini:], "\n|}")
	if fim < 0 {
		fim = len(corpo) - ini
	}
	tabela := corpo[ini : ini+fim]

	var out []banco.Titulo
	var avisos []Aviso
	escopo := "outros"
	locs := reLinhaTabela.FindAllStringIndex(tabela, -1)
	inicios := []int{0}
	for _, l := range locs {
		inicios = append(inicios, l[1])
	}
	for i, ini := range inicios {
		fimLinha := len(tabela)
		if i+1 < len(inicios) {
			fimLinha = locs[i][0]
		}
		linha := tabela[ini:fimLinha]
		celulas := celulasDaLinha(linha)
		switch {
		case len(celulas) == 1 && strings.Contains(linha, "colspan"):
			if e, ok := escopos[strings.ToLower(limpar(celulas[0]))]; ok {
				escopo = e
			}
		case len(celulas) == 3:
			comp := limpar(celulas[0])
			if strings.EqualFold(comp, "competitions") || comp == "" {
				continue
			}
			ts := edicoes(comp, celulas[2], escopo, true, categoria, notas)
			if q, err := strconv.Atoi(strings.Trim(limpar(celulas[1]), "' ")); err == nil && q != len(ts) {
				avisos = append(avisos, Aviso{ini, fmt.Sprintf("%s: a tabela diz %d títulos, mas lista %d anos", comp, q, len(ts))})
			}
			out = append(out, ts...)
		}
	}
	return out, avisos
}

// celulasDaLinha separa as células de uma linha de wikitable, sem os atributos.
func celulasDaLinha(linha string) []string {
	var celulas []string
	for _, l := range strings.Split(linha, "\n") {
		l = strings.TrimRight(l, " ")
		if l == "" {
			continue
		}
		if l[0] != '!' && l[0] != '|' {
			if len(celulas) > 0 {
				celulas[len(celulas)-1] += "\n" + l // continuação da célula anterior
			}
			continue
		}
		if strings.HasPrefix(l, "|}") || strings.HasPrefix(l, "{|") || strings.HasPrefix(l, "|+") {
			continue
		}
		for _, c := range reCelulaInline.Split(l[1:], -1) {
			celulas = append(celulas, semAtributos(c))
		}
	}
	return celulas
}

// semAtributos tira o 'style="..." |' do começo de uma célula.
func semAtributos(celula string) string {
	partes := separarParametros(celula)
	if len(partes) >= 2 && strings.Contains(partes[0], "=") && !strings.Contains(partes[0], "[[") {
		return strings.TrimSpace(strings.Join(partes[1:], "|"))
	}
	return strings.TrimSpace(celula)
}

// lerListaTitulos lê linhas como "*[[Torneio X]] (2): 1950, [[1953 Torneio X|1953]]".
// Os títulos de nível 4 da subseção ("International", "State") dão o escopo.
func lerListaTitulos(corpo, secao, categoria string) []banco.Titulo {
	escopo := escopos[secao]
	if escopo == "" {
		escopo = "outros"
	}
	var out []banco.Titulo
	for _, linha := range strings.Split(corpo, "\n") {
		linha = strings.TrimSpace(linha)
		if m := reTitulo.FindStringSubmatch(linha); m != nil {
			if e, ok := escopos[strings.ToLower(limpar(m[2]))]; ok {
				escopo = e
			}
			continue
		}
		if !strings.HasPrefix(linha, "*") {
			continue
		}
		linha = strings.TrimLeft(linha, "* ")
		m := reLinhaOutros.FindStringSubmatch(limpar(linha))
		loc := reFimDeNome.FindStringIndex(linha)
		if m == nil || loc == nil {
			continue
		}
		out = append(out, edicoes(m[1], linha[loc[1]:], escopo, false, categoria, nil)...)
	}
	return out
}

// edicoes transforma a célula de temporadas ("[[2012 Copa Libertadores|2012]], 2013")
// em um título por ano, com o artigo da edição e a nota de rodapé quando existem.
func edicoes(comp, temporadas, escopo string, oficial bool, categoria string, notas map[string]string) []banco.Titulo {
	nome, slug := nomeESlug(strings.TrimSpace(comp))
	var out []banco.Titulo
	for _, pedaco := range strings.Split(semNotas(temporadas), ",") {
		ano := reAnoTitulo.FindString(limpar(pedaco))
		if ano == "" {
			continue
		}
		n, _ := strconv.Atoi(ano)
		t := banco.Titulo{Categoria: categoria, Competicao: nome, Slug: slug, Escopo: escopo, Oficial: oficial, Ano: n}
		if l := reLinkRotulo.FindStringSubmatch(pedaco); l != nil {
			t.EdicaoWikiEN = strings.TrimSpace(l[1])
		}
		if c := reCref.FindStringSubmatch(pedaco); c != nil {
			t.Nota = notas[strings.TrimSpace(c[1])]
		}
		out = append(out, t)
	}
	return out
}

func nomeESlug(comp string) (string, string) {
	if c, ok := competicoes[strings.ToLower(comp)]; ok {
		return c[0], c[1]
	}
	return comp, strings.Trim(reNaoAlfanum.ReplaceAllString(banco.Normalizar(comp), "-"), "-")
}
