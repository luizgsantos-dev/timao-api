// Package openfootball importa partidas do repositório openfootball/south-america
// (domínio público, CC0), no formato texto Football.TXT. Ele cobre Brasileirão,
// Copa do Brasil, Libertadores e Sul-Americana, com placar, fase e horário, mas
// sem autores dos gols. Só entram as partidas do Corinthians (masculino).
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
	Licenca = "CC0 1.0"
	repo    = "openfootball/south-america"
)

// Pastas e sufixos de arquivo -> competição.
var arquivos = []struct {
	pasta, sufixo, competicao string
}{
	{"brazil", "_br1.txt", "Campeonato Brasileiro"},
	{"brazil", "_brcup.txt", "Copa do Brasil"},
	{"copa-libertadores", "_copal.txt", "Copa Libertadores"},
	{"copa-libertadores", "_copas.txt", "Copa Sul-Americana"},
}

type OpenFootball struct {
	API    string // API do GitHub, para listar os arquivos
	Raw    string // onde baixar o conteúdo dos arquivos
	Coleta *coleta.Cliente
}

func Novo(c *coleta.Cliente) *OpenFootball {
	return &OpenFootball{
		API:    "https://api.github.com/repos/" + repo + "/contents/",
		Raw:    "https://raw.githubusercontent.com/" + repo + "/master/",
		Coleta: c,
	}
}

// Arquivo é um arquivo de temporada já lido.
type Arquivo struct {
	Caminho  string
	URL      string
	Partidas []banco.Partida
}

// Partidas lista os arquivos de cada pasta e extrai as partidas do Corinthians.
func (o *OpenFootball) Partidas(ctx context.Context) ([]Arquivo, error) {
	var out []Arquivo
	listados := map[string][]string{}
	for _, a := range arquivos {
		nomes, ok := listados[a.pasta]
		if !ok {
			var err error
			if nomes, err = o.listar(ctx, a.pasta); err != nil {
				return nil, fmt.Errorf("openfootball: listando %s: %w", a.pasta, err)
			}
			listados[a.pasta] = nomes
		}
		for _, nome := range nomes {
			if !strings.HasSuffix(nome, a.sufixo) {
				continue
			}
			temporada, err := strconv.Atoi(strings.TrimSuffix(nome, a.sufixo))
			if err != nil {
				continue
			}
			caminho := a.pasta + "/" + nome
			body, err := o.Coleta.Get(ctx, o.Raw+caminho, "")
			if err != nil {
				return nil, fmt.Errorf("openfootball: %s: %w", caminho, err)
			}
			out = append(out, Arquivo{
				Caminho:  caminho,
				URL:      "https://github.com/" + repo + "/blob/master/" + caminho,
				Partidas: Extrair(string(body), a.competicao, temporada),
			})
		}
	}
	return out, nil
}

func (o *OpenFootball) listar(ctx context.Context, pasta string) ([]string, error) {
	body, err := o.Coleta.Get(ctx, o.API+pasta, "application/vnd.github+json")
	if err != nil {
		return nil, err
	}
	var itens []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &itens); err != nil {
		return nil, fmt.Errorf("resposta inválida: %w", err)
	}
	nomes := make([]string, 0, len(itens))
	for _, i := range itens {
		nomes = append(nomes, i.Name)
	}
	sort.Strings(nomes)
	return nomes, nil
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

// nomeDoTime tira o país ("(ARG)") e a sigla do estado ("RJ") do nome.
func nomeDoTime(s string) string {
	s = rePais.ReplaceAllString(strings.TrimSpace(s), "")
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
