package server

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"timao-api/internal/acervo"
	"timao-api/internal/atualizador"
)

// Rotas servidas a partir do timao.db (gerado por cmd/importar).

// ComAtualizador expõe em GET /v1/acervo a situação das atualizações do banco.
func (s *Server) ComAtualizador(st StatusAcervo) *Server {
	s.status = st
	return s
}

// statusAcervo mostra quando o banco foi gerado e quando será atualizado.
func (s *Server) statusAcervo(w http.ResponseWriter, r *http.Request) {
	if s.status != nil {
		writeJSON(w, http.StatusOK, s.status.Status())
		return
	}
	writeJSON(w, http.StatusOK, atualizador.Status{Disponivel: s.acervo.Disponivel()})
}

// ComAcervo liga o servidor ao banco de dados importado. Sem ele, as rotas do
// acervo respondem 503.
func (s *Server) ComAcervo(a *acervo.Acervo) *Server {
	s.acervo = a
	return s
}

func (s *Server) registrarAcervo(mux *http.ServeMux, sec secao) {
	p := sec.prefixo
	acervoErros := []int{http.StatusBadRequest, http.StatusServiceUnavailable}

	s.handle(mux, "GET "+p+"/pessoas", "?funcao=jogador&nome="+sec.nomeExemplo, doc{
		resumo: "Jogadores(as) e técnicos(as) que passaram pelo " + sec.time,
		params: []schema{
			q("funcao", "", strEnum("", "jogador", "tecnico")),
			q("nome", "Parte do nome, sem diferenciar acentos.", schema{"type": "string", "example": sec.nomeExemplo}),
			q("posicao", "Parte da posição, ex.: goleir.", schema{"type": "string"}),
			pPagina, pLimite,
		},
		resposta: ref("ListaPessoas"), erros: acervoErros,
	}, s.pessoas(sec))
	s.handle(mux, "GET "+p+"/pessoas/{wikidata}", "", doc{
		resumo: "Uma pessoa, com passagens, gols nas partidas e elencos", resposta: ref("Pessoa"),
		erros: []int{http.StatusNotFound, http.StatusServiceUnavailable},
	}, s.pessoa(sec))
	s.handle(mux, "GET "+p+"/pessoas/{wikidata}/gols", "", doc{
		resumo: "Cada gol da pessoa pelo Corinthians nas partidas do acervo", resposta: ref("GolsDaPessoa"),
		erros: []int{http.StatusNotFound, http.StatusServiceUnavailable},
	}, s.golsDaPessoa(sec))
	s.handle(mux, "GET "+p+"/partidas", "?temporada=2025&resultado=vitoria", doc{
		resumo:    "Partidas, com o resumo de desempenho do filtro",
		descricao: "Das mais recentes para as mais antigas. O resumo considera todas as partidas disputadas do filtro, não só a página.",
		params: []schema{
			pTemporada, pCompetPar,
			q("adversario", "Parte do nome do adversário, sem diferenciar acentos.", schema{"type": "string", "example": "palmeiras"}),
			q("mando", "", strEnum("", "casa", "fora")),
			q("resultado", "", strEnum("", "vitoria", "empate", "derrota")),
			q("de", "Data inicial, AAAA-MM-DD.", schema{"type": "string", "format": "date"}),
			q("ate", "Data final, AAAA-MM-DD.", schema{"type": "string", "format": "date"}),
			q("ordem", "", strEnum("", "desc", "asc")),
			pPagina, pLimite,
		},
		resposta: ref("ListaPartidas"), erros: acervoErros,
	}, s.partidas(sec))
	s.handle(mux, "GET "+p+"/partidas/{data}", "", doc{
		resumo: "A partida do dia, com os gols", resposta: ref("Partida"),
		erros: []int{http.StatusBadRequest, http.StatusNotFound, http.StatusServiceUnavailable},
	}, s.partida(sec))
	s.handle(mux, "GET "+p+"/artilharia", "?temporada=2025", doc{
		resumo:    "Artilharia nas partidas do acervo",
		descricao: "Conta os gols das partidas do banco (sem gols contra), não a carreira inteira.",
		params:    []schema{pTemporada, pCompetPar, q("limite", "", schema{"type": "integer", "minimum": 1, "maximum": 200, "default": 20})},
		resposta:  ref("Artilharia"), erros: acervoErros,
	}, s.artilharia(sec))
	s.handle(mux, "GET "+p+"/temporadas", "", doc{
		resumo: "Temporadas com partidas no acervo e o resumo de cada uma", resposta: ref("Temporadas"),
		erros: []int{http.StatusServiceUnavailable},
	}, s.temporadas(sec))
	s.handle(mux, "GET "+p+"/elenco", "?temporada=2025", doc{
		resumo: "Elenco de uma temporada", descricao: "Sem temporada, devolve a mais recente.",
		params: []schema{pTemporada}, resposta: ref("Elenco"),
		erros: []int{http.StatusBadRequest, http.StatusNotFound, http.StatusServiceUnavailable},
	}, s.elenco(sec))
}

