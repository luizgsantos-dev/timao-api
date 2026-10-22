// Package brasilapi é um cliente enxuto para a Brasil API (https://brasilapi.com.br).
// Só cobre os endpoints que o projeto usa e guarda as respostas em cache na memória
// para não sobrecarregar um serviço gratuito. Falhas também ficam em cache por pouco
// tempo, e chamadas simultâneas ao mesmo recurso compartilham uma única requisição.
package brasilapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound     = errors.New("brasilapi: recurso não encontrado")
	ErrInvalidInput = errors.New("brasilapi: entrada inválida")
)

type CEP struct {
	CEP          string          `json:"cep"`
	State        string          `json:"state"`
	City         string          `json:"city"`
	Neighborhood string          `json:"neighborhood"`
	Street       string          `json:"street"`
	Location     json.RawMessage `json:"location,omitempty"`
}

type Feriado struct {
	Date string `json:"date"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type cacheEntry struct {
	body    []byte
	err     error
	expires time.Time
}

// chamada é uma requisição em andamento, compartilhada por quem pedir o mesmo path.
type chamada struct {
	done chan struct{}
	body []byte
	err  error
}

type Client struct {
	baseURL string
	http    *http.Client
	ttl     time.Duration
	errTTL  time.Duration

	mu       sync.Mutex
	cache    map[string]cacheEntry
	inflight map[string]*chamada
}

type Option func(*Client)

func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") }
}

func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.http = h }
}

// WithTTL define por quanto tempo as respostas ficam em cache. Zero desliga o cache.
func WithTTL(d time.Duration) Option {
	return func(c *Client) { c.ttl = d }
}

// WithErrorTTL define por quanto tempo uma falha (404, erro 5xx, rede) fica em cache,
// para não repetir a chamada a cada requisição enquanto a Brasil API está fora. Zero desliga.
func WithErrorTTL(d time.Duration) Option {
	return func(c *Client) { c.errTTL = d }
}

func New(opts ...Option) *Client {
	c := &Client{
		baseURL:  "https://brasilapi.com.br",
		http:     &http.Client{Timeout: 8 * time.Second},
		ttl:      time.Hour,
		errTTL:   30 * time.Second,
		cache:    make(map[string]cacheEntry),
		inflight: make(map[string]*chamada),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// CEP consulta um CEP. Aceita com ou sem hífen.
func (c *Client) CEP(ctx context.Context, cep string) (*CEP, error) {
	digits := onlyDigits(cep)
	if len(digits) != 8 {
		return nil, fmt.Errorf("%w: CEP precisa ter 8 dígitos", ErrInvalidInput)
	}
	var out CEP
	if err := c.getJSON(ctx, "/api/cep/v2/"+digits, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Feriados lista os feriados nacionais de um ano (a Brasil API cobre de 1900 a 2199).
func (c *Client) Feriados(ctx context.Context, ano int) ([]Feriado, error) {
	if ano < 1900 || ano > 2199 {
		return nil, fmt.Errorf("%w: ano precisa estar entre 1900 e 2199", ErrInvalidInput)
	}
	var out []Feriado
	if err := c.getJSON(ctx, "/api/feriados/v1/"+strconv.Itoa(ano), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	body, err := c.buscar(ctx, path)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

// buscar devolve o corpo de path, do cache quando possível. Chamadas simultâneas
// para o mesmo path esperam a mesma requisição em vez de abrir outras.
func (c *Client) buscar(ctx context.Context, path string) ([]byte, error) {
	for {
		c.mu.Lock()
		if e, ok := c.cache[path]; ok {
			if time.Now().Before(e.expires) {
				c.mu.Unlock()
				return e.body, e.err
			}
			delete(c.cache, path)
		}

		if ch, ok := c.inflight[path]; ok {
			c.mu.Unlock()
			select {
			case <-ch.done:
			case <-ctx.Done():
				return nil, fmt.Errorf("brasilapi: %w", ctx.Err())
			}
			// Se a requisição compartilhada foi cancelada pelo contexto de quem a abriu,
			// tenta de novo com o próprio contexto.
			if isCtxErr(ch.err) && ctx.Err() == nil {
				continue
			}
			return ch.body, ch.err
		}

		ch := &chamada{done: make(chan struct{})}
		c.inflight[path] = ch
		c.mu.Unlock()

		ch.body, ch.err = c.requisitar(ctx, path)

		c.mu.Lock()
		delete(c.inflight, path)
		c.guardar(path, ch.body, ch.err)
		c.mu.Unlock()
		close(ch.done)

		return ch.body, ch.err
	}
}

// guardar coloca a resposta no cache. Precisa ser chamada com c.mu travado.
func (c *Client) guardar(path string, body []byte, err error) {
	ttl := c.ttl
	if err != nil {
		if isCtxErr(err) {
			return // cancelamento é do cliente, não diz nada sobre a Brasil API
		}
		ttl = c.errTTL
	}
	if ttl <= 0 {
		return
	}
	c.cache[path] = cacheEntry{body: body, err: err, expires: time.Now().Add(ttl)}
}

// requisitar faz a chamada HTTP e devolve o corpo, já conferido como JSON válido.
func (c *Client) requisitar(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "timao-api/0.1")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("brasilapi: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("brasilapi: lendo resposta: %w", err)
	}

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, ErrNotFound
	case resp.StatusCode >= 400:
		return nil, fmt.Errorf("brasilapi: status %d", resp.StatusCode)
	}

	if !json.Valid(body) {
		return nil, errors.New("brasilapi: resposta inválida: não é JSON")
	}
	return body, nil
}

func isCtxErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
