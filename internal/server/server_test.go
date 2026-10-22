package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"timao-api/internal/brasilapi"
	"timao-api/internal/data"
)

type fakeBrasil struct {
	cep         *brasilapi.CEP
	cepErr      error
	feriados    []brasilapi.Feriado
	feriadosErr error
}

func (f fakeBrasil) CEP(ctx context.Context, cep string) (*brasilapi.CEP, error) {
	return f.cep, f.cepErr
}

func (f fakeBrasil) Feriados(ctx context.Context, ano int) ([]brasilapi.Feriado, error) {
	return f.feriados, f.feriadosErr
}

func novoHandler(b fakeBrasil) http.Handler {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(data.New(), b, fakeJogos{}, log).Handler()
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("JSON inválido: %v\n%s", err, rec.Body.String())
	}
	return m
}

func TestRaiz(t *testing.T) {
	rec := get(t, novoHandler(fakeBrasil{}), "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if got := decode(t, rec)["bordao"]; got != "Vai, Corinthians!" {
		t.Fatalf("bordão inesperado: %v", got)
	}
	if rec.Header().Get("X-Vai-Corinthians") != "sempre" {
		t.Fatal("faltou o header X-Vai-Corinthians")
	}
}

func TestRotaInexistente(t *testing.T) {
	rec := get(t, novoHandler(fakeBrasil{}), "/nao-existe")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d", rec.Code)
	}
	if decode(t, rec)["erro"] == nil {
		t.Fatal("esperava corpo JSON com a chave erro")
	}
}

func TestTitulosFiltrandoPorEscopo(t *testing.T) {
	rec := get(t, handlerComAcervo(t), "/v1/titulos?escopo=mundial")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body := decode(t, rec)
	if body["total"] != float64(2) {
		t.Fatalf("total esperado 2, veio %v", body["total"])
	}
	if lista, _ := body["titulos"].([]any); len(lista) != 1 {
		t.Fatalf("esperava 1 competição, veio %d", len(lista))
	}
}

func TestIdolos(t *testing.T) {
	h := novoHandler(fakeBrasil{})

	if rec := get(t, h, "/v1/idolos/aleatorio"); rec.Code != http.StatusOK {
		t.Fatalf("aleatório: status %d", rec.Code)
	}
	if rec := get(t, h, "/v1/idolos/socrates"); rec.Code != http.StatusOK {
		t.Fatalf("socrates: status %d", rec.Code)
	}
	if rec := get(t, h, "/v1/idolos/ninguem"); rec.Code != http.StatusNotFound {
		t.Fatalf("inexistente: status %d", rec.Code)
	}
}

func TestEstadioEndereco(t *testing.T) {
	casos := []struct {
		nome string
		fake fakeBrasil
		want int
	}{
		{"ok", fakeBrasil{cep: &brasilapi.CEP{City: "São Paulo", State: "SP"}}, http.StatusOK},
		{"cep não encontrado", fakeBrasil{cepErr: brasilapi.ErrNotFound}, http.StatusNotFound},
		{"brasil api fora do ar", fakeBrasil{cepErr: errors.New("timeout")}, http.StatusBadGateway},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			rec := get(t, novoHandler(c.fake), "/v1/estadio/endereco")
			if rec.Code != c.want {
				t.Fatalf("status %d, esperava %d", rec.Code, c.want)
			}
		})
	}
}

func TestFeriados(t *testing.T) {
	fake := fakeBrasil{feriados: []brasilapi.Feriado{
		{Date: "2026-09-07", Name: "Independência do Brasil", Type: "national"},
	}}
	h := novoHandler(fake)

	rec := get(t, h, "/v1/feriados?ano=2026")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	lista, _ := decode(t, rec)["feriados"].([]any)
	if len(lista) != 1 {
		t.Fatalf("esperava 1 feriado, veio %d", len(lista))
	}
	if dia := lista[0].(map[string]any)["dia_semana"]; dia != "segunda-feira" {
		t.Fatalf("2026-09-07 é segunda-feira, veio %v", dia)
	}

	if rec := get(t, h, "/v1/feriados?ano=abc"); rec.Code != http.StatusBadRequest {
		t.Fatalf("ano inválido: status %d", rec.Code)
	}

	invalido := novoHandler(fakeBrasil{feriadosErr: brasilapi.ErrInvalidInput})
	if rec := get(t, invalido, "/v1/feriados?ano=1800"); rec.Code != http.StatusBadRequest {
		t.Fatalf("ano fora do intervalo: status %d", rec.Code)
	}
}
