// Package wikidata importa do Wikidata (CC0) as pessoas que passaram pelo
// Corinthians, como jogadores(as) e técnicos(as). Os títulos vêm da Wikipédia,
// que é mais completa (veja o pacote wikipedia).
package wikidata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"timao-api/internal/banco"
	"timao-api/internal/importar/coleta"
)

const (
	TimeMasculino = "Q35933"    // Sport Club Corinthians Paulista
	TimeFeminino  = "Q28681033" // Sport Club Corinthians Paulista (futebol feminino)

	generoFeminino  = "Q6581072"
	generoMasculino = "Q6581097"

	Endpoint = "https://query.wikidata.org/sparql"
	Licenca  = "CC0 1.0"

	tamanhoLote = 150 // pessoas por consulta de detalhes
)

type Wikidata struct {
	Endpoint string
	Coleta   *coleta.Cliente
}

func Novo(c *coleta.Cliente) *Wikidata {
	return &Wikidata{Endpoint: Endpoint, Coleta: c}
}

// Vinculo liga uma pessoa (pelo QID) a uma passagem pelo clube.
type Vinculo struct {
	Pessoa   string
	Passagem banco.Passagem
}

type Resultado struct {
	Pessoas  []banco.Pessoa
	Vinculos []Vinculo
}

// Importar busca jogadores(as), técnicos(as) e os detalhes de cada pessoa.
func (w *Wikidata) Importar(ctx context.Context) (*Resultado, error) {
	jogadores, err := w.jogadores(ctx)
	if err != nil {
		return nil, fmt.Errorf("wikidata: jogadores: %w", err)
	}
	tecnicos, err := w.tecnicos(ctx)
	if err != nil {
		return nil, fmt.Errorf("wikidata: técnicos: %w", err)
	}
	vinculos := append(jogadores, tecnicos...)

	qids := make([]string, 0, len(vinculos))
	vistos := map[string]bool{}
	for _, v := range vinculos {
		if !vistos[v.Pessoa] {
			vistos[v.Pessoa] = true
			qids = append(qids, v.Pessoa)
		}
	}
	sort.Strings(qids)

	pessoas, err := w.pessoas(ctx, qids)
	if err != nil {
		return nil, fmt.Errorf("wikidata: pessoas: %w", err)
	}

	return &Resultado{Pessoas: pessoas, Vinculos: vinculos}, nil
}

const consultaJogadores = `
SELECT ?st ?p ?time ?gen ?ini ?iniP ?fim ?fimP ?jogos ?gols WHERE {
  VALUES ?time { wd:Q35933 wd:Q28681033 }
  ?p p:P54 ?st . ?st ps:P54 ?time .
  FILTER NOT EXISTS { ?st wikibase:rank wikibase:DeprecatedRank }
  OPTIONAL { ?p wdt:P21 ?gen }
  OPTIONAL { ?st pqv:P580 [ wikibase:timeValue ?ini ; wikibase:timePrecision ?iniP ] }
  OPTIONAL { ?st pqv:P582 [ wikibase:timeValue ?fim ; wikibase:timePrecision ?fimP ] }
  OPTIONAL { ?st pq:P1350 ?jogos }
  OPTIONAL { ?st pq:P1351 ?gols }
}`

// jogadores lista as passagens como jogador(a). Jogadoras às vezes estão ligadas
// ao item do time masculino, então a categoria leva em conta o gênero da pessoa.
func (w *Wikidata) jogadores(ctx context.Context) ([]Vinculo, error) {
	linhas, err := w.consultar(ctx, consultaJogadores)
	if err != nil {
		return nil, err
	}
	var out []Vinculo
	vistos := map[string]bool{}
	for _, l := range linhas {
		if vistos[l["st"]] {
			continue // a mesma declaração repete quando a pessoa tem mais de um gênero cadastrado
		}
		vistos[l["st"]] = true

		cat := banco.Masculino
		if qid(l["time"]) == TimeFeminino || qid(l["gen"]) == generoFeminino {
			cat = banco.Feminino
		}
		out = append(out, Vinculo{
			Pessoa: qid(l["p"]),
			Passagem: banco.Passagem{
				Categoria: cat,
				Funcao:    "jogador",
				Inicio:    data(l["ini"], l["iniP"]),
				Fim:       data(l["fim"], l["fimP"]),
				Jogos:     inteiro(l["jogos"]),
				Gols:      inteiro(l["gols"]),
			},
		})
	}
	return out, nil
}

