package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"timao-api/internal/brasilapi"
	"timao-api/internal/data"
)

func TestMetodoNaoPermitido(t *testing.T) {
	h := novoHandler(fakeBrasil{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/clube", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status %d, esperava 405", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
		t.Fatalf("header Allow deveria listar GET, veio %q", allow)
	}
	if decode(t, rec)["erro"] == nil {
		t.Fatal("esperava corpo JSON com a chave erro")
	}
}

func TestRotasListadasNaRaiz(t *testing.T) {
	lista, _ := decode(t, get(t, novoHandler(fakeBrasil{}), "/"))["rotas"].([]any)
	if len(lista) != 41 {
		t.Fatalf("esperava 41 rotas, veio %d: %v", len(lista), lista)
	}
	vistas := map[string]bool{}
	for _, r := range lista {
		vistas[r.(string)] = true
	}
	for _, want := range []string{
		"GET /healthz",
		"GET /v1/titulos?escopo=nacional",
		"GET /v1/feminino/idolas/aleatoria",
		"GET /v1/feminino/jogos?competicao=Campeonato Brasileiro Feminino",
		"GET /v1/feriados?ano=2026",
	} {
		if !vistas[want] {
			t.Errorf("faltou a rota %q na lista", want)
		}
	}
}

func TestSlugSemDiferenciarMaiusculas(t *testing.T) {
	h := novoHandler(fakeBrasil{})
	for _, path := range []string{"/v1/idolos/SOCRATES", "/v1/feminino/idolas/Gabi-Zanotti"} {
		if rec := get(t, h, path); rec.Code != http.StatusOK {
			t.Errorf("%s: status %d", path, rec.Code)
		}
	}
}

// brasilAnos guarda os anos pedidos em Feriados.
type brasilAnos struct {
	fakeBrasil
	anos *[]int
}

func (b brasilAnos) Feriados(ctx context.Context, ano int) ([]brasilapi.Feriado, error) {
	*b.anos = append(*b.anos, ano)
	return nil, nil
}

func TestFeriadosAnoPadraoUsaORelogioDoServidor(t *testing.T) {
	var anos []int
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := New(data.New(), brasilAnos{anos: &anos}, fakeJogos{}, log)
	// 31/12/2026 às 23h em Brasília já é 2027 em UTC.
	s.now = func() time.Time { return time.Date(2027, 1, 1, 2, 0, 0, 0, time.UTC) }

	if rec := get(t, s.Handler(), "/v1/feriados"); rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if len(anos) != 1 || anos[0] != 2026 {
		t.Fatalf("esperava o ano 2026 (horário de Brasília), veio %v", anos)
	}
}

func TestRecovererAntesDeEscrever(t *testing.T) {
	s := novoServidor(fakeBrasil{}, fakeJogos{})
	h := s.recoverer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("bola murcha")
	}))
	rec := get(t, h, "/")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status %d, esperava 500", rec.Code)
	}
	if decode(t, rec)["erro"] == nil {
		t.Fatal("esperava corpo JSON com a chave erro")
	}
}

func TestRecovererDepoisDeEscrever(t *testing.T) {
	s := novoServidor(fakeBrasil{}, fakeJogos{})
	h := s.recoverer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "parcial")
		panic("bola murcha")
	}))
	rec := get(t, h, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, deveria manter o 200 já enviado", rec.Code)
	}
	if rec.Body.String() != "parcial" {
		t.Fatalf("o corpo não deveria ganhar o JSON de erro: %q", rec.Body.String())
	}
}
