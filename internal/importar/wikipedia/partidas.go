package wikipedia

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"timao-api/internal/banco"
	"timao-api/internal/importar/nomes"
)

var (
	reCaixaJogo   = regexp.MustCompile(`(?i)^football ?box( collapsible)?$`)
	reTitulo      = regexp.MustCompile(`(?m)^(={2,6})\s*(.*?)\s*={2,6}\s*$`)
	reDataPredef  = regexp.MustCompile(`(?i)\{\{\s*(?:start date|dts|date)\s*\|(?:\s*df\s*=\s*\w+\s*\|)?\s*(\d{4})\s*\|\s*(\d{1,2})\s*\|\s*(\d{1,2})`)
	rePlacar      = regexp.MustCompile(`(\d+)\s*[–\-−—]\s*(\d+)`)
	reHorario     = regexp.MustCompile(`\b([01]?\d|2[0-3])[:.h]([0-5]\d)\b`)
	reDigitos     = regexp.MustCompile(`\d`)
	reMinuto      = regexp.MustCompile(`^(\d{1,3}(?:\+\d{1,2})?)'?$`)
	reMinutoTxt   = regexp.MustCompile(`(\d{1,3}(?:\+\d{1,2})?)'`)
	rePredefGol   = regexp.MustCompile(`(?i)\{\{\s*(goal|pengoal|pen goal|ogol|own goal|og)\s*((?:\|[^{}]*)?)\}\}`)
	reGolContra   = regexp.MustCompile(`(?i)\bo\.?\s?g\.?\b|own goal|gol contra`)
	reGolPenalti  = regexp.MustCompile(`(?i)^(pen\.?|p|penalty|pk)$`)
	reCorinthians = regexp.MustCompile(`(?i)corinthians`)
	reCompeticao  = regexp.MustCompile(`(?i)\b(copa|campeonato|supercopa|recopa|torneio|ta[cç]a|cup|league|championship|trophy|libertadores|sudamericana|brasileir|paulist|florida|series)\b`)
	reAmistoso    = regexp.MustCompile(`(?i)friendl|amistos|pre-?season`)
	reQuebra      = regexp.MustCompile(`(?i)<br\s*/?>`)
)

// Títulos de seção que não dizem a competição, só organizam a página.
var titulosGenericos = map[string]bool{
	"matches": true, "results": true, "competitions": true, "fixtures and results": true,
	"overview": true, "season": true, "fixtures": true, "match results": true,
}

// Aviso explica por que uma caixa de jogo foi ignorada.
type Aviso struct {
	Posicao int
	Motivo  string
}

func (a Aviso) String() string { return fmt.Sprintf("posição %d: %s", a.Posicao, a.Motivo) }

// ExtrairPartidas lê as caixas {{football box}} de uma página de temporada.
// A competição de cada jogo vem dos títulos de seção.
func ExtrairPartidas(wikitexto, categoria string, temporada int) ([]banco.Partida, []Aviso) {
	return extrair(wikitexto, categoria, temporada, "")
}

// ExtrairPartidasDaCompeticao lê as caixas de uma página de edição de competição,
// como "2019 Copa Libertadores Femenina": a competição é a da página, e os
// títulos de seção ("Group stage", "Final") dizem a fase.
func ExtrairPartidasDaCompeticao(wikitexto, categoria string, temporada int, competicao string) ([]banco.Partida, []Aviso) {
	return extrair(wikitexto, categoria, temporada, competicao)
}

func extrair(wikitexto, categoria string, temporada int, competicao string) ([]banco.Partida, []Aviso) {
	titulos := reTitulo.FindAllStringSubmatchIndex(wikitexto, -1)
	var partidas []banco.Partida
	var avisos []Aviso

	for _, caixa := range acharPredefinicoes(wikitexto, reCaixaJogo) {
		p, err := lerCaixa(caixa.corpo, temporada)
		if err != nil {
			avisos = append(avisos, Aviso{caixa.inicio, err.Error()})
			continue
		}
		p.Categoria = categoria
		p.Temporada = temporada
		if corrigida, ok := corrigirAno(p.Data, temporada); ok {
			avisos = append(avisos, Aviso{caixa.inicio, fmt.Sprintf("data %s fora da temporada; ano trocado para %d", p.Data, temporada)})
			p.Data = corrigida
		}
		h2, h3 := secao(wikitexto, titulos, caixa.inicio)
		if competicao == "" {
			p.Competicao, p.Fase = competicaoEFase(h2, h3, p.Fase)
		} else {
			p.Competicao, p.Fase = competicao, faseDaSecao(h2, h3, p.Fase)
		}
		p.Competicao, p.Fase = nomes.Competicao(p.Competicao), nomes.Fase(p.Fase)
		partidas = append(partidas, p)
	}
	return partidas, avisos
}

