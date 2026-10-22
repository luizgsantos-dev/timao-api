package rsssf

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"timao-api/internal/banco"
	"timao-api/internal/importar/nomes"
)

// Formatos de linha entendidos (as três famílias de página usam o mesmo leitor,
// porque as épocas se misturam: o Paulista de 2003 tem o formato da Série A, a
// final da Copa de 1995 tem o formato de súmula etc.):
//
//	[Apr 24]                                     data dos jogos seguintes
//	Round 37 [Nov 28]                            fase (e data)
//	Corinthians    2-2  Juventude                jogo, com a data corrente
//	  [Tevez 9', Mattos 43'; Enilton 6', 78'p]   gols do mandante ; do visitante
//	23/04/1922 Palestra Itália 2 x 2 Corinthians jogo com data na linha
//	24/Jan Corinthians 0-1 Bragantino            idem, mês abreviado
//	14/06/1995 Corinthians 2-1 Grêmio            súmula, seguida de linhas
//	                   Viola     41'   (1-0)     de gol com o placar corrido
//	 9 Itumbiara (GO) - Corinthians (SP)   0-2 (04/03)   n/p (19/03)   [3-2pen]
//	                                       [Chicão 44pen, André Santos 49]
//	                                             confronto de ida e volta (Copa)
//	Operário(MT)1-1 0-4 Corinthians(SP)          ida e volta sem data (Copa antiga)
//
// O placar e os gols vêm sempre na ordem dos times na linha; o mando é do
// primeiro time (no confronto de ida e volta, a volta é na casa do segundo).
// Gols contra ("og") aparecem na lista do time beneficiado.

var meses = map[string]int{
	"jan": 1, "feb": 2, "fev": 2, "mar": 3, "apr": 4, "abr": 4, "may": 5, "mai": 5, "jun": 6,
	"jul": 7, "aug": 8, "ago": 8, "sep": 9, "set": 9, "oct": 10, "out": 10, "nov": 11, "dec": 12, "dez": 12,
}

const (
	reMes     = `([A-Za-z]{3})[a-z]*\.?`
	rePlacar  = `(\d+\s*-\s*\d+|n/p|w/?o|awd)`
	reNaoNum  = `awd|n/p|w/o|w\.o\.|wo|abd|canc\.?|susp\.?`
	reNomeSem = `(\p{L}[^\d\[\]]*?)` // nome de time: começa por letra, sem dígitos
)

