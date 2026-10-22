package wikipedia

import (
	"regexp"
	"strings"
)

// Ferramentas mínimas para ler wikitexto: achar predefinições ({{...}}),
// separar parâmetros e transformar trechos em texto simples.

var (
	reComentario = regexp.MustCompile(`(?s)<!--.*?-->`)
	reRefVazia   = regexp.MustCompile(`(?is)<ref[^>]*/>`)
	reRef        = regexp.MustCompile(`(?is)<ref[^>]*>.*?</ref>`)
	reTag        = regexp.MustCompile(`(?s)<[^>]+>`)
	reArquivo    = regexp.MustCompile(`(?i)\[\[(?:file|image|arquivo|imagem):[^\[\]]*\]\]`)
	reLinkRotulo = regexp.MustCompile(`\[\[([^\[\]|]*)\|([^\[\]]*)\]\]`)
	reLink       = regexp.MustCompile(`\[\[([^\[\]|]*)\]\]`)
	reLinkExt    = regexp.MustCompile(`\[https?://[^\s\]]+\s*([^\]]*)\]`)
	rePredefInt  = regexp.MustCompile(`\{\{[^{}]*\}\}`)
	reEspacos    = regexp.MustCompile(`\s+`)
)

// semNotas remove comentários HTML e referências (<ref>), que só atrapalham a leitura.
func semNotas(s string) string {
	s = reComentario.ReplaceAllString(s, "")
	s = reRefVazia.ReplaceAllString(s, "")
	return reRef.ReplaceAllString(s, "")
}

// predefinicao é uma ocorrência de {{nome|...}} no texto.
type predefinicao struct {
	inicio, fim int // posições no texto, fim exclusivo
	corpo       string
}

// acharPredefinicoes devolve as predefinições de primeiro nível cujo nome casa com nome
// (sem diferenciar maiúsculas), respeitando chaves aninhadas.
func acharPredefinicoes(texto string, nome *regexp.Regexp) []predefinicao {
	var out []predefinicao
	for i := 0; i < len(texto)-1; i++ {
		if texto[i] != '{' || texto[i+1] != '{' {
			continue
		}
		fim := fecharChaves(texto, i)
		if fim < 0 {
			break
		}
		corpo := texto[i+2 : fim-2]
		partes := separarParametros(corpo)
		if len(partes) > 0 && nome.MatchString(strings.TrimSpace(partes[0])) {
			out = append(out, predefinicao{inicio: i, fim: fim, corpo: corpo})
			i = fim - 1
		}
	}
	return out
}

// fecharChaves devolve a posição logo após o "}}" que fecha o "{{" em inicio, ou -1.
func fecharChaves(texto string, inicio int) int {
	prof := 0
	for i := inicio; i < len(texto)-1; i++ {
		switch {
		case texto[i] == '{' && texto[i+1] == '{':
			prof++
			i++
		case texto[i] == '}' && texto[i+1] == '}':
			prof--
			i++
			if prof == 0 {
				return i + 1
			}
		}
	}
	return -1
}

// separarParametros divide o corpo de uma predefinição nos "|" de primeiro nível,
// ignorando os que estão dentro de outras predefinições ou de links.
func separarParametros(corpo string) []string {
	var partes []string
	chaves, colchetes, ini := 0, 0, 0
	for i := 0; i < len(corpo); i++ {
		switch {
		case strings.HasPrefix(corpo[i:], "{{"):
			chaves++
			i++
		case strings.HasPrefix(corpo[i:], "}}"):
			chaves--
			i++
		case strings.HasPrefix(corpo[i:], "[["):
			colchetes++
			i++
		case strings.HasPrefix(corpo[i:], "]]"):
			colchetes--
			i++
		case corpo[i] == '|' && chaves == 0 && colchetes == 0:
			partes = append(partes, corpo[ini:i])
			ini = i + 1
		}
	}
	return append(partes, corpo[ini:])
}

// parametrosNomeados monta o mapa nome -> valor dos parâmetros "nome = valor".
func parametrosNomeados(corpo string) map[string]string {
	out := map[string]string{}
	for _, p := range separarParametros(corpo)[1:] {
		nome, valor, ok := strings.Cut(p, "=")
		if !ok {
			continue
		}
		out[strings.ToLower(strings.TrimSpace(nome))] = strings.TrimSpace(valor)
	}
	return out
}

// predefinicoesComTexto viram o próprio primeiro parâmetro ao limpar o texto.
var predefinicoesComTexto = map[string]bool{
	"nowrap": true, "nobr": true, "small": true, "abbr": true, "utz": true,
	"center": true, "big": true,
}

// limpar transforma wikitexto em texto simples: links viram o texto exibido,
// predefinições conhecidas viram seu conteúdo e as demais somem.
func limpar(s string) string {
	s = semNotas(s)
	s = reArquivo.ReplaceAllString(s, "")
	s = reLinkRotulo.ReplaceAllString(s, "$2")
	s = reLink.ReplaceAllString(s, "$1")
	s = reLinkExt.ReplaceAllString(s, "$1")
	for {
		novo := rePredefInt.ReplaceAllStringFunc(s, expandir)
		if novo == s {
			break
		}
		s = novo
	}
	s = reTag.ReplaceAllString(s, " ")
	s = strings.NewReplacer("'''", "", "''", "", "&nbsp;", " ", "&ndash;", "–").Replace(s)
	return strings.TrimSpace(reEspacos.ReplaceAllString(s, " "))
}

func expandir(predef string) string {
	partes := separarParametros(predef[2 : len(predef)-2])
	nome := strings.ToLower(strings.TrimSpace(partes[0]))
	switch {
	case nome == "sortname" && len(partes) >= 3:
		return strings.TrimSpace(partes[1]) + " " + strings.TrimSpace(partes[2])
	case predefinicoesComTexto[nome] && len(partes) >= 2:
		return strings.TrimSpace(partes[1])
	case nome == "nbsp" || nome == "spaces":
		return " "
	case nome == "ndash" || nome == "–":
		return "–"
	case strings.HasPrefix(nome, "futebol "):
		// {{Futebol Corinthians|estado=depois}} -> Corinthians (Wikipédia em português)
		time := strings.TrimSpace(partes[0])[len("futebol "):]
		return strings.TrimSuffix(strings.TrimSuffix(time, " Feminino"), " feminino")
	case nome == "arbitro" || nome == "árbitro":
		// {{Arbitro|BR-PR|PR|[[Heber Roberto Lopes]]}} -> o último parâmetro
		return strings.TrimSpace(partes[len(partes)-1])
	}
	return ""
}