func lerCaixa(corpo string, temporada int) (banco.Partida, error) {
	campos := parametrosNomeados(corpo)
	var p banco.Partida

	time1, time2 := limpar(campos["team1"]), limpar(campos["team2"])
	c1, c2 := reCorinthians.MatchString(time1), reCorinthians.MatchString(time2)
	switch {
	case c1 && !c2:
		p.Mando, p.Adversario = "casa", time2
	case c2 && !c1:
		p.Mando, p.Adversario = "fora", time1
	default:
		return p, fmt.Errorf("não achei o Corinthians em só um dos lados (%q x %q)", time1, time2)
	}
	if p.Adversario == "" {
		return p, fmt.Errorf("adversário vazio")
	}

	d, err := lerData(campos["date"], temporada)
	if err != nil {
		return p, err
	}
	p.Data = d

	if m := reHorario.FindStringSubmatch(semNotas(campos["time"])); m != nil {
		h, _ := strconv.Atoi(m[1])
		p.Horario = fmt.Sprintf("%02d:%s", h, m[2])
	}

	if a, b, ok := placar(campos["score"]); ok {
		p.GolsPro, p.GolsContra = ordenar(p.Mando, a, b)
	}
	if a, b, ok := placar(campos["penaltyscore"]); ok {
		p.PenaltisPro, p.PenaltisContra = ordenar(p.Mando, a, b)
	}

	p.Fase = limpar(campos["round"])
	p.Estadio = limpar(campos["stadium"])
	p.Cidade = limpar(campos["location"])
	p.Arbitro = limpar(campos["referee"])
	if pub := limpar(campos["attendance"]); reDigitos.MatchString(pub) {
		digitos := strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, strings.Fields(pub)[0])
		if n, err := strconv.Atoi(digitos); err == nil {
			p.Publico = &n
		}
	}

	lado1, lado2 := "pro", "contra"
	if p.Mando == "fora" {
		lado1, lado2 = "contra", "pro"
	}
	p.Gols = append(lerGols(campos["goals1"], lado1), lerGols(campos["goals2"], lado2)...)
	conciliarGolsContra(&p)
	return p, nil
}

// conciliarGolsContra corrige uma convenção que varia entre editores: o gol contra
// às vezes é listado do lado de quem o marcou, e não do time beneficiado. Se trocar
// o lado dos gols contra faz os dois lados baterem com o placar, a troca é feita.
func conciliarGolsContra(p *banco.Partida) {
	if p.GolsPro == nil || p.GolsContra == nil {
		return
	}
	contar := func() (pro, contra int) {
		for _, g := range p.Gols {
			if g.Lado == "pro" {
				pro++
			} else {
				contra++
			}
		}
		return pro, contra
	}
	pro, contra := contar()
	if pro == *p.GolsPro && contra == *p.GolsContra {
		return
	}
	trocados := make([]banco.Gol, len(p.Gols))
	copy(trocados, p.Gols)
	for i := range trocados {
		if trocados[i].Tipo != "contra" {
			continue
		}
		if trocados[i].Lado == "pro" {
			trocados[i].Lado = "contra"
		} else {
			trocados[i].Lado = "pro"
		}
	}
	original := p.Gols
	p.Gols = trocados
	if pro, contra := contar(); pro != *p.GolsPro || contra != *p.GolsContra {
		p.Gols = original
	}
}