// semAcervo responde 503 quando a API subiu sem o timao.db.
func (s *Server) semAcervo(w http.ResponseWriter) bool {
	if s.acervo.Disponivel() {
		return false
	}
	writeError(w, http.StatusServiceUnavailable, "O acervo ainda não foi gerado.",
		"Rode go run ./cmd/importar e reinicie a API (ou aponte DB_FILE para o timao.db).")
	return true
}

func (s *Server) erroAcervo(w http.ResponseWriter, err error, naoEncontrado string) {
	if errors.Is(err, acervo.ErrNaoEncontrado) {
		writeError(w, http.StatusNotFound, naoEncontrado, "")
		return
	}
	s.log.Error("falha ao consultar o acervo", "erro", err)
	writeError(w, http.StatusInternalServerError, "Não consegui consultar o acervo.", "")
}

// parametros lê e valida a query string, guardando o primeiro erro encontrado.
type parametros struct {
	r    *http.Request
	erro string
}

func (p *parametros) texto(nome string) string {
	return strings.TrimSpace(p.r.URL.Query().Get(nome))
}

func (p *parametros) opcao(nome string, validas ...string) string {
	v := strings.ToLower(p.texto(nome))
	if v == "" {
		return ""
	}
	for _, ok := range validas {
		if v == ok {
			return v
		}
	}
	p.falhar(fmt.Sprintf("Parâmetro %s inválido. Use: %s.", nome, strings.Join(validas, ", ")))
	return ""
}

func (p *parametros) inteiro(nome string) int {
	v := p.texto(nome)
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		p.falhar(fmt.Sprintf("Parâmetro %s inválido. Use um número inteiro maior que zero.", nome))
		return 0
	}
	return n
}

func (p *parametros) data(nome string) string {
	v := p.texto(nome)
	if v == "" {
		return ""
	}
	if _, err := time.Parse("2006-01-02", v); err != nil {
		p.falhar(fmt.Sprintf("Parâmetro %s inválido. Use AAAA-MM-DD.", nome))
		return ""
	}
	return v
}

func (p *parametros) paginacao() acervo.Paginacao {
	pg := acervo.Paginacao{Pagina: p.inteiro("pagina"), Limite: p.inteiro("limite")}
	if pg.Limite > acervo.LimiteMaximo {
		p.falhar(fmt.Sprintf("Parâmetro limite inválido. O máximo é %d.", acervo.LimiteMaximo))
	}
	return pg
}

// falhar guarda a mensagem do primeiro parâmetro inválido (mostrada ao usuário).
func (p *parametros) falhar(msg string) {
	if p.erro == "" {
		p.erro = msg
	}
}

// ok responde 400 e devolve false se algum parâmetro estava errado.
func (p *parametros) ok(w http.ResponseWriter) bool {
	if p.erro != "" {
		writeError(w, http.StatusBadRequest, p.erro, "")
		return false
	}
	return true
}

func (s *Server) pessoas(sec secao) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.semAcervo(w) {
			return
		}
		q := parametros{r: r}
		f := acervo.FiltroPessoas{
			Categoria: string(sec.categoria),
			Funcao:    q.opcao("funcao", "jogador", "tecnico"),
			Nome:      q.texto("nome"),
			Posicao:   q.texto("posicao"),
			Paginacao: q.paginacao(),
		}
		if !q.ok(w) {
			return
		}
		out, err := s.acervo.Pessoas(r.Context(), f)
		if err != nil {
			s.erroAcervo(w, err, "")
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func (s *Server) pessoa(sec secao) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.semAcervo(w) {
			return
		}
		p, err := s.acervo.Pessoa(r.Context(), string(sec.categoria), r.PathValue("wikidata"))
		if err != nil {
			s.erroAcervo(w, err, "Essa pessoa não aparece no acervo desta categoria. Use o código do Wikidata, como Q312937.")
			return
		}
		writeJSON(w, http.StatusOK, p)
	}
}

func (s *Server) golsDaPessoa(sec secao) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.semAcervo(w) {
			return
		}
		gols, err := s.acervo.GolsDaPessoa(r.Context(), string(sec.categoria), r.PathValue("wikidata"))
		if err != nil {
			s.erroAcervo(w, err, "Essa pessoa não aparece no acervo desta categoria.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"total": len(gols), "gols": gols})
	}
}

