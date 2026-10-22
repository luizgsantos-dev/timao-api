package brasilapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestCEPComCache(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/api/cep/v2/08295005" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"cep":"08295005","state":"SP","city":"São Paulo","neighborhood":"Itaquera","street":"Rua Teste"}`)
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL))

	got, err := c.CEP(context.Background(), "08295-005")
	if err != nil {
		t.Fatalf("CEP: %v", err)
	}
	if got.City != "São Paulo" || got.State != "SP" {
		t.Fatalf("resposta inesperada: %+v", got)
	}

	if _, err := c.CEP(context.Background(), "08295005"); err != nil {
		t.Fatalf("segunda chamada: %v", err)
	}
	if n := hits.Load(); n != 1 {
		t.Fatalf("esperava 1 requisição (segunda vem do cache), houve %d", n)
	}
}

func TestCEPNaoEncontrado(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	_, err := New(WithBaseURL(srv.URL)).CEP(context.Background(), "00000000")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("esperava ErrNotFound, veio %v", err)
	}
}

func TestCEPInvalido(t *testing.T) {
	_, err := New().CEP(context.Background(), "123")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("esperava ErrInvalidInput, veio %v", err)
	}
}

func TestFeriados(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/feriados/v1/2026" {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, `[{"date":"2026-09-07","name":"Independência do Brasil","type":"national"}]`)
	}))
	defer srv.Close()

	got, err := New(WithBaseURL(srv.URL)).Feriados(context.Background(), 2026)
	if err != nil {
		t.Fatalf("Feriados: %v", err)
	}
	if len(got) != 1 || got[0].Date != "2026-09-07" {
		t.Fatalf("resposta inesperada: %+v", got)
	}
}

func TestFeriadosAnoInvalido(t *testing.T) {
	_, err := New().Feriados(context.Background(), 1800)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("esperava ErrInvalidInput, veio %v", err)
	}
}

func TestChamadasSimultaneasCompartilhamARequisicao(t *testing.T) {
	var hits atomic.Int32
	liberar := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		<-liberar
		_, _ = io.WriteString(w, `[{"date":"2026-09-07","name":"Independência do Brasil","type":"national"}]`)
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL))
	const n = 10
	erros := make(chan error, n)
	for range n {
		go func() {
			_, err := c.Feriados(context.Background(), 2026)
			erros <- err
		}()
	}

	// Espera a primeira requisição chegar e dá tempo para as outras se enfileirarem.
	for hits.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	close(liberar)

	for range n {
		if err := <-erros; err != nil {
			t.Fatalf("Feriados: %v", err)
		}
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("esperava 1 requisição para %d chamadas simultâneas, houve %d", n, got)
	}
}

func TestFalhaFicaEmCachePorPoucoTempo(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Error(w, "fora do ar", http.StatusBadGateway)
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL), WithErrorTTL(time.Hour))
	for range 3 {
		if _, err := c.Feriados(context.Background(), 2026); err == nil {
			t.Fatal("esperava erro")
		}
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("esperava 1 requisição (falha em cache), houve %d", got)
	}

	semCache := New(WithBaseURL(srv.URL), WithErrorTTL(0))
	for range 2 {
		_, _ = semCache.Feriados(context.Background(), 2026)
	}
	if got := hits.Load(); got != 3 {
		t.Fatalf("sem cache de erro, esperava 3 requisições no total, houve %d", got)
	}
}

func TestRespostaQueNaoEJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "<html>manutenção</html>")
	}))
	defer srv.Close()

	if _, err := New(WithBaseURL(srv.URL)).Feriados(context.Background(), 2026); err == nil {
		t.Fatal("esperava erro para resposta que não é JSON")
	}
}
