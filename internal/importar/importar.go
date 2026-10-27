// Package importar gera o timao.db a partir de fontes abertas:
//   - Wikidata: pessoas e passagens;
//   - Wikipédia: títulos, partidas das páginas de temporada (com gols), elencos,
//     técnicos(as) e categorias de jogadoras e técnicos; partidas das páginas de
//     edição das competições femininas (jogos das Brabas antes de 2022);
//   - openfootball, RSSSF e pacote brasileirao: partidas que faltam na Wikipédia,
//     e gols e outros dados que faltam nas partidas já gravadas.
//
// Quando mais de uma fonte traz a mesma partida, vale a primeira, na ordem acima.
package importar

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"timao-api/internal/banco"
	"timao-api/internal/importar/brasileirao"
	"timao-api/internal/importar/coleta"
	"timao-api/internal/importar/openfootball"
	"timao-api/internal/importar/rsssf"
	"timao-api/internal/importar/wikidata"
	"timao-api/internal/importar/wikipedia"
)

// Executar gera o banco em saida a partir de todas as fontes. Uma importação que
// falha no meio não estraga o arquivo anterior (veja banco.Criar).
func Executar(ctx context.Context, log *slog.Logger, saida string, detalhes bool) error {
	inicio := time.Now()
	c := coleta.Novo()

	esc, err := banco.Criar(ctx, saida)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		if !ok {
			esc.Descartar()
		}
	}()

	wd := wikidata.Novo(c)
	reg, err := importarWikidata(ctx, log, esc, wd)
	if err != nil {
		return err
	}
	wp := wikipedia.Novo(c)
	if err := importarTitulos(ctx, log, esc, wp, detalhes); err != nil {
		return err
	}
	temporadas, err := importarWikipedia(ctx, log, esc, wp, detalhes)
	if err != nil {
		return err
	}
	if err := importarPessoasDaWikipedia(ctx, log, esc, wp, wd, reg, temporadas); err != nil {
		return err
	}
	if err := importarCompeticoes(ctx, log, esc, wp, temporadas, detalhes); err != nil {
		return err
	}
	if err := importarOpenFootball(ctx, log, esc, openfootball.Novo(c), detalhes); err != nil {
		return err
	}
	if err := importarRSSSF(ctx, log, esc, rsssf.Novo(c), detalhes); err != nil {
		return err
	}
	if err := importarBrasileirao(ctx, log, esc, brasileirao.Novo(c), detalhes); err != nil {
		return err
	}
	ajustadas, err := esc.AjustarPassagensPelosGols(ctx)
	if err != nil {
		return err
	}
	log.Info("passagens recuadas até o primeiro gol", "passagens", ajustadas)

	if err := esc.Concluir(ctx, inicio); err != nil {
		return err
	}
	ok = true
	log.Info("banco gerado", "arquivo", saida, "duracao", time.Since(inicio).Round(time.Second))
	return nil
}

func importarWikidata(ctx context.Context, log *slog.Logger, esc *banco.Escritor, wd *wikidata.Wikidata) (*registro, error) {
	log.Info("consultando o Wikidata")
	agora := time.Now()
	r, err := wd.Importar(ctx)
	if err != nil {
		return nil, err
	}
	fonteID, err := esc.Fonte(ctx, banco.Fonte{
		Nome:       "Wikidata",
		URL:        wikidata.Endpoint,
		Licenca:    wikidata.Licenca,
		ColetadoEm: agora,
	})
	if err != nil {
		return nil, err
	}

	reg := novoRegistro(fonteID)
	for _, p := range r.Pessoas {
		if err := reg.gravarPessoa(ctx, esc, p); err != nil {
			return nil, err
		}
	}
	cont := map[string]int{}
	for _, v := range r.Vinculos {
		id, ok := reg.porQID[v.Pessoa]
		if !ok {
			continue // pessoa sem rótulo nem dados: o Wikidata não devolveu detalhes
		}
		if err := reg.gravarPassagem(ctx, esc, id, v.Passagem, fonteID); err != nil {
			return nil, err
		}
		cont[v.Passagem.Categoria+"/"+v.Passagem.Funcao]++
	}
	log.Info("Wikidata importado", "pessoas", len(r.Pessoas),
		"jogadores", cont["masculino/jogador"], "jogadoras", cont["feminino/jogador"],
		"tecnicos_masc", cont["masculino/tecnico"], "tecnicos_fem", cont["feminino/tecnico"])
	return reg, nil
}

