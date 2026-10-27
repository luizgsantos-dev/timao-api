// Package openfootball importa partidas dos repositórios openfootball/south-america
// e openfootball/club-worldcup (domínio público, CC0), no formato texto
// Football.TXT. Eles cobrem Brasileirão, Copa do Brasil, Libertadores,
// Sul-Americana e Mundial de Clubes, com placar, fase e horário, mas sem autores
// dos gols. Só entram as partidas do Corinthians (masculino).
package openfootball

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"timao-api/internal/banco"
	"timao-api/internal/importar/coleta"
	"timao-api/internal/importar/nomes"
)

const (
	Licenca      = "CC0 1.0"
	southAmerica = "openfootball/south-america"
	clubWorldCup = "openfootball/club-worldcup"
)

// Arquivos de cada repositório -> competição. O caminho tem a temporada no
// primeiro grupo.
var arquivos = []struct {
	repo       string
	caminho    *regexp.Regexp
	competicao string
}{
	{southAmerica, regexp.MustCompile(`^brazil/(\d{4})_br1\.txt$`), "Campeonato Brasileiro"},
	{southAmerica, regexp.MustCompile(`^brazil/(\d{4})_brcup\.txt$`), "Copa do Brasil"},
	{southAmerica, regexp.MustCompile(`^copa-libertadores/(\d{4})_copal\.txt$`), "Copa Libertadores"},
	{southAmerica, regexp.MustCompile(`^copa-libertadores/(\d{4})_copas\.txt$`), "Copa Sul-Americana"},
	// A pasta "more" do mesmo repositório usa outro formato de linha; vale a do ano.
	{clubWorldCup, regexp.MustCompile(`^(\d{4})/clubworldcup\.txt$`), "Mundial de Clubes da FIFA"},
}

type OpenFootball struct {
	API    string // API do GitHub, para listar os arquivos
	Raw    string // onde baixar o conteúdo dos arquivos
	Coleta *coleta.Cliente
}

func Novo(c *coleta.Cliente) *OpenFootball {
	return &OpenFootball{
		API:    "https://api.github.com/repos/",
		Raw:    "https://raw.githubusercontent.com/",
		Coleta: c,
	}
}

// Arquivo é um arquivo de temporada já lido.
type Arquivo struct {
	Caminho  string
	URL      string
	Partidas []banco.Partida
}

// Partidas lista os arquivos de cada repositório e extrai as partidas do Corinthians.
func (o *OpenFootball) Partidas(ctx context.Context) ([]Arquivo, error) {
	var out []Arquivo
	listados := map[string][]string{}
	for _, a := range arquivos {
		caminhos, ok := listados[a.repo]
		if !ok {
			var err error
			if caminhos, err = o.listar(ctx, a.repo); err != nil {
				return nil, fmt.Errorf("openfootball: listando %s: %w", a.repo, err)
			}
			listados[a.repo] = caminhos
		}
		for _, caminho := range caminhos {
			m := a.caminho.FindStringSubmatch(caminho)
			if m == nil {
				continue
			}
			temporada, _ := strconv.Atoi(m[1])
			body, err := o.Coleta.Get(ctx, o.Raw+a.repo+"/master/"+caminho, "")
			if err != nil {
				return nil, fmt.Errorf("openfootball: %s/%s: %w", a.repo, caminho, err)
			}
			out = append(out, Arquivo{
				Caminho:  strings.TrimPrefix(a.repo, "openfootball/") + "/" + caminho,
				URL:      "https://github.com/" + a.repo + "/blob/master/" + caminho,
				Partidas: Extrair(string(body), a.competicao, temporada),
			})
		}
	}
	return out, nil
}