func (s *Server) partidas(sec secao) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.semAcervo(w) {
			return
		}
		q := parametros{r: r}
		f := acervo.FiltroPartidas{
			Categoria:  string(sec.categoria),
			Temporada:  q.inteiro("temporada"),
			Competicao: q.texto("competicao"),
			Adversario: q.texto("adversario"),
			Mando:      q.opcao("mando", "casa", "fora"),
			Resultado:  q.opcao("resultado", "vitoria", "empate", "derrota"),
			De:         q.data("de"),
			Ate:        q.data("ate"),
			Crescente:  q.opcao("ordem", "asc", "desc") == "asc",
			Paginacao:  q.paginacao(),
		}
		if !q.ok(w) {
			return
		}
		out, err := s.acervo.Partidas(r.Context(), f)
		if err != nil {
			s.erroAcervo(w, err, "")
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func (s *Server) partida(sec secao) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.semAcervo(w) {
			return
		}
		data := r.PathValue("data")
		if _, err := time.Parse("2006-01-02", data); err != nil {
			writeError(w, http.StatusBadRequest, "Data inválida. Use AAAA-MM-DD.", "GET "+sec.prefixo+"/partidas lista as datas.")
			return
		}
		p, err := s.acervo.Partida(r.Context(), string(sec.categoria), data)
		if err != nil {
			s.erroAcervo(w, err, "Não há partida desta categoria nessa data no acervo.")
			return
		}
		writeJSON(w, http.StatusOK, p)
	}
}

func (s *Server) artilharia(sec secao) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.semAcervo(w) {
			return
		}
		q := parametros{r: r}
		f := acervo.FiltroArtilharia{
			Categoria:  string(sec.categoria),
			Temporada:  q.inteiro("temporada"),
			Competicao: q.texto("competicao"),
			Limite:     q.inteiro("limite"),
		}
		if f.Limite > acervo.LimiteMaximo {
			q.falhar(fmt.Sprintf("Parâmetro limite inválido. O máximo é %d.", acervo.LimiteMaximo))
		}
		if !q.ok(w) {
			return
		}
		lista, err := s.acervo.Artilharia(r.Context(), f)
		if err != nil {
			s.erroAcervo(w, err, "")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"aviso":      "Conta só os gols das partidas do acervo (temporadas com página na Wikipédia), não a carreira inteira.",
			"artilharia": lista,
		})
	}
}

func (s *Server) temporadas(sec secao) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.semAcervo(w) {
			return
		}
		lista, err := s.acervo.Temporadas(r.Context(), string(sec.categoria))
		if err != nil {
			s.erroAcervo(w, err, "")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"temporadas": lista})
	}
}

func (s *Server) titulos(sec secao) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.semAcervo(w) {
			return
		}
		q := parametros{r: r}
		f := acervo.FiltroTitulos{
			Categoria: string(sec.categoria),
			Escopo:    q.opcao("escopo", "mundial", "continental", "nacional", "interestadual", "estadual", "internacional", "amistoso", "outros"),
			Tipo:      q.opcao("tipo", "oficial", "outros", "todos"),
		}
		if !q.ok(w) {
			return
		}
		lista, err := s.acervo.Titulos(r.Context(), f)
		if err != nil {
			s.erroAcervo(w, err, "")
			return
		}
		total := 0
		for _, c := range lista {
			total += c.Quantidade
		}
		writeJSON(w, http.StatusOK, map[string]any{"total": total, "titulos": lista})
	}
}

func (s *Server) tituloPorSlug(sec secao) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.semAcervo(w) {
			return
		}
		c, err := s.acervo.Titulo(r.Context(), string(sec.categoria), r.PathValue("slug"))
		if err != nil {
			s.erroAcervo(w, err, "Esse título não está na nossa sala de troféus. GET "+sec.prefixo+"/titulos?tipo=todos lista os slugs.")
			return
		}
		writeJSON(w, http.StatusOK, c)
	}
}

func (s *Server) elenco(sec secao) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.semAcervo(w) {
			return
		}
		q := parametros{r: r}
		temporada := q.inteiro("temporada")
		if !q.ok(w) {
			return
		}
		e, err := s.acervo.Elenco(r.Context(), string(sec.categoria), temporada)
		if err != nil {
			s.erroAcervo(w, err, "Não há elenco dessa temporada no acervo. GET "+sec.prefixo+"/elenco mostra o mais recente e as temporadas disponíveis.")
			return
		}
		writeJSON(w, http.StatusOK, e)
	}
}
