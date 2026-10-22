package wikipedia

import (
	"regexp"
	"strconv"
	"strings"

	"timao-api/internal/banco"
)

var (
	reJogadorElenco = regexp.MustCompile(`(?i)^fs2? player$`)
	reInfobox       = regexp.MustCompile(`(?i)^infobox football club season$`)
	reSubsecaoFora  = regexp.MustCompile(`(?i)loan|out|left|departed|reserve|youth|academy`)
)

// Posições do elenco (códigos da predefinição {{Fs player}}) em português.
var posicoes = map[string][2]string{ // masculino, feminino
	"GK": {"goleiro", "goleira"},
	"DF": {"defensor", "defensora"},
	"MF": {"meio-campista", "meio-campista"},
	"FW": {"atacante", "atacante"},
}

// Tecnico é quem comandou o time na temporada, segundo o quadro da página.
type Tecnico struct {
	Nome   string
	WikiEN string
}

// ExtrairElenco lê o elenco principal da seção "Squad" de uma página de temporada,
// nas predefinições {{Fs player}}. Subseções de emprestados ficam de fora.
func ExtrairElenco(wikitexto, categoria string) []banco.Elenco {
	secao, _ := secaoNivel2(wikitexto, "squad")
	if secao == "" {
		return nil
	}
	secao = semSubsecoes(secao, reSubsecaoFora)

	var out []banco.Elenco
	vistos := map[string]bool{}
	for _, p := range acharPredefinicoes(secao, reJogadorElenco) {
		campos := parametrosNomeados(p.corpo)
		nomeBruto := campos["name"]
		e := banco.Elenco{Categoria: categoria, Nome: limpar(nomeBruto), Nacionalidade: strings.ToUpper(limpar(campos["nat"]))}
		if e.Nome == "" || vistos[e.Nome] {
			continue
		}
		vistos[e.Nome] = true
		if m := reLinkRotulo.FindStringSubmatch(nomeBruto); m != nil {
			e.WikiEN = strings.TrimSpace(m[1])
		} else if m := reLink.FindStringSubmatch(nomeBruto); m != nil {
			e.WikiEN = strings.TrimSpace(m[1])
		}
		if n, err := strconv.Atoi(limpar(campos["no"])); err == nil {
			e.Numero = &n
		}
		if pos, ok := posicoes[strings.ToUpper(limpar(campos["pos"]))]; ok {
			e.Posicao = pos[0]
			if categoria == banco.Feminino {
				e.Posicao = pos[1]
			}
		}
		out = append(out, e)
	}
	return out
}

// ExtrairTecnicos lê o campo "manager" do quadro {{Infobox football club season}}.
// Quando houve troca na temporada, o campo traz mais de um nome.
func ExtrairTecnicos(wikitexto string) []Tecnico {
	caixas := acharPredefinicoes(wikitexto, reInfobox)
	if len(caixas) == 0 {
		return nil
	}
	valor := semNotas(parametrosNomeados(caixas[0].corpo)["manager"])
	var out []Tecnico
	for _, m := range reLinkRotulo.FindAllStringSubmatch(valor, -1) {
		out = append(out, Tecnico{Nome: limpar(m[2]), WikiEN: strings.TrimSpace(m[1])})
	}
	for _, m := range reLink.FindAllStringSubmatch(valor, -1) {
		out = append(out, Tecnico{Nome: limpar(m[1]), WikiEN: strings.TrimSpace(m[1])})
	}
	return out
}

// semSubsecoes remove do texto as subseções (nível 3 ou mais) cujo título casa com padrao.
func semSubsecoes(texto string, padrao *regexp.Regexp) string {
	titulos := reTitulo.FindAllStringSubmatchIndex(texto, -1)
	var b strings.Builder
	ultimo := 0
	pular := false
	for _, t := range titulos {
		if pular {
			ultimo = t[0]
		} else {
			b.WriteString(texto[ultimo:t[0]])
			ultimo = t[0]
		}
		pular = padrao.MatchString(texto[t[4]:t[5]])
	}
	if !pular {
		b.WriteString(texto[ultimo:])
	}
	return b.String()
}
