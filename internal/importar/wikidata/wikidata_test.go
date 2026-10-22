package wikidata

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"timao-api/internal/banco"
	"timao-api/internal/importar/coleta"
)

const ent = "http://www.wikidata.org/entity/"

// linhas monta uma resposta SPARQL a partir de mapas variável -> valor.
func linhas(ls ...map[string]string) []byte {
	var r struct {
		Results struct {
			Bindings []map[string]map[string]string `json:"bindings"`
		} `json:"results"`
	}
	for _, l := range ls {
		b := map[string]map[string]string{}
		for k, v := range l {
			b[k] = map[string]string{"value": v}
		}
		r.Results.Bindings = append(r.Results.Bindings, b)
	}
	out, _ := json.Marshal(r)
	return out
}

// fakeSPARQL responde conforme a consulta recebida.
func fakeSPARQL(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("query")
		var body []byte
		switch {
		case strings.Contains(q, "p:P54 ?st"):
			body = linhas(
				// Jogador do masculino, com datas de precisão diferente.
				map[string]string{"st": "s1", "p": ent + "Q1", "time": ent + TimeMasculino, "gen": ent + generoMasculino,
					"ini": "2009-01-01T00:00:00Z", "iniP": "9", "fim": "2011-02-14T00:00:00Z", "fimP": "11", "jogos": "69", "gols": "35"},
				// Jogadora ligada por engano ao item do masculino.
				map[string]string{"st": "s2", "p": ent + "Q2", "time": ent + TimeMasculino, "gen": ent + generoFeminino},
				// A mesma declaração repetida (pessoa com dois gêneros cadastrados).
				map[string]string{"st": "s2", "p": ent + "Q2", "time": ent + TimeMasculino, "gen": ent + "Q999"},
				map[string]string{"st": "s3", "p": ent + "Q3", "time": ent + TimeFeminino},
			)
		case strings.Contains(q, "P286"):
			body = linhas(
				map[string]string{"st": "c1", "p": ent + "Q4", "time": ent + TimeMasculino, "ini": "2011-01-01T00:00:00Z", "iniP": "9"},
				// O mesmo vínculo cadastrado na pessoa, agora com fim.
				map[string]string{"st": "c2", "p": ent + "Q4", "time": ent + TimeMasculino, "ini": "2011-01-01T00:00:00Z", "iniP": "9",
					"fim": "2013-01-01T00:00:00Z", "fimP": "9"},
				// Sem datas: some, porque há outro vínculo datado.
				map[string]string{"st": "c3", "p": ent + "Q4", "time": ent + TimeMasculino},
			)
		case strings.Contains(q, "VALUES ?p"):
			body = linhas(
				map[string]string{"p": ent + "Q1", "pLabel": "Ronaldo", "gen": ent + generoMasculino,
					"nasc": "1976-09-18T00:00:00Z", "nascP": "11", "posLabel": "atacante", "nacLabel": "Brasil",
					"img": "http://commons.wikimedia.org/wiki/Special:FilePath/R.jpg", "enwiki": "Ronaldo (Brazilian footballer)"},
				map[string]string{"p": ent + "Q1", "pLabel": "Ronaldo", "posLabel": "atacante", "nacLabel": "Espanha"},
				map[string]string{"p": ent + "Q2", "pLabel": "Gabi", "gen": ent + generoFeminino, "posLabel": "Q123456"},
				map[string]string{"p": ent + "Q3", "pLabel": "Tamires"},
				map[string]string{"p": ent + "Q4", "pLabel": "Tite"},
			)
		default:
			t.Errorf("consulta inesperada: %s", q)
		}
		w.Header().Set("Content-Type", "application/sparql-results+json")
		_, _ = w.Write(body)
	}))
}

func TestImportar(t *testing.T) {
	srv := fakeSPARQL(t)
	defer srv.Close()

	wd := Novo(coleta.Novo())
	wd.Endpoint = srv.URL
	r, err := wd.Importar(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(r.Vinculos) != 4 {
		t.Fatalf("esperava 4 vínculos (3 jogadores + 1 técnico), vieram %d: %+v", len(r.Vinculos), r.Vinculos)
	}
	ronaldo := r.Vinculos[0].Passagem
	if ronaldo.Categoria != banco.Masculino || ronaldo.Inicio != "2009" || ronaldo.Fim != "2011-02-14" ||
		ronaldo.Jogos == nil || *ronaldo.Jogos != 69 || *ronaldo.Gols != 35 {
		t.Errorf("passagem do Ronaldo errada: %+v", ronaldo)
	}
	if c := r.Vinculos[1].Passagem.Categoria; c != banco.Feminino {
		t.Errorf("jogadora ligada ao item do masculino deveria ser feminino, veio %s", c)
	}
	if c := r.Vinculos[2].Passagem.Categoria; c != banco.Feminino {
		t.Errorf("jogadora do item feminino deveria ser feminino, veio %s", c)
	}
	tite := r.Vinculos[3].Passagem
	if tite.Funcao != "tecnico" || tite.Inicio != "2011" || tite.Fim != "2013" {
		t.Errorf("técnico deveria ser unido numa passagem com início e fim: %+v", tite)
	}

	if len(r.Pessoas) != 4 {
		t.Fatalf("esperava 4 pessoas, vieram %d", len(r.Pessoas))
	}
	p := r.Pessoas[0]
	if p.Nome != "Ronaldo" || p.Nascimento != "1976-09-18" || p.Nacionalidade != "Brasil, Espanha" ||
		p.Posicao != "atacante" || !strings.HasPrefix(p.FotoURL, "https://") || p.WikiEN == "" {
		t.Errorf("pessoa errada: %+v", p)
	}
	if r.Pessoas[1].Posicao != "" {
		t.Errorf("rótulo sem tradução (QID) deveria ser descartado: %q", r.Pessoas[1].Posicao)
	}
}

func TestData(t *testing.T) {
	casos := []struct{ valor, prec, want string }{
		{"1910-09-01T00:00:00Z", "11", "1910-09-01"},
		{"1910-09-01T00:00:00Z", "10", "1910-09"},
		{"1910-01-01T00:00:00Z", "9", "1910"},
		{"1900-01-01T00:00:00Z", "7", ""},
		{"", "", ""},
	}
	for _, c := range casos {
		if got := data(c.valor, c.prec); got != c.want {
			t.Errorf("data(%q, %q) = %q, esperava %q", c.valor, c.prec, got, c.want)
		}
	}
}
