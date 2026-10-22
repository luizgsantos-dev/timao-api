// Package server monta as rotas HTTP da timao-api.
package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"timao-api/internal/acervo"
	"timao-api/internal/atualizador"
	"timao-api/internal/brasilapi"
	"timao-api/internal/data"
	"timao-api/internal/jogos"
)

const version = "0.1.0"

// BrasilAPI é o que o servidor precisa da Brasil API.
// Ser uma interface permite trocar por um fake nos testes.
type BrasilAPI interface {
	CEP(ctx context.Context, cep string) (*brasilapi.CEP, error)
	Feriados(ctx context.Context, ano int) ([]brasilapi.Feriado, error)
}

// StatusAcervo informa o banco em uso e a situação das atualizações automáticas.
type StatusAcervo interface {
	Status() atualizador.Status
}

// JogosProvider fornece o calendário de jogos.
// Hoje vem de um arquivo JSON; amanhã pode vir de uma API esportiva.
type JogosProvider interface {
	Jogos(ctx context.Context) ([]jogos.Jogo, error)
}

type Server struct {
	repo   *data.Repo
	brasil BrasilAPI
	jogos  JogosProvider
	acervo *acervo.Acervo // nil ou vazio quando a API sobe sem o timao.db
	status StatusAcervo   // situação das atualizações do acervo; pode ser nil
	log    *slog.Logger

	now func() time.Time // trocável nos testes
	loc *time.Location   // fuso usado para decidir o que é "hoje"

	rotas []string  // preenchida por Handler e exibida em GET /
	docs  []rotaDoc // preenchida por Handler; vira o OpenAPI
}

func New(repo *data.Repo, brasil BrasilAPI, calendario JogosProvider, log *slog.Logger) *Server {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		loc = time.FixedZone("BRT", -3*60*60)
	}
	return &Server{
		repo:   repo,
		brasil: brasil,
		jogos:  calendario,
		log:    log,
		now:    time.Now,
		loc:    loc,
	}
}

// Handler devolve o http.Handler com rotas e middlewares.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	s.rotas, s.docs = nil, nil

	mux.HandleFunc("GET /{$}", s.root)
	s.documentar("GET /{$}", doc{resumo: "Nome, versão, bordão e a lista de rotas", resposta: ref("Raiz")})
	s.handle(mux, "GET /healthz", "", doc{resumo: "Health check", resposta: ref("Saude")}, s.health)
	s.handle(mux, "GET /openapi.json", "", doc{resumo: "Este documento OpenAPI",
		resposta: schema{"type": "object", "description": "Documento OpenAPI 3.1."}}, s.openAPIJSON)
	s.handle(mux, "GET /docs", "", doc{resumo: "Documentação navegável (Swagger UI)",
		resposta: schema{"type": "string", "description": "Página HTML."}}, s.paginaDocs)

	s.registrarTime(mux, secaoMasculino)
	s.registrarTime(mux, secaoFeminino)

	s.handle(mux, "GET /v1/acervo", "", doc{
		resumo:    "Situação do banco de dados",
		descricao: "Arquivo em uso, quando foi gerado, quando será atualizado e o erro da última tentativa, se houve.",
		resposta:  ref("StatusAcervo"),
	}, s.statusAcervo)
	s.handle(mux, "GET /v1/estadio", "", doc{resumo: "Dados da Neo Química Arena", resposta: ref("Estadio")}, s.estadio)
	s.handle(mux, "GET /v1/estadio/endereco", "", doc{
		resumo: "Endereço da arena, via CEP na Brasil API", resposta: ref("Endereco"),
		erros: []int{http.StatusNotFound, http.StatusBadGateway},
	}, s.estadioEndereco)
	s.handle(mux, "GET /v1/feriados", "?ano=2026", doc{
		resumo:   "Feriados nacionais do ano, via Brasil API",
		params:   []schema{q("ano", "Ano entre 1900 e 2199; padrão: o ano corrente em Brasília.", schema{"type": "integer", "example": 2026})},
		resposta: ref("Feriados"),
		erros:    []int{http.StatusBadRequest, http.StatusBadGateway},
	}, s.feriados)

	return chain(errosEmJSON(mux), vaiCorinthians, s.logging, s.recoverer)
}

// handle registra a rota no mux, na lista exibida em GET / e no OpenAPI.
// exemplo é uma query string ilustrativa, que só aparece na lista.
func (s *Server) handle(mux *http.ServeMux, pattern, exemplo string, d doc, h http.HandlerFunc) {
	mux.HandleFunc(pattern, h)
	s.rotas = append(s.rotas, pattern+exemplo)
	s.documentar(pattern, d)
}

