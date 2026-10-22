package coleta

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRepeteEmFalhaPassageira(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != UserAgent {
			t.Errorf("User-Agent ausente: %q", r.Header.Get("User-Agent"))
		}
		if n.Add(1) < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := Novo()
	c.Espera = time.Millisecond
	body, err := c.Get(context.Background(), srv.URL, "")
	if err != nil || string(body) != "ok" {
		t.Fatalf("Get: %q %v", body, err)
	}
	if n.Load() != 3 {
		t.Fatalf("esperava 3 tentativas, houve %d", n.Load())
	}
}

func TestNaoRepeteErroDoCliente(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	c := Novo()
	c.Espera = time.Millisecond
	if _, err := c.Get(context.Background(), srv.URL, ""); err == nil || errors.Is(err, ErrNaoEncontrado) {
		t.Fatalf("esperava erro que não fosse de página inexistente, veio %v", err)
	}
	if n.Load() != 1 {
		t.Fatalf("erro 400 não deveria ser repetido, houve %d tentativas", n.Load())
	}
}

func TestPaginaInexistente(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()
	if _, err := Novo().Get(context.Background(), srv.URL, ""); !errors.Is(err, ErrNaoEncontrado) {
		t.Fatalf("404 deveria ser ErrNaoEncontrado, veio %v", err)
	}
}
