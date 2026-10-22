package wikipedia

import (
	"regexp"
	"strconv"
	"strings"

	"timao-api/internal/banco"
)

// Tabela "Squad statistics" (ou "Statistics") das páginas de temporada: uma linha
// por jogador com número, posição, nome e, para cada competição, jogos e gols, mais
// a coluna "Total". Existe em páginas que não têm {{Fs player}} (2008) e traz
// também quem saiu no meio da temporada.

var (
	reTituloEstatisticas = regexp.MustCompile(`(?i)statistic|appearances`)
	reBandeira           = regexp.MustCompile(`(?i)\{\{\s*flag ?icon\s*\|\s*([A-Za-z]{3})`)
	reJogos              = regexp.MustCompile(`^(-?\d+)(?:\s*\((\d+)\))?$`)
)

// ExtrairEstatisticas lê a tabela de estatísticas e devolve cada jogador com o
// total de jogos e gols na temporada (Jogos e Gols de banco.Elenco). Gols
// negativos, que algumas páginas usam para gols sofridos por goleiros, ficam nulos.
func ExtrairEstatisticas(wikitexto, categoria string) []banco.Elenco {
	tabela := tabelaDeEstatisticas(wikitexto)
	if tabela == "" {
		return nil
	}

	locs := reLinhaTabela.FindAllStringIndex(tabela, -1)
	inicios := []int{0}
	for _, l := range locs {
		inicios = append(inicios, l[1])
	}
	var grupos []string // nomes dos grupos de colunas (competições, Total, Discipline)
	var out []banco.Elenco
	vistos := map[string]bool{}
	for i, ini := range inicios {
		fim := len(tabela)
		if i+1 < len(inicios) {
			fim = locs[i][0]
		}
		linha := tabela[ini:fim]
		celulas := celulasDaLinha(linha)
		if grupos == nil && strings.Contains(linha, "colspan") {
			for _, c := range celulas[min(3, len(celulas)):] {
				grupos = append(grupos, strings.ToLower(limpar(c)))
			}
			continue
		}
		if len(celulas) < 4 || strings.HasPrefix(strings.TrimSpace(linha), "!") || grupos == nil {
			continue
		}
		e, ok := lerLinhaEstatistica(linha, celulas, grupos, categoria)
		if !ok || vistos[e.Nome] {
			continue
		}
		vistos[e.Nome] = true
		out = append(out, e)
	}
	return out
}

// tabelaDeEstatisticas acha a primeira wikitable de uma seção de estatísticas.
func tabelaDeEstatisticas(texto string) string {
	titulos := reTitulo.FindAllStringSubmatchIndex(texto, -1)
	for i, t := range titulos {
		if !reTituloEstatisticas.MatchString(texto[t[4]:t[5]]) {
			continue
		}
		fim := len(texto)
		if i+1 < len(titulos) {
			fim = titulos[i+1][0]
		}
		corpo := texto[t[1]:fim]
		ini := strings.Index(corpo, "{|")
		if ini < 0 {
			continue
		}
		f := strings.Index(corpo[ini:], "\n|}")
		if f < 0 {
			return corpo[ini:]
		}
		return corpo[ini : ini+f]
	}
	return ""
}

func lerLinhaEstatistica(linha string, celulas, grupos []string, categoria string) (banco.Elenco, bool) {
	nomeBruto := celulas[2]
	e := banco.Elenco{Categoria: categoria, Nome: limpar(reLinkRotulo.ReplaceAllString(nomeBruto, "$2"))}
	if l := reLinkRotulo.FindStringSubmatch(nomeBruto); l != nil {
		e.WikiEN, e.Nome = strings.TrimSpace(l[1]), limpar(l[2])
	} else if l := reLink.FindStringSubmatch(nomeBruto); l != nil {
		e.WikiEN, e.Nome = strings.TrimSpace(l[1]), limpar(l[1])
	}
	if e.Nome == "" {
		return e, false
	}
	if m := reBandeira.FindStringSubmatch(linha); m != nil {
		e.Nacionalidade = strings.ToUpper(m[1])
	}
	if n, err := strconv.Atoi(limpar(celulas[0])); err == nil {
		e.Numero = &n
	}
	if pos, ok := posicoes[strings.ToUpper(limpar(celulas[1]))]; ok {
		e.Posicao = pos[0]
		if categoria == banco.Feminino {
			e.Posicao = pos[1]
		}
	}

	numeros := celulas[3:]
	// Cada grupo ocupa duas colunas (jogos e gols). Usa o grupo "Total" se houver;
	// senão, soma as competições.
	total := -1
	for i, g := range grupos {
		if g == "total" {
			total = i
		}
	}
	jogos, gols, achou := 0, 0, false
	somaGols := true
	for i, g := range grupos {
		if (total >= 0 && i != total) || (total < 0 && (strings.Contains(g, "discipline") || strings.Contains(g, "card"))) {
			continue
		}
		if 2*i+1 >= len(numeros) {
			break
		}
		j, okJ := lerJogos(numeros[2*i])
		g2, okG := lerJogos(numeros[2*i+1])
		if !okJ {
			continue
		}
		achou = true
		jogos += j
		if !okG || g2 < 0 {
			somaGols = false
		} else {
			gols += g2
		}
	}
	if achou {
		e.Jogos = &jogos
		if somaGols {
			e.Gols = &gols
		}
	}
	return e, true
}

// lerJogos entende "12", "9 (2)" (titular e reserva) e "-17".
func lerJogos(s string) (int, bool) {
	m := reJogos.FindStringSubmatch(strings.TrimSpace(limpar(s)))
	if m == nil {
		return 0, false
	}
	n, _ := strconv.Atoi(m[1])
	if m[2] != "" {
		r, _ := strconv.Atoi(m[2])
		n += r
	}
	return n, true
}

// MesclarElenco junta o elenco das {{Fs player}} com as estatísticas: quem está
// nos dois ganha jogos e gols; quem só está nas estatísticas entra também.
func MesclarElenco(elenco, estatisticas []banco.Elenco) []banco.Elenco {
	chave := func(e banco.Elenco) string {
		if e.WikiEN != "" {
			return "w:" + e.WikiEN
		}
		return "n:" + banco.Normalizar(e.Nome)
	}
	idx := map[string]int{}
	out := append([]banco.Elenco(nil), elenco...)
	for i, e := range out {
		idx[chave(e)] = i
	}
	for _, s := range estatisticas {
		if i, ok := idx[chave(s)]; ok {
			out[i].Jogos, out[i].Gols = s.Jogos, s.Gols
			if out[i].Nacionalidade == "" {
				out[i].Nacionalidade = s.Nacionalidade
			}
			continue
		}
		idx[chave(s)] = len(out)
		out = append(out, s)
	}
	return out
}