// paginaGravada é uma página de temporada já gravada, com o id da sua fonte.
type paginaGravada struct {
	wikipedia.Pagina
	fonteID int64
}

func importarWikipedia(ctx context.Context, log *slog.Logger, esc *banco.Escritor, wp *wikipedia.Wikipedia, detalhes bool) ([]paginaGravada, error) {
	log.Info("lendo as páginas de temporada da Wikipédia")
	agora := time.Now()
	paginas, err := wp.Temporadas(ctx)
	if err != nil {
		return nil, err
	}
	var out []paginaGravada
	for _, pg := range paginas {
		fonteID, err := esc.Fonte(ctx, banco.Fonte{
			Nome:       "Wikipédia (en): " + pg.Titulo,
			URL:        pg.URL,
			Licenca:    wikipedia.Licenca,
			ColetadoEm: agora,
		})
		if err != nil {
			return nil, err
		}
		gravadas := 0
		for _, p := range pg.Partidas {
			novo, err := esc.Partida(ctx, p, fonteID)
			if err != nil {
				return nil, err
			}
			if novo {
				gravadas++
			}
		}
		log.Info("temporada", "categoria", pg.Categoria, "ano", pg.Temporada,
			"partidas", gravadas, "repetidas", len(pg.Partidas)-gravadas, "elenco", len(pg.Elenco),
			"tecnicos", len(pg.Tecnicos), "avisos", len(pg.Avisos))
		if detalhes {
			for _, a := range pg.Avisos {
				fmt.Fprintf(os.Stderr, "    %s: %s\n", pg.Titulo, a)
			}
		}
		out = append(out, paginaGravada{Pagina: pg, fonteID: fonteID})
	}
	return out, nil
}

func importarTitulos(ctx context.Context, log *slog.Logger, esc *banco.Escritor, wp *wikipedia.Wikipedia, detalhes bool) error {
	log.Info("lendo os títulos das páginas dos clubes na Wikipédia")
	agora := time.Now()
	paginas, err := wp.Titulos(ctx)
	if err != nil {
		return err
	}
	for _, pg := range paginas {
		fonteID, err := esc.Fonte(ctx, banco.Fonte{
			Nome:       "Wikipédia (en): " + pg.Titulo,
			URL:        pg.URL,
			Licenca:    wikipedia.Licenca,
			ColetadoEm: agora,
		})
		if err != nil {
			return err
		}
		oficiais, outros, repetidos := 0, 0, 0
		for _, t := range pg.Titulos {
			novo, err := esc.Titulo(ctx, t, fonteID)
			if err != nil {
				return err
			}
			switch {
			case !novo:
				repetidos++
			case t.Oficial:
				oficiais++
			default:
				outros++
			}
		}
		log.Info("títulos", "categoria", pg.Categoria, "oficiais", oficiais, "outros", outros,
			"repetidos", repetidos, "avisos", len(pg.Avisos))
		if detalhes {
			for _, a := range pg.Avisos {
				fmt.Fprintf(os.Stderr, "    %s: %s\n", pg.Titulo, a)
			}
		}
	}
	return importarTitulosRecentes(ctx, log, esc, wp, agora)
}

// importarTitulosRecentes acrescenta os títulos do ano atual e do anterior que o
// quadro da página da edição já mostra, mas a tabela "Honours" ainda não lista.
func importarTitulosRecentes(ctx context.Context, log *slog.Logger, esc *banco.Escritor, wp *wikipedia.Wikipedia, agora time.Time) error {
	ano := agora.Year()
	detectados, err := wp.TitulosRecentes(ctx, []int{ano - 1, ano})
	if err != nil {
		return err
	}
	novos := 0
	for _, d := range detectados {
		fonteID, err := esc.Fonte(ctx, banco.Fonte{Nome: "Wikipédia (" + d.Lingua + "): " + d.Pagina, URL: d.URL,
			Licenca: wikipedia.Licenca, ColetadoEm: agora})
		if err != nil {
			return err
		}
		novo, err := esc.Titulo(ctx, d.Titulo, fonteID)
		if err != nil {
			return err
		}
		if novo {
			novos++
			log.Info("título detectado no quadro da edição", "categoria", d.Titulo.Categoria,
				"competicao", d.Titulo.Competicao, "ano", d.Titulo.Ano, "pagina", d.Pagina)
		}
	}
	log.Info("títulos recentes conferidos", "corinthians_campeao", len(detectados), "novos", novos)
	return nil
}

