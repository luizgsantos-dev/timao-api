package importar

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"time"

	"timao-api/internal/banco"
	"timao-api/internal/importar/wikidata"
	"timao-api/internal/importar/wikipedia"
)

// Categorias da Wikipédia em inglês com quem passou pelo clube.
var categoriasDePessoas = []struct {
	titulo, categoria, funcao string
}{
	{"Category:SC Corinthians Paulista (women) players", banco.Feminino, "jogador"},
	{"Category:SC Corinthians Paulista (women) managers", banco.Feminino, "tecnico"},
	{"Category:SC Corinthians Paulista managers", banco.Masculino, "tecnico"},
}

// registro guarda as pessoas já gravadas, para ligar os dados de outras fontes a elas.
type registro struct {
	fonteWikidata int64
	porQID        map[string]int64
	porWiki       map[string]int64   // artigo na Wikipédia em inglês -> pessoa
	passagens     map[[3]string]bool // pessoa, categoria e função que já têm passagem
}

func novoRegistro(fonteWikidata int64) *registro {
	return &registro{
		fonteWikidata: fonteWikidata,
		porQID:        map[string]int64{},
		porWiki:       map[string]int64{},
		passagens:     map[[3]string]bool{},
	}
}

func (r *registro) gravarPessoa(ctx context.Context, esc *banco.Escritor, p banco.Pessoa) error {
	id, err := esc.Pessoa(ctx, p, r.fonteWikidata)
	if err != nil {
		return err
	}
	r.porQID[p.Wikidata] = id
	if p.WikiEN != "" {
		r.porWiki[p.WikiEN] = id
	}
	return nil
}

func (r *registro) gravarPassagem(ctx context.Context, esc *banco.Escritor, pessoaID int64, pa banco.Passagem, fonteID int64) error {
	if err := esc.Passagem(ctx, pessoaID, pa, fonteID); err != nil {
		return err
	}
	r.passagens[chavePassagem(pessoaID, pa.Categoria, pa.Funcao)] = true
	return nil
}

func chavePassagem(pessoaID int64, categoria, funcao string) [3]string {
	return [3]string{fmt.Sprint(pessoaID), categoria, funcao}
}

// aparicao é alguém citado numa fonte da Wikipédia: num elenco, como técnico(a)
// de uma temporada ou numa categoria (sem temporada).
type aparicao struct {
	wiki, categoria, funcao string
	temporada               int // 0 para categorias
	fonteID                 int64
	saiu                    bool // técnico(a) substituído(a) durante a temporada
}

// importarPessoasDaWikipedia completa as pessoas com quem aparece nos elencos, nos
// técnicos das temporadas e nas categorias da Wikipédia: acha o item do Wikidata
// de cada artigo, grava quem ainda não existe e cria a passagem quando o Wikidata
// não tinha nenhuma naquela categoria e função. Por fim, grava os elencos.
func importarPessoasDaWikipedia(ctx context.Context, log *slog.Logger, esc *banco.Escritor, wp *wikipedia.Wikipedia,
	wd *wikidata.Wikidata, reg *registro, paginas []paginaGravada) error {
	log.Info("ligando elencos, técnicos e categorias da Wikipédia às pessoas")
	agora := time.Now()

	var aps []aparicao
	ultima := map[string]int{} // última temporada com página, por categoria
	for _, pg := range paginas {
		ultima[pg.Categoria] = max(ultima[pg.Categoria], pg.Temporada)
		for _, e := range pg.Elenco {
			if e.WikiEN != "" {
				aps = append(aps, aparicao{e.WikiEN, pg.Categoria, "jogador", pg.Temporada, pg.fonteID, false})
			}
		}
		// Quando a temporada teve troca, o quadro lista os técnicos em ordem: só o último ficou.
		for i, t := range pg.Tecnicos {
			aps = append(aps, aparicao{t.WikiEN, pg.Categoria, "tecnico", pg.Temporada, pg.fonteID, i < len(pg.Tecnicos)-1})
		}
	}
	for _, c := range categoriasDePessoas {
		membros, err := wp.MembrosDaCategoria(ctx, c.titulo)
		if err != nil {
			return err
		}
		fonteID, err := esc.Fonte(ctx, banco.Fonte{
			Nome: "Wikipédia (en): " + c.titulo, URL: wikipedia.URLArtigo(c.titulo),
			Licenca: wikipedia.Licenca, ColetadoEm: agora,
		})
		if err != nil {
			return err
		}
		for _, m := range membros {
			aps = append(aps, aparicao{m, c.categoria, c.funcao, 0, fonteID, false})
		}
	}

	novas, err := gravarPessoasNovas(ctx, esc, wp, wd, reg, aps)
	if err != nil {
		return err
	}
	passagens, err := gravarPassagensDerivadas(ctx, esc, reg, aps, ultima)
	if err != nil {
		return err
	}

	elenco, ligados := 0, 0
	for _, pg := range paginas {
		for _, e := range pg.Elenco {
			id := reg.porWiki[e.WikiEN]
			if err := esc.Elenco(ctx, e, id, pg.fonteID); err != nil {
				return err
			}
			elenco++
			if id > 0 {
				ligados++
			}
		}
	}
	log.Info("pessoas da Wikipédia", "aparicoes", len(aps), "pessoas_novas", novas,
		"passagens_novas", passagens, "elenco", elenco, "elenco_ligado_a_pessoa", ligados)
	return nil
}