const consultaTecnicos = `
SELECT ?st ?p ?time ?ini ?iniP ?fim ?fimP WHERE {
  VALUES ?time { wd:Q35933 wd:Q28681033 }
  { ?time p:P286 ?st . ?st ps:P286 ?p . }
  UNION
  { ?p p:P6087 ?st . ?st ps:P6087 ?time . }
  FILTER NOT EXISTS { ?st wikibase:rank wikibase:DeprecatedRank }
  OPTIONAL { ?st pqv:P580 [ wikibase:timeValue ?ini ; wikibase:timePrecision ?iniP ] }
  OPTIONAL { ?st pqv:P582 [ wikibase:timeValue ?fim ; wikibase:timePrecision ?fimP ] }
}`

// tecnicos lista as passagens como técnico(a). O mesmo vínculo pode estar
// cadastrado no clube (P286) e na pessoa (P6087); as duplicatas são unidas.
func (w *Wikidata) tecnicos(ctx context.Context) ([]Vinculo, error) {
	linhas, err := w.consultar(ctx, consultaTecnicos)
	if err != nil {
		return nil, err
	}
	type chave struct{ pessoa, cat, inicio string }
	porChave := map[chave]Vinculo{}
	var ordem []chave
	for _, l := range linhas {
		cat := banco.Masculino
		if qid(l["time"]) == TimeFeminino {
			cat = banco.Feminino
		}
		v := Vinculo{
			Pessoa: qid(l["p"]),
			Passagem: banco.Passagem{
				Categoria: cat,
				Funcao:    "tecnico",
				Inicio:    data(l["ini"], l["iniP"]),
				Fim:       data(l["fim"], l["fimP"]),
			},
		}
		k := chave{v.Pessoa, cat, v.Passagem.Inicio}
		if atual, ok := porChave[k]; ok {
			if atual.Passagem.Fim == "" {
				porChave[k] = v
			}
			continue
		}
		porChave[k] = v
		ordem = append(ordem, k)
	}

	// Uma passagem sem data some se a mesma pessoa tem outra, datada, na mesma categoria.
	temDatada := map[[2]string]bool{}
	for _, k := range ordem {
		if k.inicio != "" {
			temDatada[[2]string{k.pessoa, k.cat}] = true
		}
	}
	var out []Vinculo
	for _, k := range ordem {
		if k.inicio == "" && temDatada[[2]string{k.pessoa, k.cat}] {
			continue
		}
		out = append(out, porChave[k])
	}
	return out, nil
}

const consultaPessoas = `
SELECT ?p ?pLabel ?gen ?nasc ?nascP ?morte ?morteP ?img ?ptwiki ?enwiki ?posLabel ?nacLabel WHERE {
  VALUES ?p { %s }
  OPTIONAL { ?p wdt:P21 ?gen }
  OPTIONAL { ?p p:P569/psv:P569 [ wikibase:timeValue ?nasc ; wikibase:timePrecision ?nascP ] }
  OPTIONAL { ?p p:P570/psv:P570 [ wikibase:timeValue ?morte ; wikibase:timePrecision ?morteP ] }
  OPTIONAL { ?p wdt:P18 ?img }
  OPTIONAL { ?a1 schema:about ?p ; schema:isPartOf <https://pt.wikipedia.org/> ; schema:name ?ptwiki }
  OPTIONAL { ?a2 schema:about ?p ; schema:isPartOf <https://en.wikipedia.org/> ; schema:name ?enwiki }
  OPTIONAL { ?p wdt:P413 ?pos }
  OPTIONAL { ?p wdt:P27 ?nac }
  SERVICE wikibase:label { bd:serviceParam wikibase:language "pt-br,pt,en". }
}`

// Pessoas busca os detalhes de QIDs que vieram de outra fonte (elencos e
// categorias da Wikipédia), no mesmo formato das pessoas importadas por Importar.
func (w *Wikidata) Pessoas(ctx context.Context, qids []string) ([]banco.Pessoa, error) {
	ps, err := w.pessoas(ctx, qids)
	if err != nil {
		return nil, fmt.Errorf("wikidata: pessoas: %w", err)
	}
	return ps, nil
}