var (
	reDataColchete     = regexp.MustCompile(`^\[\s*` + reMes + `\s+(\d{1,2})(?:,?\s+(\d{4}))?\s*\]\s*$`)
	reDuasDatas        = regexp.MustCompile(`^\[\s*` + reMes + `\s+(\d{1,2})\s*(?:,|and|&)\s*(?:` + reMes + `\s+)?(\d{1,2})\s*\]\s*$`)
	reCabecalhoComData = regexp.MustCompile(`^(.*?)\s*\[\s*` + reMes + `\s+(\d{1,2})\s*\]\s*$`)

	reConfronto = regexp.MustCompile(`^(?:\d{1,3}\s+)?(\p{L}[^\[\]]*?)\s+-\s+(\p{L}[^\[\]]*?)\s+` +
		rePlacar + `\s*\(([^)]*)\)(?:\s+` + rePlacar + `\s*\(([^)]*)\))?\s*(.*)$`)
	reDataNaLinha = regexp.MustCompile(`^(\d{1,2})[/.](\d{1,2}|[A-Za-z]{3})(?:[/.](\d{2,4}))?\s+(\p{L}.*?)\s+(\d+)\s*[-x]\s*(\d+)\s+` +
		`(\p{L}[^\[\d]*)(\[.*\]|\(.*\d.*\))?\s*$`)
	reIdaVolta = regexp.MustCompile(`^` + reNomeSem + `\s*(\d+-\d+|n/p|wo-\d+|\d+-wo|w/o)\s+(\d+-\d+|n/p|wo-\d+|\d+-wo|w/o)\s*` +
		reNomeSem + `\s*(\[.*)?$`)
	reJogo = regexp.MustCompile(`^` + reNomeSem + `\s+(\d+-\d+|` + reNaoNum + `)\s+` + reNomeSem + `\s*(\[.*)?$`)

	reNumeros    = regexp.MustCompile(`^(\d+)\s*-\s*(\d+)$`)
	reDiaMes     = regexp.MustCompile(`^(\d{1,2})/(\d{1,2})(?:/(\d{2,4}))?$`)
	rePenaltis   = regexp.MustCompile(`(?i)(?:(\p{L}[\p{L} .'/-]*?)\s+)?(\d+)\s*-\s*(\d+)\s*(?:on\s+)?(?:pen|pens|pk|penalties|pênaltis)\b`)
	reJogouEm    = regexp.MustCompile(`(?i)played in\s+([^\]\[;,]+)`)
	reAnulado    = regexp.MustCompile(`(?i)annulled|anulad`)
	reArbitro    = regexp.MustCompile(`^(?i:referee|árbitro|arbitro)\s*:\s*(.+?)\s*$`)
	rePublico    = regexp.MustCompile(`^(?i:att|attendance|público|publico)\.?\s*:\s*(\d[\d .]*)`)
	rePlacarFim  = regexp.MustCompile(`\s*\(\s*(\d+)\s*-\s*(\d+)\s*\)\s*$`)
	reSepGols    = regexp.MustCompile(`\s*,\s*|\s*;\s*|\s+e\s+|\s+and\s+|\s*&\s*`)
	reMarcaGol   = regexp.MustCompile(`(?i)\(\s*(og|o\.g\.|gc|contra|pen\.?|p|pk|pênalti)\s*\)`)
	reGol        = regexp.MustCompile(`(?i)^(.*?)\s*(\d{1,3})\s*(?:min\.?)?\s*'?\s*(?:\+\s*(\d{1,2})\s*'?)?\s*(og|pen|pk|p)?\s*'?\s*$`)
	reRodada     = regexp.MustCompile(`(?i)^(?:round|rodada|matchday)\s+(\d{1,2})$`)
	reGrupo      = regexp.MustCompile(`(?i)^(?:group|grupo)\s+(\w{1,3})$`)
	reRemarcados = regexp.MustCompile(`(?i)^replayed match(?:es)?$`)
	reParteFase  = regexp.MustCompile(`\s*\([^()]*\)|\s*\[[^\[\]]*\]`)
	reEstagio    = regexp.MustCompile(`(?i)^(first|second|third|fourth|fifth) stage$`)
)

var ordinaisEtapa = map[string]string{"first": "Primeira", "second": "Segunda", "third": "Terceira",
	"fourth": "Quarta", "fifth": "Quinta"}

// Palavras que podem formar uma linha de fase ("Second Stage - Final",
// "Quarter-Finals", "Group 3", "Primeira Fase").
var palavrasDeFase = map[string]bool{
	"first": true, "second": true, "third": true, "fourth": true, "fifth": true, "sixth": true,
	"final": true, "finals": true, "semifinal": true, "semifinals": true, "semi": true,
	"quarterfinal": true, "quarterfinals": true, "quarter": true, "round": true, "rounds": true,
	"stage": true, "phase": true, "group": true, "groups": true, "replayed": true, "match": true,
	"matches": true, "play": true, "off": true, "offs": true, "playoff": true, "playoffs": true,
	"preliminary": true, "qualifying": true, "knockout": true, "decider": true, "extra": true,
	"tournament": true, "relegation": true, "of": true, "and": true, "the": true, "leg": true, "level": true,
	"primeira": true, "segunda": true, "terceira": true, "quarta": true, "fase": true, "turno": true,
	"returno": true, "grupo": true, "rodada": true, "semifinais": true, "quartas": true,
	"oitavas": true, "de": true, "finais": true, "quadrangular": true, "hexagonal": true,
	"triangular": true, "octogonal": true, "taça": true, "torneio": true, "decisão": true,
}

var palavrasFracas = map[string]bool{"of": true, "and": true, "the": true, "de": true, "match": true,
	"matches": true, "leg": true, "level": true, "play": true, "off": true, "offs": true}

var fasesExtras = map[string]string{
	"preliminary round": "Fase preliminar", "preliminary phase": "Fase preliminar",
	"replayed matches": "Jogo remarcado", "replayed match": "Jogo remarcado",
	"play-off": "Playoff", "play-offs": "Playoffs", "playoff": "Playoff", "playoffs": "Playoffs",
}

// Títulos de outras divisões nas páginas que juntam tudo (braz2010.html tem
// Série A, Copa do Brasil, Série B...): o Brasileirão para no primeiro deles.
var outrasSecoes = map[string]bool{"copa do brasil": true, "série b": true, "serie b": true,
	"série c": true, "série d": true, "brazil cup": true}

