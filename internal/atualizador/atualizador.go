// Package atualizador mantém o timao.db em dia com a API no ar: de tempos em
// tempos (por padrão, toda semana) gera um banco novo, confere se ele não veio
// faltando dados e só então troca o banco que a API está usando.
//
// Cada atualização grava um arquivo novo ao lado do original, como
// timao-20261022T110200.db. Assim nada precisa ser sobrescrito enquanto está
// aberto (o Windows não deixa), e a API sempre sobe com o arquivo mais recente.
package atualizador

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"timao-api/internal/acervo"
	"timao-api/internal/banco"
)

// Importador gera um banco completo em destino.
type Importador func(ctx context.Context, destino string) error

const (
	// ToleranciaPadrao é a fração mínima de cada contagem (partidas, pessoas...)
	// que o banco novo precisa ter em relação ao atual para ser aceito.
	ToleranciaPadrao = 0.9
	// RepeticaoMaxima é o máximo que se espera para tentar de novo depois de uma falha.
	RepeticaoMaxima = 6 * time.Hour
	formatoGeracao  = "20060102T150405"
)

type Atualizador struct {
	Base       string        // caminho do banco principal, ex.: timao.db
	Periodo    time.Duration // de quanto em quanto tempo atualizar; 0 desliga
	Tolerancia float64
	Importar   Importador
	Acervo     *acervo.Acervo
	Log        *slog.Logger
	Agora      func() time.Time // trocável nos testes

	mu     sync.Mutex
	status Status
}

// Status descreve o banco em uso e a situação das atualizações.
type Status struct {
	Disponivel         bool       `json:"disponivel"`
	Arquivo            string     `json:"arquivo,omitempty"`
	GeradoEm           *time.Time `json:"gerado_em,omitempty"`
	AtualizacaoLigada  bool       `json:"atualizacao_automatica"`
	Periodo            string     `json:"periodo,omitempty"`
	ProximaAtualizacao *time.Time `json:"proxima_atualizacao,omitempty"`
	Atualizando        bool       `json:"atualizando"`
	UltimaTentativa    *time.Time `json:"ultima_tentativa,omitempty"`
	UltimoErro         string     `json:"ultimo_erro,omitempty"`
}

func Novo(base string, periodo time.Duration, imp Importador, ac *acervo.Acervo, log *slog.Logger) *Atualizador {
	return &Atualizador{
		Base: base, Periodo: periodo, Tolerancia: ToleranciaPadrao,
		Importar: imp, Acervo: ac, Log: log, Agora: time.Now,
	}
}

// Status devolve uma cópia da situação atual.
func (a *Atualizador) Status() Status {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := a.status
	s.Disponivel = a.Acervo.Disponivel()
	s.AtualizacaoLigada = a.Periodo > 0
	if a.Periodo > 0 {
		s.Periodo = a.Periodo.String()
	}
	return s
}

// Abrir carrega no acervo o banco mais recente entre o principal e as gerações.
// Sem nenhum banco válido, o acervo fica vazio (e a primeira atualização roda logo).
func (a *Atualizador) Abrir(ctx context.Context) error {
	arquivo, gerado, err := a.maisRecente(ctx)
	if err != nil {
		return err
	}
	if arquivo == "" {
		a.Log.Warn("nenhum banco encontrado; as rotas do acervo respondem 503 até a primeira importação", "base", a.Base)
		return nil
	}
	if err := a.Acervo.Trocar(arquivo); err != nil {
		return err
	}
	a.mu.Lock()
	a.status.Arquivo, a.status.GeradoEm = arquivo, &gerado
	a.mu.Unlock()
	a.Log.Info("acervo carregado", "arquivo", arquivo, "gerado_em", gerado.Format(time.RFC3339))
	return nil
}

// Rodar atualiza o banco a cada Periodo até ctx terminar. A primeira atualização
// acontece quando o banco atual completar Periodo de idade (na hora, se não há banco).
func (a *Atualizador) Rodar(ctx context.Context) {
	if a.Periodo <= 0 {
		return
	}
	proxima := a.Agora()
	if g := a.Status().GeradoEm; g != nil {
		proxima = g.Add(a.Periodo)
	}
	for {
		proxima = proxima.UTC()
		a.mu.Lock()
		a.status.ProximaAtualizacao = &proxima
		a.mu.Unlock()
		a.Log.Info("próxima atualização do acervo", "em", proxima.Format(time.RFC3339))

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(proxima)):
		}

		if err := a.Atualizar(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			proxima = a.Agora().Add(min(a.Periodo, RepeticaoMaxima))
			continue
		}
		proxima = a.Agora().Add(a.Periodo)
	}
}