func importarOpenFootball(ctx context.Context, log *slog.Logger, esc *banco.Escritor, of *openfootball.OpenFootball, detalhes bool) error {
	log.Info("lendo os arquivos do openfootball")
	agora := time.Now()
	arqs, err := of.Partidas(ctx)
	if err != nil {
		return err
	}
	var total uniao
	for _, a := range arqs {
		if len(a.Partidas) == 0 {
			continue
		}
		fonteID, err := esc.Fonte(ctx, banco.Fonte{Nome: "openfootball: " + a.Caminho, URL: a.URL, Licenca: openfootball.Licenca, ColetadoEm: agora})
		if err != nil {
			return err
		}
		u, err := complementar(ctx, esc, a.Partidas, fonteID)
		if err != nil {
			return err
		}
		total.somar(u)
	}
	total.registrar(log, "openfootball", detalhes)
	return nil
}

func importarBrasileirao(ctx context.Context, log *slog.Logger, esc *banco.Escritor, br *brasileirao.Brasileirao, detalhes bool) error {
	log.Info("lendo os jogos do pacote brasileirao")
	agora := time.Now()
	ps, err := br.Partidas(ctx)
	if err != nil {
		return err
	}
	fonteID, err := esc.Fonte(ctx, banco.Fonte{Nome: "Pacote brasileirao (William Amorim)", URL: brasileirao.Pagina, Licenca: brasileirao.Licenca, ColetadoEm: agora})
	if err != nil {
		return err
	}
	u, err := complementar(ctx, esc, ps, fonteID)
	if err != nil {
		return err
	}
	u.registrar(log, "brasileirao", detalhes)
	return nil
}

// uniao conta o que aconteceu ao juntar uma fonte complementar ao banco.
type uniao struct {
	novas, existentes int
	enriquecidas      int // existentes que ganharam gols ou campos vazios
	porAno            map[int]int
	divergencias      []string
}

func (u *uniao) somar(o uniao) {
	u.novas += o.novas
	u.existentes += o.existentes
	u.enriquecidas += o.enriquecidas
	u.divergencias = append(u.divergencias, o.divergencias...)
	if u.porAno == nil {
		u.porAno = map[int]int{}
	}
	for a, n := range o.porAno {
		u.porAno[a] += n
	}
}

func (u uniao) registrar(log *slog.Logger, fonte string, detalhes bool) {
	anos := make([]string, 0, len(u.porAno))
	for a, n := range u.porAno {
		anos = append(anos, fmt.Sprintf("%d:%d", a, n))
	}
	sort.Strings(anos)
	log.Info("fonte complementar", "fonte", fonte, "novas", u.novas, "ja_existiam", u.existentes, "enriquecidas", u.enriquecidas,
		"placares_divergentes", len(u.divergencias), "novas_por_ano", strings.Join(anos, " "))
	if detalhes {
		for _, d := range u.divergencias {
			fmt.Fprintf(os.Stderr, "    %s: %s\n", fonte, d)
		}
	}
}

// complementar grava as partidas que ainda não existem e completa as que existem
// (veja banco.Escritor.Enriquecer). Uma partida já existe se:
//   - há outra da categoria a até 1 dia (o Corinthians não joga em dias seguidos); ou
//   - há outra contra o mesmo adversário a até 5 dias, sem placar ou com o mesmo
//     placar (datas provisórias de jogos futuros, ou fontes que discordam da data).
//
// Para as que já existem, compara o placar e registra as divergências.
func complementar(ctx context.Context, esc *banco.Escritor, ps []banco.Partida, fonteID int64) (uniao, error) {
	u := uniao{porAno: map[int]int{}}
	for _, p := range ps {
		vizinhas, err := esc.PartidasPerto(ctx, p.Categoria, p.Data, 5)
		if err != nil {
			return u, err
		}
		if v, ok := mesmaPartida(p, vizinhas); ok {
			u.existentes++
			if v.GolsPro != nil && p.GolsPro != nil && (*v.GolsPro != *p.GolsPro || *v.GolsContra != *p.GolsContra) {
				u.divergencias = append(u.divergencias, fmt.Sprintf("%s x %s: banco %d x %d, fonte %d x %d",
					p.Data, p.Adversario, *v.GolsPro, *v.GolsContra, *p.GolsPro, *p.GolsContra))
				continue // placares diferentes: não mistura dados das duas fontes
			}
			mudou, err := esc.Enriquecer(ctx, v.ID, p)
			if err != nil {
				return u, err
			}
			if mudou {
				u.enriquecidas++
			}
			continue
		}
		novo, err := esc.Partida(ctx, p, fonteID)
		if err != nil {
			return u, err
		}
		if novo {
			u.novas++
			u.porAno[p.Temporada]++
		}
	}
	return u, nil
}