// reDivisaoInferior reconhece o título das divisões de baixo nas páginas do
// Paulista ("Second Level", "Série A2", "Segunda Divisão"...). Nelas joga outro
// "Corinthians" (o de Presidente Prudente, entre outros), então a leitura para ali.
var reDivisaoInferior = regexp.MustCompile(`(?i)^((second|third|fourth|fifth|sixth|lower|intermediate) (level|division)|(segunda|terceira|quarta|quinta) divis[aã]o|s[ée]rie a-?[23]|divis[aã]o intermedi[aá]ria|divis[aã]o de acesso)`)

type perna struct {
	idx      int // posição em partidas; -1 quando o jogo não entrou (sem data)
	time1Pro bool
	g1, g2   int // placar na ordem da linha
}

type pendente struct {
	pernas  []perna // jogos que ainda esperam a linha de gols, em ordem
	corrido bool    // súmula: gols com placar corrido
	g1, g2  int
	idx     int // jogo da súmula
}

type extrator struct {
	ano, anoData, ultimoMes int
	competicao              string
	data                    string
	datas                   []string // de "[Mar 16, 22]", para o próximo confronto
	etapa, grupo, rodada    string
	partidas                []banco.Partida
	semData                 int
	avisos                  []string
	pend                    *pendente
	colchete                string // linha de gols ainda sem o "]"
	viuJogo                 bool
	pararEmOutraSecao       bool
	pararEmDivisaoInferior  bool
}

func novoExtrator(ano int, competicao string) *extrator {
	return &extrator{ano: ano, anoData: ano, competicao: competicao}
}

// ExtrairBrasileirao lê o texto de uma página da Série A (já sem HTML).
func ExtrairBrasileirao(texto string, ano int) ([]banco.Partida, []string) {
	ps, sem, avisos := extrairBrasileirao(texto, ano)
	if sem > 0 {
		avisos = append(avisos, fmt.Sprintf("%d jogo(s) do Corinthians sem data ficaram de fora", sem))
	}
	return ps, avisos
}

func extrairBrasileirao(texto string, ano int) ([]banco.Partida, int, []string) {
	e := novoExtrator(ano, "Campeonato Brasileiro")
	e.pararEmOutraSecao = true
	return e.ler(texto)
}

// ExtrairCopaDoBrasil lê o texto de uma página da Copa do Brasil (já sem HTML).
// O int devolvido conta os jogos do Corinthians achados sem data.
func ExtrairCopaDoBrasil(texto string, ano int) ([]banco.Partida, int, []string) {
	return novoExtrator(ano, "Copa do Brasil").ler(texto)
}

// ExtrairPaulista lê o texto de uma página do Campeonato Paulista (já sem HTML).
// A leitura para na primeira divisão inferior da página.
func ExtrairPaulista(texto string, ano int) ([]banco.Partida, int, []string) {
	e := novoExtrator(ano, "Campeonato Paulista")
	e.pararEmDivisaoInferior = true
	return e.ler(texto)
}

func (e *extrator) ler(texto string) ([]banco.Partida, int, []string) {
	for _, l := range strings.Split(strings.ReplaceAll(texto, "\r", ""), "\n") {
		if !e.linha(l) {
			break
		}
	}
	return e.partidas, e.semData, e.avisos
}

