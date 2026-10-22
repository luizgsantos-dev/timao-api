package server

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// O documento OpenAPI é montado a partir das próprias rotas registradas em
// Handler: cada s.handle traz a descrição da rota, então não há como uma rota
// existir sem estar documentada (ou o contrário).

// doc descreve uma rota para o OpenAPI.
type doc struct {
	resumo    string
	descricao string
	params    []schema // parâmetros de query (os de caminho saem do padrão da rota)
	resposta  schema   // schema da resposta 200
	erros     []int
}

type rotaDoc struct {
	metodo, caminho string
	doc             doc
}

// Parâmetros de caminho, pelo nome usado no padrão da rota.
var paramsDeCaminho = map[string]string{
	"slug":     "Identificador (veja a lista correspondente).",
	"wikidata": "Código do Wikidata da pessoa, ex.: Q102331.",
	"data":     "Data da partida, AAAA-MM-DD.",
}

var reParamCaminho = regexp.MustCompile(`\{(\w+)\}`)

func q(nome, descricao string, s schema) schema {
	return schema{"name": nome, "in": "query", "description": descricao, "schema": s}
}

// Parâmetros de query usados em mais de uma rota.
var (
	pPagina    = q("pagina", "Página, a partir de 1.", schema{"type": "integer", "minimum": 1, "default": 1})
	pLimite    = q("limite", "Itens por página.", schema{"type": "integer", "minimum": 1, "maximum": 200, "default": 50})
	pTemporada = q("temporada", "Ano da temporada.", schema{"type": "integer", "example": 2025})
	pCompetPar = q("competicao", "Parte do nome da competição, ex.: paulista.", schema{"type": "string"})
)

var descricaoErros = map[int]string{
	http.StatusBadRequest:          "Parâmetro inválido.",
	http.StatusNotFound:            "Não encontrado.",
	http.StatusInternalServerError: "Erro interno.",
	http.StatusBadGateway:          "A Brasil API não respondeu.",
	http.StatusServiceUnavailable:  "O acervo (timao.db) ainda não foi gerado.",
}

// documentar guarda a descrição de uma rota registrada.
func (s *Server) documentar(pattern string, d doc) {
	metodo, caminho, _ := strings.Cut(pattern, " ")
	s.docs = append(s.docs, rotaDoc{metodo: metodo, caminho: strings.TrimSuffix(caminho, "{$}"), doc: d})
}

// openAPI monta o documento OpenAPI 3.1 com as rotas documentadas.
func (s *Server) openAPI() schema {
	caminhos := schema{}
	for _, r := range s.docs {
		op := schema{
			"tags":        []string{grupo(r.caminho)},
			"summary":     r.doc.resumo,
			"operationId": idOperacao(r.caminho),
		}
		if r.doc.descricao != "" {
			op["description"] = r.doc.descricao
		}
		var params []schema
		for _, m := range reParamCaminho.FindAllStringSubmatch(r.caminho, -1) {
			params = append(params, schema{"name": m[1], "in": "path", "required": true,
				"description": paramsDeCaminho[m[1]], "schema": schema{"type": "string"}})
		}
		params = append(params, r.doc.params...)
		if len(params) > 0 {
			op["parameters"] = params
		}
		respostas := schema{"200": schema{
			"description": "OK",
			"content":     schema{"application/json": schema{"schema": r.doc.resposta}},
		}}
		for _, c := range r.doc.erros {
			respostas[strconv.Itoa(c)] = schema{"description": descricaoErros[c],
				"content": schema{"application/json": schema{"schema": ref("Erro")}}}
		}
		op["responses"] = respostas

		item, _ := caminhos[r.caminho].(schema)
		if item == nil {
			item = schema{}
			caminhos[r.caminho] = item
		}
		item[strings.ToLower(r.metodo)] = op
	}

	return schema{
		"openapi": "3.1.0",
		"info": schema{
			"title":   "timao-api",
			"version": version,
			"description": "API não oficial sobre o Sport Club Corinthians Paulista, com o masculino e o feminino (as Brabas) " +
				"com o mesmo peso: troque `/v1` por `/v1/feminino`. Os dados do acervo vêm de fontes abertas " +
				"(Wikidata, Wikipédia, openfootball, pacote brasileirao) e cada item traz a sua `fonte`. " +
				"Toda resposta leva o header `X-Vai-Corinthians: sempre`.",
			"license": schema{"name": "Dados: veja a licença de cada fonte"},
		},
		"servers": []schema{{"url": "/"}},
		"tags": []schema{
			{"name": "Masculino", "description": "Rotas do time masculino, em /v1."},
			{"name": "Feminino", "description": "As mesmas rotas para as Brabas, em /v1/feminino."},
			{"name": "Geral", "description": "Estádio, feriados, situação do acervo e a própria API."},
		},
		"paths":      caminhos,
		"components": schema{"schemas": componentes()},
	}
}

func grupo(caminho string) string {
	switch {
	case strings.HasPrefix(caminho, "/v1/feminino"):
		return "Feminino"
	case caminho == "/" || caminho == "/healthz" || strings.HasPrefix(caminho, "/v1/estadio") ||
		caminho == "/v1/feriados" || caminho == "/v1/acervo" || caminho == "/openapi.json" || caminho == "/docs":
		return "Geral"
	default:
		return "Masculino"
	}
}

// idOperacao gera um identificador estável: /v1/feminino/pessoas/{wikidata} -> feminino_pessoas_wikidata.
func idOperacao(caminho string) string {
	c := strings.TrimPrefix(caminho, "/v1")
	c = strings.NewReplacer("{", "", "}", "", "/", "_").Replace(strings.Trim(c, "/"))
	if c == "" {
		return "raiz"
	}
	if !strings.HasPrefix(c, "feminino") && strings.HasPrefix(caminho, "/v1/") {
		c = "masculino_" + c
	}
	return c
}

func (s *Server) openAPIJSON(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.openAPI())
}

// paginaDocs mostra o OpenAPI no Swagger UI, nas cores do clube.
func (s *Server) paginaDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(htmlDocs))
}

const htmlDocs = `<!doctype html>
<html lang="pt-BR">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>timao-api · documentação</title>
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui.css">
<style>
  :root { --preto: #111; --branco: #fff; --cinza: #e9e9e9; }
  body { margin: 0; background: var(--branco); }
  .faixa { background: var(--preto); color: var(--branco); padding: 20px 24px;
           font: 600 22px/1.2 system-ui, sans-serif; letter-spacing: .02em;
           border-bottom: 6px solid var(--branco); box-shadow: 0 6px 0 var(--preto); }
  .faixa small { display: block; font-weight: 400; font-size: 14px; opacity: .8; margin-top: 4px; }
  .swagger-ui .topbar { display: none; }
  .swagger-ui .opblock.opblock-get { border-color: var(--preto); background: var(--cinza); }
  .swagger-ui .opblock.opblock-get .opblock-summary-method { background: var(--preto); }
  .swagger-ui .opblock-tag { border-bottom: 2px solid var(--preto); }
  .swagger-ui .btn.execute { background: var(--preto); border-color: var(--preto); }
</style>
</head>
<body>
<div class="faixa">timao-api<small>Masculino e Brabas, com dados de fontes abertas. Não oficial.</small></div>
<div id="swagger-ui"></div>
<script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"></script>
<script>
  window.ui = SwaggerUIBundle({ url: "/openapi.json", dom_id: "#swagger-ui",
    docExpansion: "none", defaultModelsExpandDepth: 0, tryItOutEnabled: true });
</script>
</body>
</html>`