// registrarTime registra as rotas de uma categoria: clube, títulos, ídolos,
// história e jogos. O masculino fica em /v1 e o feminino em /v1/feminino.
func (s *Server) registrarTime(mux *http.ServeMux, sec secao) {
	p := sec.prefixo
	pCompeticao := q("competicao", "Nome exato da competição, sem diferenciar maiúsculas.", schema{"type": "string", "example": sec.competicaoExemplo})

	s.handle(mux, "GET "+p+"/clube", "", doc{resumo: "Dados gerais do " + sec.time, resposta: ref("Clube")}, s.clube(sec))
	s.handle(mux, "GET "+p+"/titulos", "?escopo="+sec.escopoExemplo, doc{
		resumo:    "Títulos por competição",
		descricao: "Da seção Honours da Wikipédia em inglês. Por padrão, só os oficiais.",
		params: []schema{
			q("escopo", "Filtra pelo escopo.", strEnum("", "mundial", "continental", "nacional", "interestadual", "estadual", "internacional", "amistoso", "outros")),
			q("tipo", "oficial (padrão), outros (torneios não oficiais) ou todos.", strEnum("", "oficial", "outros", "todos")),
		},
		resposta: ref("Titulos"),
		erros:    []int{http.StatusBadRequest, http.StatusServiceUnavailable},
	}, s.titulos(sec))
	s.handle(mux, "GET "+p+"/titulos/{slug}", "", doc{
		resumo: "Uma competição, com cada edição vencida", descricao: "Ex.: libertadores, brasileirao, paulista.",
		resposta: ref("Conquista"), erros: []int{http.StatusNotFound, http.StatusServiceUnavailable},
	}, s.tituloPorSlug(sec))
	s.handle(mux, "GET "+p+"/"+sec.idolos, "", doc{resumo: "Lista de " + sec.idolosTexto + " (escrita à mão)",
		resposta: obj("", schema{sec.idolos: arr(ref("Idolo"))}, sec.idolos)}, s.idolos(sec))
	s.handle(mux, "GET "+p+"/"+sec.idolos+"/"+sec.aleatorio, "", doc{resumo: sec.idoloSorteado,
		resposta: ref("Idolo"), erros: []int{http.StatusNotFound}}, s.idoloAleatorio(sec))
	s.handle(mux, "GET "+p+"/"+sec.idolos+"/{slug}", "", doc{resumo: sec.idoloPorSlugTexto,
		resposta: ref("Idolo"), erros: []int{http.StatusNotFound}}, s.idoloPorSlug(sec))
	s.handle(mux, "GET "+p+"/historia", "", doc{resumo: "Marcos da história, em ordem cronológica",
		resposta: obj("", schema{"marcos": arr(ref("Marco"))}, "marcos")}, s.historia(sec))
	s.handle(mux, "GET "+p+"/jogos", "?competicao="+sec.competicaoExemplo, doc{
		resumo: "Calendário de jogos", descricao: "Do arquivo JOGOS_FILE, com dia da semana e se o jogo cai em feriado.",
		params:   []schema{pCompeticao, q("limite", "Máximo de jogos.", schema{"type": "integer", "minimum": 1})},
		resposta: ref("Calendario"), erros: []int{http.StatusBadRequest, http.StatusInternalServerError},
	}, s.jogosDaCategoria(sec.categoria, false))
	s.handle(mux, "GET "+p+"/jogos/proximos", "?limite=3", doc{
		resumo:   "Próximos jogos do calendário (de hoje em diante)",
		params:   []schema{pCompeticao, q("limite", "Máximo de jogos.", schema{"type": "integer", "minimum": 1, "default": 5})},
		resposta: ref("Calendario"), erros: []int{http.StatusBadRequest, http.StatusInternalServerError},
	}, s.jogosDaCategoria(sec.categoria, true))
	s.registrarAcervo(mux, sec)
}

// errosEmJSON troca as respostas 404 e 405 em texto puro do ServeMux por JSON.
// O mux já responde 405, com o header Allow, quando o caminho existe com outro método.
func errosEmJSON(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, pattern := mux.Handler(r); pattern != "" {
			mux.ServeHTTP(w, r)
			return
		}

		// Sem rota: deixa o mux decidir entre 404 e 405 e descarta o texto dele.
		rec := &descartaCorpo{header: http.Header{}, status: http.StatusNotFound}
		mux.ServeHTTP(rec, r)

		if rec.status == http.StatusMethodNotAllowed {
			allow := rec.header.Get("Allow")
			w.Header().Set("Allow", allow)
			writeError(w, http.StatusMethodNotAllowed, "Método não permitido nesta rota. Falta marcada.", "Métodos aceitos: "+allow)
			return
		}
		writeError(w, http.StatusNotFound, "Rota não encontrada. Impedimento marcado.", "GET / lista as rotas disponíveis.")
	})
}

// descartaCorpo guarda status e headers de uma resposta e joga o corpo fora.
type descartaCorpo struct {
	header http.Header
	status int
}

func (d *descartaCorpo) Header() http.Header         { return d.header }
func (d *descartaCorpo) Write(b []byte) (int, error) { return len(b), nil }
func (d *descartaCorpo) WriteHeader(code int)        { d.status = code }

// chain aplica os middlewares de modo que o primeiro da lista fique por fora.
func chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

func vaiCorinthians(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Vai-Corinthians", "sempre")
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool // o cabeçalho já foi enviado
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.wrote {
		return
	}
	r.status = code
	r.wrote = true
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.wrote {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(b)
}

// Unwrap permite que http.ResponseController alcance o writer original.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.log.Info("requisição",
			"metodo", r.Method,
			"caminho", r.URL.Path,
			"status", rec.status,
			"duracao", time.Since(start),
		)
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if p := recover(); p != nil {
				if p == http.ErrAbortHandler {
					panic(p) // o próprio net/http usa esse panic para abortar a resposta
				}
				s.log.Error("panic no handler", "erro", p, "caminho", r.URL.Path)
				// Se a resposta já começou, não dá para trocar o status: só registra no log.
				if !rec.wrote {
					writeError(w, http.StatusInternalServerError, "Deu ruim dentro de campo.", "")
				}
			}
		}()
		next.ServeHTTP(rec, r)
	})
}

type apiError struct {
	Erro string `json:"erro"`
	Dica string `json:"dica,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg, dica string) {
	writeJSON(w, status, apiError{Erro: msg, Dica: dica})
}