// linha trata uma linha; devolve false para encerrar a leitura.
func (e *extrator) linha(bruta string) bool {
	bruta = strings.ReplaceAll(bruta, "\t", "    ")
	t := strings.TrimSpace(bruta)
	if t == "" {
		return true
	}
	recuada := bruta[0] == ' '

	if e.colchete != "" { // linha de gols quebrada em duas
		e.colchete += " " + t
		if strings.Contains(t, "]") {
			s := e.colchete
			e.colchete = ""
			e.golsEntreColchetes(s)
		}
		return true
	}
	if e.pend != nil {
		if recuada && strings.HasPrefix(t, "[") {
			if !strings.Contains(t, "]") {
				e.colchete = t
				return true
			}
			e.golsEntreColchetes(t)
			return true
		}
		if e.pend.corrido && e.sumula(t, recuada) {
			return true
		}
	}

	if e.pararEmOutraSecao && e.viuJogo && outrasSecoes[strings.ToLower(t)] {
		return false
	}
	if e.pararEmDivisaoInferior && len(t) < 60 && reDivisaoInferior.MatchString(t) {
		return false
	}
	if recuada && strings.HasPrefix(t, "[") {
		return true // gols de jogo que não é do Corinthians
	}

	if m := reDuasDatas.FindStringSubmatch(t); m != nil {
		mes2 := m[3]
		if mes2 == "" {
			mes2 = m[1]
		}
		e.datas = []string{e.dataMes(m[1], m[2], ""), e.dataMes(mes2, m[4], "")}
		e.pend = nil
		return true
	}
	if m := reDataColchete.FindStringSubmatch(t); m != nil {
		e.data = e.dataMes(m[1], m[2], m[3])
		e.pend = nil
		return true
	}

	if e.confronto(t) || e.jogoComData(t) || e.idaEVolta(t) || e.jogo(t) {
		e.viuJogo = true
		return true
	}

	if !recuada {
		if m := reCabecalhoComData.FindStringSubmatch(t); m != nil && ehCabecalho(m[1]) {
			e.cabecalho(m[1])
			e.data = e.dataMes(m[2], m[3], "")
			return true
		}
		if ehCabecalho(t) {
			e.cabecalho(t)
			return true
		}
	}
	e.pend = nil
	return true
}

// ehCabecalho reconhece linhas de fase: todas as palavras precisam ser de fase
// (ou números e letras soltas), e ao menos uma tem que ser forte.
func ehCabecalho(t string) bool {
	t = strings.TrimSpace(reParteFase.ReplaceAllString(t, ""))
	if t == "" || len(t) > 60 {
		return false
	}
	palavras := strings.FieldsFunc(strings.ToLower(t), func(r rune) bool {
		return unicode.IsSpace(r) || r == '-' || r == '/' || r == ':'
	})
	forte := false
	for _, p := range palavras {
		switch {
		case palavrasDeFase[p]:
			forte = forte || !palavrasFracas[p]
		case isNumero(p), len([]rune(p)) == 1, p == "1st" || p == "2nd" || p == "3rd" || p == "4th":
		default:
			return false
		}
	}
	return forte && len(palavras) <= 8
}

