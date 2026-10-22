// Package coleta faz as requisições HTTP dos importadores, com User-Agent
// identificado (exigido pela Wikimedia) e novas tentativas em falhas passageiras.
package coleta

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// ErrNaoEncontrado indica que o servidor respondeu 404 ou 410 (a página não existe).
var ErrNaoEncontrado = errors.New("coleta: página não encontrada")

const UserAgent = "timao-api-importador/0.1 (projeto aberto sobre o Corinthians; importacao semanal)"

type Cliente struct {
	HTTP       *http.Client
	Tentativas int
	Espera     time.Duration // espera base entre tentativas, dobrada a cada falha
}

func Novo() *Cliente {
	return &Cliente{
		HTTP:       &http.Client{Timeout: 90 * time.Second},
		Tentativas: 4,
		Espera:     2 * time.Second,
	}
}

// Get busca url e devolve o corpo. Repete em erro de rede, 429 e 5xx,
// respeitando o header Retry-After quando ele vem.
func (c *Cliente) Get(ctx context.Context, url, accept string) ([]byte, error) {
	espera := c.Espera
	var ultimo error
	for tentativa := 1; tentativa <= c.Tentativas; tentativa++ {
		body, repetir, depois, err := c.uma(ctx, url, accept)
		if err == nil {
			return body, nil
		}
		ultimo = err
		if !repetir || tentativa == c.Tentativas {
			break
		}
		if depois > 0 {
			espera = depois
		}
		select {
		case <-time.After(espera):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		espera *= 2
	}
	return nil, ultimo
}

func (c *Cliente) uma(ctx context.Context, url, accept string) (body []byte, repetir bool, depois time.Duration, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, 0, err
	}
	req.Header.Set("User-Agent", UserAgent)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, ctx.Err() == nil, 0, fmt.Errorf("coleta: %w", err)
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, true, 0, fmt.Errorf("coleta: lendo %s: %w", url, err)
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil {
			depois = time.Duration(s) * time.Second
		}
		return nil, true, depois, fmt.Errorf("coleta: %s respondeu %d", url, resp.StatusCode)
	}
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		return nil, false, 0, fmt.Errorf("%w: %s respondeu %d", ErrNaoEncontrado, url, resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return nil, false, 0, fmt.Errorf("coleta: %s respondeu %d", url, resp.StatusCode)
	}
	return body, false, 0, nil
}