// gravarPessoasNovas acha o QID de cada artigo ainda sem pessoa e grava as que faltam.
func gravarPessoasNovas(ctx context.Context, esc *banco.Escritor, wp *wikipedia.Wikipedia, wd *wikidata.Wikidata,
	reg *registro, aps []aparicao) (int, error) {
	var titulos []string
	vistos := map[string]bool{}
	for _, a := range aps {
		if _, ok := reg.porWiki[a.wiki]; !ok && !vistos[a.wiki] {
			vistos[a.wiki] = true
			titulos = append(titulos, a.wiki)
		}
	}
	sort.Strings(titulos)
	itens, err := wp.ItensWikidata(ctx, titulos)
	if err != nil {
		return 0, err
	}

	var qids []string
	pedidos := map[string]bool{}
	for _, t := range titulos {
		qid, ok := itens[t]
		if !ok {
			continue
		}
		if id, ok := reg.porQID[qid]; ok {
			reg.porWiki[t] = id // mesmo artigo com outro título (redirecionamento)
			continue
		}
		if !pedidos[qid] {
			pedidos[qid] = true
			qids = append(qids, qid)
		}
	}
	sort.Strings(qids)
	pessoas, err := wd.Pessoas(ctx, qids)
	if err != nil {
		return 0, err
	}
	for _, p := range pessoas {
		if err := reg.gravarPessoa(ctx, esc, p); err != nil {
			return 0, err
		}
	}
	for _, t := range titulos {
		if id, ok := reg.porQID[itens[t]]; ok {
			reg.porWiki[t] = id
		}
	}
	return len(pessoas), nil
}

// gravarPassagensDerivadas cria passagens para quem o Wikidata não tinha na
// categoria e função. O período vai da primeira à última temporada em que a
// pessoa aparece; se ela está na temporada mais recente (e não foi substituída
// nela), a passagem fica aberta.
func gravarPassagensDerivadas(ctx context.Context, esc *banco.Escritor, reg *registro, aps []aparicao, ultima map[string]int) (int, error) {
	type acumulado struct {
		pessoaID         int64
		categoria        string
		funcao           string
		primeira, ultima int
		fonteID          int64 // da aparição mais recente
		saiu             bool  // saiu durante a última temporada em que aparece
	}
	porChave := map[[3]string]*acumulado{}
	var ordem [][3]string
	for _, a := range aps {
		id, ok := reg.porWiki[a.wiki]
		if !ok {
			continue
		}
		k := chavePassagem(id, a.categoria, a.funcao)
		if reg.passagens[k] {
			continue
		}
		ac, ok := porChave[k]
		if !ok {
			ac = &acumulado{pessoaID: id, categoria: a.categoria, funcao: a.funcao, fonteID: a.fonteID}
			porChave[k] = ac
			ordem = append(ordem, k)
		}
		if a.temporada == 0 {
			continue
		}
		if ac.primeira == 0 || a.temporada < ac.primeira {
			ac.primeira = a.temporada
		}
		if a.temporada > ac.ultima {
			ac.ultima, ac.fonteID, ac.saiu = a.temporada, a.fonteID, a.saiu
		}
	}

	for _, k := range ordem {
		ac := porChave[k]
		pa := banco.Passagem{Categoria: ac.categoria, Funcao: ac.funcao}
		if ac.primeira > 0 {
			pa.Inicio = fmt.Sprint(ac.primeira)
			if ac.ultima < ultima[ac.categoria] || ac.saiu {
				pa.Fim = fmt.Sprint(ac.ultima)
			}
		}
		if err := reg.gravarPassagem(ctx, esc, ac.pessoaID, pa, ac.fonteID); err != nil {
			return 0, err
		}
	}
	return len(ordem), nil
}

// importarCompeticoesFemininas completa as partidas das Brabas com as páginas de
// edição das competições (Libertadores, Brasileirão, Paulista...).
func importarCompeticoesFemininas(ctx context.Context, log *slog.Logger, esc *banco.Escritor, wp *wikipedia.Wikipedia, detalhes bool) error {
	log.Info("lendo as páginas das competições femininas na Wikipédia")
	agora := time.Now()
	paginas, err := wp.CompeticoesFemininas(ctx)
	if err != nil {
		return err
	}
	var total uniao
	for _, pg := range paginas {
		fonteID, err := esc.Fonte(ctx, banco.Fonte{Nome: "Wikipédia (en): " + pg.Titulo, URL: pg.URL, Licenca: wikipedia.Licenca, ColetadoEm: agora})
		if err != nil {
			return err
		}
		u, err := complementar(ctx, esc, pg.Partidas, fonteID)
		if err != nil {
			return err
		}
		total.somar(u)
		if detalhes {
			for _, a := range pg.Avisos {
				fmt.Fprintf(os.Stderr, "    %s: %s\n", pg.Titulo, a)
			}
		}
	}
	total.registrar(log, "competições femininas", detalhes)
	return nil
}