func isNumero(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

func (e *extrator) cabecalho(t string) {
	t = strings.TrimSpace(reParteFase.ReplaceAllString(t, ""))
	e.pend, e.data, e.datas = nil, "", nil
	switch {
	case reRodada.MatchString(t):
		e.rodada = "Rodada " + reRodada.FindStringSubmatch(t)[1]
	case reGrupo.MatchString(t):
		e.grupo, e.rodada = "Grupo "+strings.ToUpper(reGrupo.FindStringSubmatch(t)[1]), ""
	case reRemarcados.MatchString(t):
		e.rodada = "Jogo remarcado"
	default:
		e.etapa, e.grupo, e.rodada = traduzirFase(t), "", ""
	}
}

// traduzirFase traduz "Second Stage - Final" parte por parte.
func traduzirFase(t string) string {
	var partes []string
	pedacos := strings.Split(t, " - ")
	for _, p := range pedacos {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// "First Stage - First Phase": o estágio vira etapa, para não repetir "fase"
		if m := reEstagio.FindStringSubmatch(p); m != nil && len(pedacos) > 1 {
			partes = append(partes, ordinaisEtapa[strings.ToLower(m[1])]+" etapa")
			continue
		}
		if f, ok := fasesExtras[strings.ToLower(p)]; ok {
			partes = append(partes, f)
			continue
		}
		f := nomes.Fase(p)
		if f == p && strings.ToUpper(p) == p { // "FINAL"
			f = nomes.Fase(strings.ToLower(p))
		}
		if f != "" {
			partes = append(partes, f)
		}
	}
	return strings.Join(partes, " - ")
}

func (e *extrator) fase() string {
	var partes []string
	for _, p := range []string{e.etapa, e.grupo, e.rodada} {
		if p != "" {
			partes = append(partes, p)
		}
	}
	return strings.Join(partes, " - ")
}

// dataMes monta a data a partir de "Apr"/"04", dia e ano opcional. Sem ano, usa
// o da temporada; se as datas voltam de outubro para o começo do ano, a
// temporada virou o ano.
func (e *extrator) dataMes(mes, dia, ano string) string {
	m, ok := meses[strings.ToLower(mes)]
	if !ok {
		n, err := strconv.Atoi(mes)
		if err != nil {
			return ""
		}
		m = n
	}
	d, err := strconv.Atoi(dia)
	if err != nil {
		return ""
	}
	a := 0
	if ano != "" {
		a, _ = strconv.Atoi(ano)
		if a < 100 {
			a += 1900
			if a < e.ano-50 {
				a += 100
			}
		}
	} else {
		if e.ultimoMes >= 10 && m <= 3 {
			e.anoData++
		}
		e.ultimoMes = m
		a = e.anoData
	}
	if m < 1 || m > 12 || d < 1 {
		return ""
	}
	dt := time.Date(a, time.Month(m), d, 12, 0, 0, 0, time.UTC)
	if dt.Day() != d {
		return ""
	}
	return dt.Format("2006-01-02")
}

func (e *extrator) aviso(format string, args ...any) {
	e.avisos = append(e.avisos, fmt.Sprintf(format, args...))
}

// novoJogo grava o jogo do Corinthians (se tiver data) e devolve o índice, ou
// -1 se não gravou. ok=false quando o Corinthians não está no jogo.
func (e *extrator) novoJogo(data, time1, time2 string, g1, g2 int, nota string, mandoTime1 bool) (idx int, time1Pro, ok bool) {
	c1, c2 := ehCorinthians(time1), ehCorinthians(time2)
	if c1 == c2 {
		return -1, false, false
	}
	adv := nomeAdversario(time2)
	if c2 {
		adv = nomeAdversario(time1)
	}
	if reAnulado.MatchString(nota) {
		e.aviso("jogo anulado contra %s (%s) ficou de fora", adv, data)
		return -1, c1, true
	}
	if data == "" {
		e.semData++
		return -1, c1, true
	}
	pro, contra := g1, g2
	if c2 {
		pro, contra = g2, g1
	}
	p := banco.Partida{
		Categoria:  banco.Masculino,
		Temporada:  e.ano,
		Data:       data,
		Competicao: e.competicao,
		Fase:       e.fase(),
		Adversario: adv,
		Mando:      "fora",
		GolsPro:    &pro,
		GolsContra: &contra,
	}
	if c1 == mandoTime1 {
		p.Mando = "casa"
	}
	if m := reJogouEm.FindStringSubmatch(nota); m != nil {
		p.Cidade = strings.TrimSpace(m[1])
	}
	if m := rePenaltis.FindStringSubmatch(nota); m != nil {
		a, _ := strconv.Atoi(m[2])
		b, _ := strconv.Atoi(m[3])
		primeiroEhCorinthians := c1
		if nome := strings.TrimSpace(m[1]); nome != "" && !strings.EqualFold(nome, "on") {
			primeiroEhCorinthians = ehCorinthians(nome)
		}
		if !primeiroEhCorinthians {
			a, b = b, a
		}
		p.PenaltisPro, p.PenaltisContra = &a, &b
	}
	e.partidas = append(e.partidas, p)
	return len(e.partidas) - 1, c1, true
}

func placar(s string) (int, int, bool) {
	m := reNumeros.FindStringSubmatch(strings.ReplaceAll(s, " ", ""))
	if m == nil {
		return 0, 0, false
	}
	a, _ := strconv.Atoi(m[1])
	b, _ := strconv.Atoi(m[2])
	return a, b, true
}

// jogo: "Corinthians    2-2  Juventude   [nota]", com a data corrente.
func (e *extrator) jogo(t string) bool {
	m := reJogo.FindStringSubmatch(t)
	if m == nil {
		return false
	}
	e.pend = nil
	g1, g2, ok := placar(m[2])
	if !ok {
		if ehCorinthians(m[1]) != ehCorinthians(m[3]) {
			e.aviso("jogo do Corinthians sem placar numérico (%q) ficou de fora: %s", m[2], t)
		}
		return true
	}
	idx, t1, ok := e.novoJogo(e.data, m[1], m[3], g1, g2, m[4], true)
	if ok && g1+g2 > 0 {
		e.pend = &pendente{pernas: []perna{{idx: idx, time1Pro: t1, g1: g1, g2: g2}}}
	}
	return true
}

// jogoComData: "23/04/1922 Palestra Itália 2 x 2 Corinthians" e "24/Jan A 0-1 B".
func (e *extrator) jogoComData(t string) bool {
	m := reDataNaLinha.FindStringSubmatch(t)
	if m == nil {
		return false
	}
	e.pend = nil
	data := e.dataMes(m[2], m[1], m[3])
	g1, _ := strconv.Atoi(m[5])
	g2, _ := strconv.Atoi(m[6])
	idx, t1, ok := e.novoJogo(data, m[4], strings.TrimSpace(m[7]), g1, g2, m[8], true)
	if ok && idx >= 0 {
		e.pend = &pendente{corrido: true, idx: idx}
		if g1+g2 > 0 {
			e.pend.pernas = []perna{{idx: idx, time1Pro: t1, g1: g1, g2: g2}}
		}
	}
	return true
}

// confronto: "9 Itumbiara (GO) - Corinthians (SP)   0-2 (04/03)   n/p (19/03)".
func (e *extrator) confronto(t string) bool {
	m := reConfronto.FindStringSubmatch(t)
	if m == nil {
		return false
	}
	e.pend = nil
	time1, time2 := strings.TrimSpace(m[1]), strings.TrimSpace(m[2])
	if ehCorinthians(time1) == ehCorinthians(time2) {
		return true
	}
	pend := &pendente{}
	pernas := [][2]string{{m[3], m[4]}, {m[5], m[6]}}
	for i, pr := range pernas {
		if pr[0] == "" {
			continue
		}
		g1, g2, ok := placar(pr[0])
		if !ok {
			continue // n/p: não jogado
		}
		data := ""
		if d := reDiaMes.FindStringSubmatch(strings.TrimSpace(pr[1])); d != nil {
			data = e.dataMes(d[2], d[1], d[3])
		}
		nota := ""
		if i == 1 || m[5] == "" || !reNumeros.MatchString(strings.ReplaceAll(m[5], " ", "")) {
			nota = m[7] // pênaltis valem para o último jogo disputado
		}
		idx, t1, _ := e.novoJogo(data, time1, time2, g1, g2, nota, i == 0)
		if g1+g2 > 0 {
			pend.pernas = append(pend.pernas, perna{idx: idx, time1Pro: t1, g1: g1, g2: g2})
		}
	}
	if len(pend.pernas) > 0 {
		e.pend = pend
	}
	return true
}

// idaEVolta: "Operário(MT)1-1 0-4 Corinthians(SP)", sem datas (a não ser que a
// linha anterior traga "[Mar 16, 22]").
func (e *extrator) idaEVolta(t string) bool {
	m := reIdaVolta.FindStringSubmatch(t)
	if m == nil {
		return false
	}
	e.pend = nil
	datas := e.datas
	e.datas = nil
	time1, time2 := strings.TrimSpace(m[1]), strings.TrimSpace(m[4])
	if ehCorinthians(time1) == ehCorinthians(time2) {
		return true
	}
	pend := &pendente{}
	for i, s := range []string{m[2], m[3]} {
		g1, g2, ok := placar(s)
		if !ok {
			continue
		}
		data := ""
		if len(datas) == 2 {
			data = datas[i]
		}
		nota := ""
		if i == 1 {
			nota = m[5]
		}
		idx, t1, _ := e.novoJogo(data, time1, time2, g1, g2, nota, i == 0)
		if g1+g2 > 0 {
			pend.pernas = append(pend.pernas, perna{idx: idx, time1Pro: t1, g1: g1, g2: g2})
		}
	}
	if len(pend.pernas) > 0 {
		e.pend = pend
	}
	return true
}

// sumula trata as linhas que seguem um jogo com data na linha: gols com placar
// corrido, árbitro e público. Devolve true se a linha foi consumida.
func (e *extrator) sumula(t string, recuada bool) bool {
	p := e.pend
	if p.idx < 0 {
		return false
	}
	partida := &e.partidas[p.idx]
	if m := reArbitro.FindStringSubmatch(t); m != nil {
		partida.Arbitro = m[1]
		return true
	}
	if m := rePublico.FindStringSubmatch(t); m != nil {
		if n, err := strconv.Atoi(strings.NewReplacer(" ", "", ".", "").Replace(m[1])); err == nil {
			partida.Publico = &n
		}
		return true
	}
	if !recuada {
		return false
	}
	m := rePlacarFim.FindStringSubmatchIndex(t)
	if m == nil {
		return false
	}
	n1, _ := strconv.Atoi(t[m[2]:m[3]])
	n2, _ := strconv.Atoi(t[m[4]:m[5]])
	gols := lerGols(t[:m[0]])
	if len(gols) != 1 {
		return false
	}
	if len(p.pernas) == 0 {
		return false
	}
	time1Pro := p.pernas[0].time1Pro
	var lado1 bool // gol do time 1?
	switch {
	case n1 == p.g1+1 && n2 == p.g2:
		lado1 = true
	case n2 == p.g2+1 && n1 == p.g1:
		lado1 = false
	default:
		e.aviso("placar corrido incoerente em %s contra %s: %s", partida.Data, partida.Adversario, t)
		return true
	}
	p.g1, p.g2 = n1, n2
	g := gols[0]
	g.Lado = "contra"
	if lado1 == time1Pro {
		g.Lado = "pro"
	}
	partida.Gols = append(partida.Gols, g)
	p.pernas[0].g1, p.pernas[0].g2 = -1, -1 // a linha de gols entre colchetes não vale mais
	return true
}

// golsEntreColchetes: "[Tevez 9', Mattos 43'; Enilton 6', Naldo 78']".
func (e *extrator) golsEntreColchetes(t string) {
	p := e.pend
	if len(p.pernas) == 0 {
		return
	}
	pr := p.pernas[0]
	p.pernas = p.pernas[1:]
	if pr.idx < 0 || pr.g1 < 0 {
		return
	}
	partida := &e.partidas[pr.idx]
	conteudo := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(t, "["), "]"))
	if i := strings.LastIndex(conteudo, "]"); i >= 0 { // nota depois dos gols
		conteudo = conteudo[:i]
	}
	var lista1, lista2 string
	switch partes := strings.SplitN(conteudo, ";", 2); {
	case len(partes) == 2:
		lista1, lista2 = partes[0], partes[1]
	case pr.g2 == 0:
		lista1 = conteudo
	case pr.g1 == 0:
		lista2 = conteudo
	default:
		e.aviso("gols sem separação entre os times em %s contra %s: %s", partida.Data, partida.Adversario, t)
		return
	}
	lado1, lado2 := "pro", "contra"
	if !pr.time1Pro {
		lado1, lado2 = "contra", "pro"
	}
	var gols []banco.Gol
	for _, g := range lerGols(lista1) {
		g.Lado = lado1
		gols = append(gols, g)
	}
	for _, g := range lerGols(lista2) {
		g.Lado = lado2
		gols = append(gols, g)
	}
	pro, contra := 0, 0
	for _, g := range gols {
		if g.Lado == "pro" {
			pro++
		} else {
			contra++
		}
	}
	if pro != *partida.GolsPro || contra != *partida.GolsContra {
		e.aviso("gols não batem com o placar em %s contra %s (%d-%d, lidos %d-%d): %s",
			partida.Data, partida.Adversario, *partida.GolsPro, *partida.GolsContra, pro, contra, t)
		return
	}
	partida.Gols = gols
}

