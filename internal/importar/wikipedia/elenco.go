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

// ExtrairElenco lê o elenco principal de uma página de temporada, nas predefinições
// {{Fs player}}. A seção muda de página para página ("==Squad==", "===Squad===" dentro
// de "Background", "===First team squad===" dentro de "Players"); vale a primeira
// seção com cara de elenco que tenha jogadores. Subseções de emprestados ficam de fora.
func ExtrairElenco(wikitexto, categoria string) []banco.Elenco {
	secao := secaoDoElenco(wikitexto)
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

var (
	reTituloElenco = regexp.MustCompile(`(?i)squad|first.team|players|elenco`)
	reNaoElenco    = regexp.MustCompile(`(?i)statistic|number|loan|out|transfer|former|kit`)
)

// secaoDoElenco devolve o texto da primeira seção, de qualquer nível, cujo título
// parece de elenco e que contém {{Fs player}}. A seção vai até o próximo título de
// nível igual ou maior.
func secaoDoElenco(texto string) string {
	titulos := reTitulo.FindAllStringSubmatchIndex(texto, -1)
	for i, t := range titulos {
		nome := limpar(texto[t[4]:t[5]])
		if !reTituloElenco.MatchString(nome) || reNaoElenco.MatchString(nome) {
			continue
		}
		nivel := t[3] - t[2]
		fim := len(texto)
		for _, prox := range titulos[i+1:] {
			if prox[3]-prox[2] <= nivel {
				fim = prox[0]
				break
			}
		}
		corpo := texto[t[1]:fim]
		if len(acharPredefinicoes(corpo, reJogadorElenco)) > 0 {
			return corpo
		}
	}
	return ""
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