// corrigirAno trata um erro comum de digitação: numa página de temporada (ano
// civil), a data só pode ser do próprio ano, do seguinte (temporada que atravessa
// o ano) ou de dezembro do anterior (pré-temporada). Fora disso, o ano está
// errado e é trocado pelo da temporada.
func corrigirAno(data string, temporada int) (string, bool) {
	t, err := time.Parse("2006-01-02", data)
	if err != nil {
		return data, false
	}
	ano := t.Year()
	if ano == temporada || ano == temporada+1 || (ano == temporada-1 && t.Month() == time.December) {
		return data, false
	}
	return time.Date(temporada, t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02"), true
}

var formatosData = []string{"2 January 2006", "January 2, 2006", "2006-01-02", "2 Jan 2006", "Jan 2, 2006", "January 2 2006"}

// lerData entende a predefinição {{start date}} e datas por extenso em inglês.
// Quando a data vem sem ano ("22 April"), usa o ano da temporada.
func lerData(valor string, temporada int) (string, error) {
	if m := reDataPredef.FindStringSubmatch(valor); m != nil {
		a, _ := strconv.Atoi(m[1])
		mes, _ := strconv.Atoi(m[2])
		d, _ := strconv.Atoi(m[3])
		return fmt.Sprintf("%04d-%02d-%02d", a, mes, d), nil
	}
	texto := limpar(valor)
	if i := strings.IndexByte(texto, '('); i > 0 {
		texto = strings.TrimSpace(texto[:i])
	}
	for _, tentativa := range []string{texto, texto + " " + strconv.Itoa(temporada)} {
		for _, f := range formatosData {
			if t, err := time.Parse(f, tentativa); err == nil {
				return t.Format("2006-01-02"), nil
			}
		}
	}
	return "", fmt.Errorf("data %q não reconhecida", texto)
}

func placar(valor string) (int, int, bool) {
	for _, s := range []string{limpar(valor), semNotas(valor)} {
		if m := rePlacar.FindStringSubmatch(s); m != nil {
			a, _ := strconv.Atoi(m[1])
			b, _ := strconv.Atoi(m[2])
			return a, b, true
		}
	}
	return 0, 0, false
}

// ordenar converte o placar mandante x visitante em Corinthians x adversário.
func ordenar(mando string, a, b int) (*int, *int) {
	if mando == "fora" {
		a, b = b, a
	}
	return &a, &b
}

// lerGols lê a lista de autores de um lado, uma linha por jogador(a), com os minutos
// marcados pela predefinição {{goal}} (ou variantes) ou no formato "12'".
func lerGols(valor, lado string) []banco.Gol {
	valor = semNotas(valor)
	valor = reQuebra.ReplaceAllString(valor, "\n")
	var gols []banco.Gol
	for _, linha := range strings.Split(valor, "\n") {
		linha = strings.TrimSpace(strings.TrimLeft(linha, "*: "))
		if linha == "" {
			continue
		}
		autor, wiki := autorDaLinha(linha)
		contraNaLinha := reGolContra.MatchString(limpar(linha))

		achou := false
		for _, m := range rePredefGol.FindAllStringSubmatch(linha, -1) {
			achou = true
			nome := strings.ToLower(m[1])
			padrao := "normal"
			switch {
			case nome == "pengoal" || nome == "pen goal":
				padrao = "penalti"
			case nome == "ogol" || nome == "own goal" || nome == "og" || contraNaLinha:
				padrao = "contra"
			}
			for _, g := range minutosDaPredefinicao(m[2], padrao) {
				gols = append(gols, banco.Gol{Lado: lado, Autor: autor, AutorWikiEN: wiki, Minuto: g[0], Tipo: g[1]})
			}
		}
		if achou {
			continue
		}
		// Formato antigo, sem predefinição: "Fulano 12', 45+1' (pen.)".
		texto := limpar(linha)
		for _, m := range reMinutoTxt.FindAllStringSubmatch(texto, -1) {
			tipo := "normal"
			if contraNaLinha {
				tipo = "contra"
			}
			gols = append(gols, banco.Gol{Lado: lado, Autor: autor, AutorWikiEN: wiki, Minuto: m[1], Tipo: tipo})
		}
	}
	return gols
}

// minutosDaPredefinicao lê "|12|pen|45+2" em pares minuto/observação.
func minutosDaPredefinicao(params, padrao string) [][2]string {
	var out [][2]string
	for _, p := range strings.Split(strings.TrimPrefix(params, "|"), "|") {
		p = strings.TrimSpace(p)
		if p == "" || strings.Contains(p, "=") {
			continue
		}
		if m := reMinuto.FindStringSubmatch(p); m != nil {
			out = append(out, [2]string{m[1], padrao})
			continue
		}
		if len(out) == 0 {
			continue
		}
		switch ultimo := &out[len(out)-1]; {
		case reGolPenalti.MatchString(p):
			ultimo[1] = "penalti"
		case reGolContra.MatchString(p):
			ultimo[1] = "contra"
		}
	}
	return out
}

// autorDaLinha devolve o nome exibido e o título do artigo (quando há link) do autor.
func autorDaLinha(linha string) (nome, wiki string) {
	if m := reLinkRotulo.FindStringSubmatch(linha); m != nil && strings.Index(linha, m[0]) < indiceGol(linha) {
		return limpar(m[2]), strings.TrimSpace(m[1])
	}
	if m := reLink.FindStringSubmatch(linha); m != nil && strings.Index(linha, m[0]) < indiceGol(linha) {
		return limpar(m[1]), strings.TrimSpace(m[1])
	}
	antes := linha[:indiceGol(linha)]
	if i := strings.IndexAny(antes, "0123456789"); i > 0 {
		antes = antes[:i]
	}
	if nome := strings.Trim(limpar(antes), " ,"); nome != "" {
		return nome, ""
	}
	// Formato invertido: "{{goal|85}} [[Dentinho]]".
	depois := rePredefGol.ReplaceAllString(linha, "")
	if m := reLinkRotulo.FindStringSubmatch(depois); m != nil {
		return limpar(m[2]), strings.TrimSpace(m[1])
	}
	if m := reLink.FindStringSubmatch(depois); m != nil {
		return limpar(m[1]), strings.TrimSpace(m[1])
	}
	return strings.Trim(reGolContra.ReplaceAllString(limpar(depois), ""), " ,()"), ""
}

func indiceGol(linha string) int {
	if loc := rePredefGol.FindStringIndex(linha); loc != nil {
		return loc[0]
	}
	return len(linha)
}

// secao devolve os títulos de nível 2 e 3 em vigor na posição pos.
func secao(texto string, titulos [][]int, pos int) (h2, h3 string) {
	for _, t := range titulos {
		if t[0] > pos {
			break
		}
		nivel := t[3] - t[2]
		nome := limpar(texto[t[4]:t[5]])
		switch nivel {
		case 2:
			h2, h3 = nome, ""
		case 3:
			h3 = nome
		}
	}
	return h2, h3
}

// competicaoEFase decide a competição pelos títulos de seção. Em geral a competição
// é o título de nível 2 e a fase vem do campo round da caixa ou do título de nível 3.
func competicaoEFase(h2, h3, round string) (competicao, fase string) {
	g2, g3 := titulosGenericos[strings.ToLower(h2)], titulosGenericos[strings.ToLower(h3)]
	switch {
	case reAmistoso.MatchString(h2) || reAmistoso.MatchString(h3):
		competicao = "Amistoso"
	case g2 && h3 != "":
		competicao, h3 = h3, ""
	case h3 != "" && !g3 && reCompeticao.MatchString(h3) && !strings.EqualFold(h3, h2) && !ehFase(h3):
		competicao, h3 = h3, "" // ex.: "Copa Paulista" dentro de "Campeonato Paulista"
	default:
		competicao = h2
	}
	fase = round
	if fase == "" && !g3 {
		fase = h3
	}
	return competicao, fase
}

// faseDaSecao escolhe a fase numa página de competição: o campo round da caixa,
// ou o título de seção mais específico que não seja genérico.
func faseDaSecao(h2, h3, round string) string {
	for _, f := range []string{round, h3, h2} {
		if f != "" && !titulosGenericos[strings.ToLower(f)] {
			return f
		}
	}
	return ""
}

var reFase = regexp.MustCompile(`(?i)stage|round|group|final|phase|knockout|fase|quarter|semi|play-?off|table|summary`)

func ehFase(s string) bool { return reFase.MatchString(s) }

// OrdenarPartidas ordena por data, para gravar o banco em ordem estável.
func OrdenarPartidas(ps []banco.Partida) {
	sort.SliceStable(ps, func(i, j int) bool { return ps[i].Data < ps[j].Data })
}