// lerGols lê "Deivid 4', 90', Robinho 20'p, Leonardo 20'og": o nome vale para
// os minutos seguintes sem nome.
func lerGols(lista string) []banco.Gol {
	var out []banco.Gol
	autor := ""
	for _, peca := range reSepGols.Split(strings.TrimSpace(lista), -1) {
		peca = strings.TrimSpace(peca)
		if peca == "" {
			continue
		}
		tipo := "normal"
		marcar := func(s string) {
			switch strings.TrimSuffix(strings.ToLower(s), ".") {
			case "og", "o.g", "gc", "contra":
				tipo = "contra"
			case "pen", "p", "pk", "pênalti":
				tipo = "penalti"
			}
		}
		if m := reMarcaGol.FindStringSubmatch(peca); m != nil {
			marcar(m[1])
			peca = strings.TrimSpace(reMarcaGol.ReplaceAllString(peca, " "))
		}
		minuto := ""
		nome := peca
		if m := reGol.FindStringSubmatch(peca); m != nil {
			nome, minuto = m[1], m[2]
			if m[3] != "" {
				minuto += "+" + m[3]
			}
			if m[4] != "" {
				marcar(m[4])
			}
		}
		nome = strings.Trim(strings.TrimSpace(nome), "'(),.")
		nome = strings.TrimSpace(reEspacos.ReplaceAllString(nome, " "))
		if nome == "" {
			nome = autor
		}
		if nome == "" {
			continue
		}
		autor = nome
		out = append(out, banco.Gol{Autor: nome, Minuto: minuto, Tipo: tipo})
	}
	return out
}