// pessoas busca nome, nascimento, posição, nacionalidade, foto e artigos de cada QID.
func (w *Wikidata) pessoas(ctx context.Context, qids []string) ([]banco.Pessoa, error) {
	type acumulado struct {
		p     banco.Pessoa
		pos   []string
		nac   []string
		ordem int
	}
	porQID := map[string]*acumulado{}

	for i := 0; i < len(qids); i += tamanhoLote {
		lote := qids[i:min(i+tamanhoLote, len(qids))]
		valores := "wd:" + strings.Join(lote, " wd:")
		linhas, err := w.consultar(ctx, fmt.Sprintf(consultaPessoas, valores))
		if err != nil {
			return nil, err
		}
		for _, l := range linhas {
			id := qid(l["p"])
			a, ok := porQID[id]
			if !ok {
				a = &acumulado{p: banco.Pessoa{Wikidata: id, Nome: l["pLabel"]}, ordem: len(porQID)}
				porQID[id] = a
			}
			p := &a.p
			if p.Genero == "" {
				switch qid(l["gen"]) {
				case generoFeminino:
					p.Genero = banco.Feminino
				case generoMasculino:
					p.Genero = banco.Masculino
				}
			}
			preencher(&p.Nascimento, data(l["nasc"], l["nascP"]))
			preencher(&p.Falecimento, data(l["morte"], l["morteP"]))
			preencher(&p.FotoURL, strings.Replace(l["img"], "http://", "https://", 1))
			preencher(&p.WikiPT, l["ptwiki"])
			preencher(&p.WikiEN, l["enwiki"])
			a.pos = juntar(a.pos, rotulo(l["posLabel"]))
			a.nac = juntar(a.nac, rotulo(l["nacLabel"]))
		}
	}

	out := make([]banco.Pessoa, 0, len(porQID))
	for _, a := range porQID {
		a.p.Posicao = strings.Join(a.pos, ", ")
		a.p.Nacionalidade = strings.Join(a.nac, ", ")
		out = append(out, a.p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Wikidata < out[j].Wikidata })
	return out, nil
}

type respostaSPARQL struct {
	Results struct {
		Bindings []map[string]struct {
			Value string `json:"value"`
		} `json:"bindings"`
	} `json:"results"`
}

// consultar roda uma consulta SPARQL e devolve cada linha como variável -> valor.
func (w *Wikidata) consultar(ctx context.Context, consulta string) ([]map[string]string, error) {
	u := w.Endpoint + "?format=json&query=" + url.QueryEscape(consulta)
	body, err := w.Coleta.Get(ctx, u, "application/sparql-results+json")
	if err != nil {
		return nil, err
	}
	var r respostaSPARQL
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("resposta SPARQL inválida: %w", err)
	}
	out := make([]map[string]string, 0, len(r.Results.Bindings))
	for _, b := range r.Results.Bindings {
		linha := make(map[string]string, len(b))
		for k, v := range b {
			linha[k] = v.Value
		}
		out = append(out, linha)
	}
	return out, nil
}

// qid extrai o identificador de uma URI de entidade (http://www.wikidata.org/entity/Q1 -> Q1).
func qid(uri string) string {
	if i := strings.LastIndexByte(uri, '/'); i >= 0 {
		return uri[i+1:]
	}
	return uri
}

// data formata um valor de tempo do Wikidata conforme a precisão:
// 9 = ano, 10 = mês, 11 ou mais = dia.
func data(valor, precisao string) string {
	if len(valor) < 10 || strings.HasPrefix(valor, "-") {
		return ""
	}
	valor = strings.TrimPrefix(valor, "+")
	p, _ := strconv.Atoi(precisao)
	switch {
	case p >= 11:
		return valor[:10]
	case p == 10:
		return valor[:7]
	case p == 9:
		return valor[:4]
	default:
		return ""
	}
}

func inteiro(s string) *int {
	if s == "" {
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	n := int(f)
	return &n
}

// rotulo descarta rótulos que o serviço de rótulos devolve como QID quando falta tradução.
func rotulo(s string) string {
	if len(s) > 1 && s[0] == 'Q' {
		if _, err := strconv.Atoi(s[1:]); err == nil {
			return ""
		}
	}
	return s
}

func preencher(campo *string, valor string) {
	if *campo == "" {
		*campo = valor
	}
}

func juntar(lista []string, v string) []string {
	if v == "" {
		return lista
	}
	for _, x := range lista {
		if x == v {
			return lista
		}
	}
	return append(lista, v)
}
