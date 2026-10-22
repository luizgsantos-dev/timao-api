package server

import (
	"io"
	"log/slog"
	"net/http"
	"testing"

	"timao-api/internal/acervo/acervotest"
	"timao-api/internal/data"
)

func handlerComAcervo(t *testing.T) http.Handler {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(data.New(), fakeBrasil{}, fakeJogos{}, log).ComAcervo(acervotest.Novo(t)).Handler()
}

func TestAcervoIndisponivel(t *testing.T) {
	rec := get(t, novoHandler(fakeBrasil{}), "/v1/partidas")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("sem o banco, esperava 503, veio %d", rec.Code)
	}
	if decode(t, rec)["dica"] == nil {
		t.Fatal("o 503 deveria trazer uma dica de como gerar o banco")
	}
}

func TestRotasDoAcervo(t *testing.T) {
	h := handlerComAcervo(t)
	casos := []struct {
		path string
		want int
	}{
		{"/v1/pessoas?nome=socrates", http.StatusOK},
		{"/v1/pessoas?funcao=goleiro", http.StatusBadRequest},
		{"/v1/pessoas?limite=500", http.StatusBadRequest},
		{"/v1/pessoas/Q1", http.StatusOK},
		{"/v1/pessoas/Q2", http.StatusNotFound},
		{"/v1/feminino/pessoas/Q2", http.StatusOK},
		{"/v1/feminino/pessoas/Q2/gols", http.StatusOK},
		{"/v1/partidas?temporada=2025&resultado=vitoria", http.StatusOK},
		{"/v1/partidas?resultado=goleada", http.StatusBadRequest},
		{"/v1/partidas?de=ontem", http.StatusBadRequest},
		{"/v1/partidas/2025-03-01", http.StatusOK},
		{"/v1/partidas/01-03-2025", http.StatusBadRequest},
		{"/v1/feminino/partidas/2025-03-01", http.StatusNotFound},
		{"/v1/feminino/artilharia", http.StatusOK},
		{"/v1/temporadas", http.StatusOK},
		{"/v1/feminino/elenco", http.StatusOK},
		{"/v1/feminino/elenco?temporada=2024", http.StatusOK},
		{"/v1/feminino/elenco?temporada=1990", http.StatusNotFound},
		{"/v1/feminino/elenco?temporada=abc", http.StatusBadRequest},
		{"/v1/titulos", http.StatusOK},
		{"/v1/titulos?tipo=todos&escopo=estadual", http.StatusOK},
		{"/v1/titulos?tipo=amistosos", http.StatusBadRequest},
		{"/v1/titulos/Libertadores", http.StatusOK},
		{"/v1/feminino/titulos/mundial-de-clubes", http.StatusNotFound},
	}
	for _, c := range casos {
		if rec := get(t, h, c.path); rec.Code != c.want {
			t.Errorf("%s: status %d, esperava %d\n%s", c.path, rec.Code, c.want, rec.Body.String())
		}
	}
}

func TestPartidasComResumo(t *testing.T) {
	body := decode(t, get(t, handlerComAcervo(t), "/v1/partidas?temporada=2025"))
	if body["total"] != float64(2) {
		t.Fatalf("total: %v", body["total"])
	}
	resumo := body["resumo"].(map[string]any)
	if resumo["vitorias"] != float64(1) || resumo["empates"] != float64(1) {
		t.Fatalf("resumo: %v", resumo)
	}
}

func TestArtilhariaFeminina(t *testing.T) {
	lista, _ := decode(t, get(t, handlerComAcervo(t), "/v1/feminino/artilharia"))["artilharia"].([]any)
	if len(lista) == 0 || lista[0].(map[string]any)["nome"] != "Tamires" {
		t.Fatalf("artilharia: %v", lista)
	}
}

func TestStatusDoAcervo(t *testing.T) {
	body := decode(t, get(t, handlerComAcervo(t), "/v1/acervo"))
	if body["disponivel"] != true {
		t.Fatalf("com banco, deveria estar disponível: %v", body)
	}
	body = decode(t, get(t, novoHandler(fakeBrasil{}), "/v1/acervo"))
	if body["disponivel"] != false {
		t.Fatalf("sem banco, não deveria estar disponível: %v", body)
	}
}
