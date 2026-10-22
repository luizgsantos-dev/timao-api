package server

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"timao-api/internal/data"
	"timao-api/internal/jogos"
)

const limitePadraoProximos = 5

// jogoView é o jogo do calendário acrescido do cruzamento com o calendário civil.
type jogoView struct {
	jogos.Jogo
	DiaSemana    string `json:"dia_semana"`
	FimDeSemana  bool   `json:"fim_de_semana"`
	CaiEmFeriado *bool  `json:"cai_em_feriado,omitempty"`
	Feriado      string `json:"feriado,omitempty"`
}

type respostaJogos struct {
	Jogos []jogoView `json:"jogos"`
	Aviso string     `json:"aviso,omitempty"`
}

// jogosDaCategoria devolve o handler de jogos de uma categoria.
// Com apenasFuturos, só entram jogos de hoje em diante.
func (s *Server) jogosDaCategoria(cat data.Categoria, apenasFuturos bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.listarJogos(w, r, cat, apenasFuturos)
	}
}

// listarJogos lê o calendário, filtra e cruza cada data com os feriados nacionais.
// Se a Brasil API falhar, os jogos saem mesmo assim, sem a informação de feriado.
func (s *Server) listarJogos(w http.ResponseWriter, r *http.Request, cat data.Categoria, apenasFuturos bool) {
	q := r.URL.Query()

	limite := 0
	if apenasFuturos {
		limite = limitePadraoProximos
	}
	if v := q.Get("limite"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "Parâmetro limite inválido.", "Use um número inteiro maior que zero, como ?limite=3")
			return
		}
		limite = n
	}

	todos, err := s.jogos.Jogos(r.Context())
	if err != nil {
		s.log.Error("falha ao ler o calendário de jogos", "erro", err)
		writeError(w, http.StatusInternalServerError, "Não consegui ler o calendário de jogos.", "")
		return
	}

	hoje := s.now().In(s.loc).Format("2006-01-02")
	competicao := q.Get("competicao")

	selecionados := make([]jogos.Jogo, 0, len(todos))
	for _, j := range todos {
		if !j.DaCategoria(string(cat)) {
			continue
		}
		if apenasFuturos && j.Data < hoje {
			continue
		}
		if competicao != "" && !strings.EqualFold(j.Competicao, competicao) {
			continue
		}
		selecionados = append(selecionados, j)
		if limite > 0 && len(selecionados) == limite {
			break
		}
	}

	feriados, ferr := s.feriadosPorData(r.Context(), selecionados)

	views := make([]jogoView, 0, len(selecionados))
	for _, j := range selecionados {
		v := jogoView{Jogo: j}
		if t, err := time.Parse("2006-01-02", j.Data); err == nil {
			v.DiaSemana = diasSemana[t.Weekday()]
			v.FimDeSemana = t.Weekday() == time.Saturday || t.Weekday() == time.Sunday
		}
		if ferr == nil {
			nome, ok := feriados[j.Data]
			v.CaiEmFeriado = &ok
			v.Feriado = nome
		}
		views = append(views, v)
	}

	resp := respostaJogos{Jogos: views}
	if ferr != nil {
		s.log.Warn("feriados indisponíveis ao listar jogos", "erro", ferr)
		resp.Aviso = "Feriados indisponíveis no momento (a Brasil API não respondeu). Os jogos estão sem a informação de feriado."
	}
	writeJSON(w, http.StatusOK, resp)
}

// feriadosPorData devolve um mapa data -> nome do feriado para todos os anos
// que aparecem na lista. O cliente da Brasil API guarda as respostas em cache.
func (s *Server) feriadosPorData(ctx context.Context, lista []jogos.Jogo) (map[string]string, error) {
	anos := make(map[int]struct{})
	for _, j := range lista {
		if t, err := time.Parse("2006-01-02", j.Data); err == nil {
			anos[t.Year()] = struct{}{}
		}
	}

	out := make(map[string]string)
	for ano := range anos {
		fs, err := s.brasil.Feriados(ctx, ano)
		if err != nil {
			return nil, err
		}
		for _, f := range fs {
			out[f.Date] = f.Name
		}
	}
	return out, nil
}