// Atualizar gera um banco novo, confere e troca. Em caso de erro, o banco atual
// continua em uso e o arquivo novo é apagado.
func (a *Atualizador) Atualizar(ctx context.Context) error {
	inicio := a.Agora().UTC()
	a.mu.Lock()
	if a.status.Atualizando {
		a.mu.Unlock()
		return errors.New("atualizador: já há uma atualização em andamento")
	}
	a.status.Atualizando = true
	a.status.UltimaTentativa = &inicio
	a.mu.Unlock()

	err := a.atualizar(ctx, inicio)

	a.mu.Lock()
	a.status.Atualizando = false
	a.status.UltimoErro = ""
	if err != nil {
		a.status.UltimoErro = err.Error()
	}
	a.mu.Unlock()
	if err != nil {
		a.Log.Error("atualização do acervo falhou; o banco atual continua no ar", "erro", err)
	}
	return err
}

func (a *Atualizador) atualizar(ctx context.Context, inicio time.Time) error {
	destino := a.geracao(inicio)
	a.Log.Info("atualizando o acervo", "arquivo", destino)
	if err := a.Importar(ctx, destino); err != nil {
		os.Remove(destino)
		return fmt.Errorf("importação: %w", err)
	}
	if err := a.conferir(ctx, destino); err != nil {
		os.Remove(destino)
		return err
	}
	if err := a.Acervo.Trocar(destino); err != nil {
		os.Remove(destino)
		return fmt.Errorf("abrindo o banco novo: %w", err)
	}

	novo, err := banco.Abrir(destino)
	if err != nil {
		return err
	}
	gerado, err := banco.GeradoEm(ctx, novo)
	novo.Close()
	if err != nil {
		return err
	}
	a.mu.Lock()
	a.status.Arquivo, a.status.GeradoEm = destino, &gerado
	a.mu.Unlock()
	a.Log.Info("acervo atualizado", "arquivo", destino, "duracao", a.Agora().Sub(inicio).Round(time.Second))
	a.limpar(destino)
	return nil
}

// conferir recusa o banco novo se alguma contagem caiu abaixo da tolerância em
// relação ao banco em uso (uma fonte fora do ar ou uma página vandalizada).
func (a *Atualizador) conferir(ctx context.Context, destino string) error {
	novo, err := banco.Abrir(destino)
	if err != nil {
		return fmt.Errorf("conferindo o banco novo: %w", err)
	}
	defer novo.Close()
	depois, err := banco.Contagens(ctx, novo)
	if err != nil {
		return err
	}
	if !a.Acervo.Disponivel() {
		return nil // primeiro banco: não há com o que comparar
	}
	antes, err := a.Acervo.Contagens(ctx)
	if err != nil {
		return err
	}
	var quedas []string
	for k, n := range antes {
		if float64(depois[k]) < a.Tolerancia*float64(n) {
			quedas = append(quedas, fmt.Sprintf("%s: %d -> %d", k, n, depois[k]))
		}
	}
	if len(quedas) > 0 {
		sort.Strings(quedas)
		return fmt.Errorf("banco novo recusado, faltam dados em relação ao atual (%s)", strings.Join(quedas, "; "))
	}
	return nil
}

// geracao é o arquivo de uma atualização: timao.db -> timao-20261022T110200.db.
func (a *Atualizador) geracao(t time.Time) string {
	ext := filepath.Ext(a.Base)
	return strings.TrimSuffix(a.Base, ext) + "-" + t.UTC().Format(formatoGeracao) + ext
}

func (a *Atualizador) geracoes() []string {
	ext := filepath.Ext(a.Base)
	achados, _ := filepath.Glob(strings.TrimSuffix(a.Base, ext) + "-*" + ext)
	var out []string
	for _, f := range achados {
		meio := strings.TrimSuffix(strings.TrimPrefix(f, strings.TrimSuffix(a.Base, ext)+"-"), ext)
		if _, err := time.Parse(formatoGeracao, meio); err == nil {
			out = append(out, f)
		}
	}
	return out
}

// maisRecente escolhe, entre o banco principal e as gerações, o gerado por último.
// Arquivos que não abrem (corrompidos, de outra versão do esquema) são ignorados.
func (a *Atualizador) maisRecente(ctx context.Context) (string, time.Time, error) {
	var melhor string
	var quando time.Time
	for _, f := range append([]string{a.Base}, a.geracoes()...) {
		if _, err := os.Stat(f); err != nil {
			continue
		}
		db, err := banco.Abrir(f)
		if err != nil {
			a.Log.Warn("banco ignorado", "arquivo", f, "erro", err)
			continue
		}
		g, err := banco.GeradoEm(ctx, db)
		db.Close()
		if err != nil {
			continue
		}
		if melhor == "" || g.After(quando) {
			melhor, quando = f, g
		}
	}
	return melhor, quando, nil
}

// limpar apaga as gerações antigas. Um arquivo que ainda está aberto (no Windows,
// durante a espera para fechar o banco anterior) fica para a próxima limpeza.
// O banco principal nunca é apagado.
func (a *Atualizador) limpar(atual string) {
	for _, f := range a.geracoes() {
		if f == atual {
			continue
		}
		if err := os.Remove(f); err != nil {
			a.Log.Debug("geração antiga ainda em uso; fica para a próxima limpeza", "arquivo", f, "erro", err)
		}
	}
}