// mesmaPartida aplica as regras de complementar às partidas vizinhas já gravadas.
func mesmaPartida(p banco.Partida, vizinhas []banco.Vizinha) (banco.Vizinha, bool) {
	dia, _ := time.Parse("2006-01-02", p.Data)
	for _, v := range vizinhas {
		d, _ := time.Parse("2006-01-02", v.Data)
		if dist := d.Sub(dia).Hours() / 24; dist >= -1 && dist <= 1 {
			return v, true
		}
		semPlacar := p.GolsPro == nil || v.GolsPro == nil
		mesmoPlacar := !semPlacar && *p.GolsPro == *v.GolsPro && *p.GolsContra == *v.GolsContra
		if banco.MesmoAdversario(p.Adversario, v.Adversario) && (semPlacar || mesmoPlacar) {
			return v, true
		}
	}
	return banco.Vizinha{}, false
}

// Anos lidos no RSSSF: os que não têm página de temporada na Wikipédia, onde ele
// traz partidas e gols que faltam. A partir de 2015, a Wikipédia cobre tudo.
const (
	rsssfPrimeiroBrasileirao = 2003
	rsssfPrimeiraCopa        = 1989
	rsssfPrimeiroPaulista    = 1902
	rsssfUltimoAno           = 2014
)

func importarRSSSF(ctx context.Context, log *slog.Logger, esc *banco.Escritor, r *rsssf.RSSSF, detalhes bool) error {
	log.Info("lendo as páginas do RSSSF e do RSSSF Brasil")
	agora := time.Now()
	anos := func(de int) []int {
		var out []int
		for a := de; a <= rsssfUltimoAno; a++ {
			out = append(out, a)
		}
		return out
	}
	familias := []struct {
		nome string
		ler  func(context.Context, []int) ([]rsssf.Pagina, error)
		de   int
	}{
		{"Campeonato Brasileiro", r.Brasileirao, rsssfPrimeiroBrasileirao},
		{"Copa do Brasil", r.CopaDoBrasil, rsssfPrimeiraCopa},
		{"Campeonato Paulista", r.Paulista, rsssfPrimeiroPaulista},
	}
	var total uniao
	semData := 0
	for _, f := range familias {
		paginas, err := f.ler(ctx, anos(f.de))
		if err != nil {
			return err
		}
		for _, pg := range paginas {
			semData += pg.SemData
			if detalhes {
				for _, a := range pg.Avisos {
					fmt.Fprintf(os.Stderr, "    RSSSF %s %d: %s\n", pg.Competicao, pg.Temporada, a)
				}
			}
			if len(pg.Partidas) == 0 {
				continue
			}
			// Crédito ao autor de cada página, como pedem os termos do RSSSF.
			fonteID, err := esc.Fonte(ctx, banco.Fonte{
				Nome:       fmt.Sprintf("RSSSF: %s %d, por %s", pg.Competicao, pg.Temporada, pg.Autor),
				URL:        pg.URL,
				Licenca:    "Cópia permitida com crédito ao autor (" + pg.Autor + "); demais direitos reservados",
				ColetadoEm: agora,
			})
			if err != nil {
				return err
			}
			u, err := complementar(ctx, esc, pg.Partidas, fonteID)
			if err != nil {
				return err
			}
			total.somar(u)
		}
	}
	total.registrar(log, "RSSSF", detalhes)
	log.Info("RSSSF: jogos achados sem data ficaram de fora", "sem_data", semData)
	return nil
}
