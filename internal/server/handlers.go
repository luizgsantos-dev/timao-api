package server

import (
	"errors"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"

	"timao-api/internal/brasilapi"
	"timao-api/internal/data"
)

// secao descreve como uma categoria aparece na API: prefixo das rotas e vocabulário.
type secao struct {
	categoria data.Categoria
	prefixo   string // "/v1" ou "/v1/feminino"
	idolos    string // "idolos" ou "idolas"
	aleatorio string // "aleatorio" ou "aleatoria"
	semIdolo  string // mensagem de 404 quando o slug não existe
	// Textos da documentação (OpenAPI).
	time              string // "time masculino", "time feminino (as Brabas)"
	idolosTexto       string // "ídolos", "ídolas"
	idoloSorteado     string
	idoloPorSlugTexto string

	// Exemplos de query string que aparecem na lista de rotas em GET /.
	escopoExemplo     string
	competicaoExemplo string
	nomeExemplo       string
}

var (
	secaoMasculino = secao{
		categoria: data.Masculino,
		prefixo:   "/v1",
		idolos:    "idolos",
		aleatorio: "aleatorio",
		semIdolo:  "Esse ídolo ainda não entrou no elenco da API.",
		time:      "time masculino",

		idolosTexto:       "ídolos",
		idoloSorteado:     "Um ídolo sorteado",
		idoloPorSlugTexto: "Um ídolo pelo slug",

		escopoExemplo:     "nacional",
		competicaoExemplo: "Campeonato Brasileiro",
		nomeExemplo:       "socrates",
	}
	secaoFeminino = secao{
		categoria: data.Feminino,
		prefixo:   "/v1/feminino",
		idolos:    "idolas",
		aleatorio: "aleatoria",
		semIdolo:  "Essa ídola ainda não entrou no elenco da API.",
		time:      "time feminino (as Brabas)",

		idolosTexto:       "ídolas",
		idoloSorteado:     "Uma ídola sorteada",
		idoloPorSlugTexto: "Uma ídola pelo slug",

		escopoExemplo:     "continental",
		competicaoExemplo: "Campeonato Brasileiro Feminino",
		nomeExemplo:       "tamires",
	}
)

func (s *Server) root(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"nome":   "timao-api",
		"versao": version,
		"bordao": s.repo.Clube(data.Masculino).Bordao,
		"rotas":  s.rotas,
	})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) clube(sec secao) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, s.repo.Clube(sec.categoria))
	}
}

func (s *Server) idolos(sec secao) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{sec.idolos: s.repo.Idolos(sec.categoria)})
	}
}

func (s *Server) idoloAleatorio(sec secao) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lista := s.repo.Idolos(sec.categoria)
		if len(lista) == 0 {
			writeError(w, http.StatusNotFound, "O elenco de ídolos ainda está vazio.", "")
			return
		}
		writeJSON(w, http.StatusOK, lista[rand.IntN(len(lista))])
	}
}

func (s *Server) idoloPorSlug(sec secao) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		i, ok := s.repo.IdoloPorSlug(sec.categoria, r.PathValue("slug"))
		if !ok {
			writeError(w, http.StatusNotFound, sec.semIdolo, "GET "+sec.prefixo+"/"+sec.idolos+" lista os slugs.")
			return
		}
		writeJSON(w, http.StatusOK, i)
	}
}

func (s *Server) historia(sec secao) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"marcos": s.repo.Historia(sec.categoria)})
	}
}

func (s *Server) estadio(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.repo.Estadio())
}

// estadioEndereco usa a Brasil API para completar o endereço a partir do CEP da arena.
func (s *Server) estadioEndereco(w http.ResponseWriter, r *http.Request) {
	est := s.repo.Estadio()
	end, err := s.brasil.CEP(r.Context(), est.CEP)
	if err != nil {
		s.brasilError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"estadio":  est.Nome,
		"endereco": end,
	})
}

type feriadoView struct {
	Data      string `json:"data"`
	Nome      string `json:"nome"`
	Tipo      string `json:"tipo"`
	DiaSemana string `json:"dia_semana"`
}

var diasSemana = [...]string{
	"domingo", "segunda-feira", "terça-feira", "quarta-feira",
	"quinta-feira", "sexta-feira", "sábado",
}

func diaSemana(date string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return ""
	}
	return diasSemana[t.Weekday()]
}

// feriados lista os feriados nacionais de um ano via Brasil API.
func (s *Server) feriados(w http.ResponseWriter, r *http.Request) {
	ano := s.now().In(s.loc).Year()
	if v := r.URL.Query().Get("ano"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Parâmetro ano inválido.", "Use algo como ?ano=2026")
			return
		}
		ano = n
	}

	lista, err := s.brasil.Feriados(r.Context(), ano)
	if err != nil {
		s.brasilError(w, err)
		return
	}

	out := make([]feriadoView, 0, len(lista))
	for _, f := range lista {
		out = append(out, feriadoView{
			Data:      f.Date,
			Nome:      f.Name,
			Tipo:      f.Type,
			DiaSemana: diaSemana(f.Date),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"ano": ano, "feriados": out})
}

func (s *Server) brasilError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, brasilapi.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, err.Error(), "")
	case errors.Is(err, brasilapi.ErrNotFound):
		writeError(w, http.StatusNotFound, "A Brasil API não encontrou esse recurso.", "")
	default:
		s.log.Error("falha ao consultar a Brasil API", "erro", err)
		writeError(w, http.StatusBadGateway, "A Brasil API deu pane. Tente de novo em instantes.", "")
	}
}