// listar devolve os caminhos de todos os arquivos do repositório, numa só
// requisição à API de árvores do GitHub.
func (o *OpenFootball) listar(ctx context.Context, repo string) ([]string, error) {
	body, err := o.Coleta.Get(ctx, o.API+repo+"/git/trees/master?recursive=1", "application/vnd.github+json")
	if err != nil {
		return nil, err
	}
	var arvore struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"tree"`
		Truncated bool `json:"truncated"`
	}
	if err := json.Unmarshal(body, &arvore); err != nil {
		return nil, fmt.Errorf("resposta inválida: %w", err)
	}
	if arvore.Truncated {
		return nil, fmt.Errorf("a listagem de %s veio incompleta", repo)
	}
	var caminhos []string
	for _, i := range arvore.Tree {
		if i.Type == "blob" {
			caminhos = append(caminhos, i.Path)
		}
	}
	sort.Strings(caminhos)
	return caminhos, nil
}

var (
	// "▪ Matchday 1", "▪ Gruppe 6", "▪ Final"
	reFase = regexp.MustCompile(`^\s*▪\s*(.+?)\s*$`)
	// "  Sat Apr 14 2018" ou "  Sun Apr 15"
	reData = regexp.MustCompile(`^\s+(Mon|Tue|Wed|Thu|Fri|Sat|Sun)\s+([A-Z][a-z]{2})\s+(\d{1,2})(?:\s+(\d{4}))?\s*$`)
	// "    16:00  Cruzeiro   v Grêmio   0-1 (0-0)" (horário opcional; placar opcional)
	reJogoComPlacar = regexp.MustCompile(`^\s+(?:(\d{1,2}:\d{2})\s+)?(.+?)\s+v\s+(.+?)\s+(\d+-\d+.*)$`)
	reJogoSemPlacar = regexp.MustCompile(`^\s+(?:(\d{1,2}:\d{2})\s+)?(.+?)\s+v\s+(.+?)\s*$`)
	rePenaltis      = regexp.MustCompile(`(\d+)-(\d+)\s*pen\.`)
	reProrrogacao   = regexp.MustCompile(`(\d+)-(\d+)\s*a\.e\.t\.`)
	reTempoNormal   = regexp.MustCompile(`^(\d+)-(\d+)\s*(?:\(|$)`)
	reParenteses    = regexp.MustCompile(`\((\d+)-(\d+)`)
	rePais          = regexp.MustCompile(`\s*\([A-Z]{3}\)$`)
	reEstado        = regexp.MustCompile(`\s+[A-Z]{2}$`)
	reCorinthians   = regexp.MustCompile(`(?i)corinthians`)
)

// Extrair lê um arquivo Football.TXT e devolve as partidas do Corinthians.
func Extrair(texto, competicao string, temporada int) []banco.Partida {
	var out []banco.Partida
	fase, data := "", ""
	for _, linha := range strings.Split(strings.ReplaceAll(texto, "\r\n", "\n"), "\n") {
		if m := reFase.FindStringSubmatch(linha); m != nil {
			fase = nomes.Fase(m[1])
			continue
		}
		if m := reData.FindStringSubmatch(linha); m != nil {
			data = resolverData(m[1], m[2], m[3], m[4], temporada)
			continue
		}
		if data == "" || !strings.Contains(linha, " v ") {
			continue
		}
		var horario, time1, time2, placar string
		if m := reJogoComPlacar.FindStringSubmatch(linha); m != nil {
			horario, time1, time2, placar = m[1], m[2], m[3], m[4]
		} else if m := reJogoSemPlacar.FindStringSubmatch(linha); m != nil {
			horario, time1, time2 = m[1], m[2], m[3]
		} else {
			continue
		}
		c1, c2 := reCorinthians.MatchString(time1), reCorinthians.MatchString(time2)
		if c1 == c2 {
			continue
		}
		p := banco.Partida{
			Categoria:  banco.Masculino,
			Temporada:  temporada,
			Data:       data,
			Horario:    horario,
			Competicao: competicao,
			Fase:       fase,
			Mando:      "casa",
			Adversario: nomeDoTime(time2),
		}
		if c2 {
			p.Mando, p.Adversario = "fora", nomeDoTime(time1)
		}
		lerPlacar(&p, placar)
		out = append(out, p)
	}
	return out
}

// resolverData monta a data de uma linha como "Sun Apr 15". O arquivo só escreve o
// ano às vezes, e não lista os jogos em ordem cronológica (grupos, jogos adiados),
// então o ano é o da temporada ou um vizinho, aquele em que a data cai no dia da
// semana indicado. Devolve "" se nenhum servir.
func resolverData(semana, mes, dia, ano string, temporada int) string {
	t, err := time.Parse("Jan 2", mes+" "+dia)
	if err != nil {
		return ""
	}
	anos := []int{temporada, temporada + 1, temporada - 1}
	if ano != "" {
		n, _ := strconv.Atoi(ano)
		anos = []int{n}
	}
	for _, a := range anos {
		d := time.Date(a, t.Month(), t.Day(), 12, 0, 0, 0, time.UTC)
		if d.Day() == t.Day() && d.Weekday().String()[:3] == semana {
			return d.Format("2006-01-02")
		}
	}
	return ""
}

// Nomes oficiais que os arquivos brasileiros usam -> nome de uso, o mesmo das
// outras fontes. Os que só têm a sigla do estado ("Flamengo RJ") não precisam.
var nomesDeUso = map[string]string{
	"AC Goianiense":       "Atlético Goianiense",
	"América MG":          "América Mineiro",
	"Avaí SC":             "Avaí",
	"Botafogo FR":         "Botafogo",
	"CA Mineiro":          "Atlético Mineiro",
	"CA Paranaense":       "Athletico Paranaense",
	"CR Flamengo":         "Flamengo",
	"CR Vasco da Gama":    "Vasco da Gama",
	"Ceará SC":            "Ceará",
	"Chapecoense AF":      "Chapecoense",
	"Clube do Remo":       "Remo",
	"Coritiba FBC":        "Coritiba",
	"Criciúma EC":         "Criciúma",
	"Cruzeiro EC":         "Cruzeiro",
	"Cuiabá EC":           "Cuiabá",
	"EC Bahia":            "Bahia",
	"EC Juventude":        "Juventude",
	"EC Vitória":          "Vitória",
	"Fluminense FC":       "Fluminense",
	"Fortaleza EC":        "Fortaleza",
	"Grêmio FBPA":         "Grêmio",
	"Grêmio Porto Alegre": "Grêmio",
	"Mirassol FC":         "Mirassol",
	"Paraná Clube":        "Paraná",
	"RB Bragantino":       "Red Bull Bragantino",
	"SC Internacional":    "Internacional",
	"SC Recife":           "Sport Recife",
	"SE Palmeiras":        "Palmeiras",
	"Santos FC":           "Santos",
	"São Paulo FC":        "São Paulo",
}

var reAnotacao = regexp.MustCompile(`\s*\[[^\]]*\]$`) // "[postponed]"

// nomeDoTime tira o país ("(ARG)"), anotações ("[postponed]") e a sigla do estado
// ("RJ") do nome, e troca o nome oficial pelo de uso.
func nomeDoTime(s string) string {
	s = reAnotacao.ReplaceAllString(strings.TrimSpace(s), "")
	s = rePais.ReplaceAllString(s, "")
	if n, ok := nomesDeUso[s]; ok {
		return n
	}
	return reEstado.ReplaceAllString(s, "")
}

// lerPlacar entende "2-1 (1-0)", "1-1 a.e.t." e "4-5 pen. 2-1 a.e.t. (2-1, 0-1)".
// O placar gravado é o do fim da prorrogação, quando houve, ou o do tempo normal.
func lerPlacar(p *banco.Partida, placar string) {
	placar = strings.TrimSpace(placar)
	if placar == "" {
		return
	}
	par := func(m []string) (int, int) {
		a, _ := strconv.Atoi(m[1])
		b, _ := strconv.Atoi(m[2])
		return a, b
	}
	ordenar := func(a, b int) (*int, *int) {
		if p.Mando == "fora" {
			a, b = b, a
		}
		return &a, &b
	}

	resto := placar
	if m := rePenaltis.FindStringSubmatch(placar); m != nil {
		p.PenaltisPro, p.PenaltisContra = ordenar(par(m))
		resto = strings.TrimSpace(strings.Replace(placar, m[0], "", 1))
	}
	switch {
	case reProrrogacao.MatchString(resto):
		p.GolsPro, p.GolsContra = ordenar(par(reProrrogacao.FindStringSubmatch(resto)))
	case reTempoNormal.MatchString(resto):
		p.GolsPro, p.GolsContra = ordenar(par(reTempoNormal.FindStringSubmatch(resto)))
	case reParenteses.MatchString(resto):
		p.GolsPro, p.GolsContra = ordenar(par(reParenteses.FindStringSubmatch(resto)))
	}
}
