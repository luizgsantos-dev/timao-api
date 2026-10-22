package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"timao-api/internal/acervo/acervotest"
	"timao-api/internal/brasilapi"
	"timao-api/internal/data"
	"timao-api/internal/jogos"
)

// servidorCompleto tem tudo preenchido: banco de teste, Brasil API e calendário falsos.
func servidorCompleto(t *testing.T) *Server {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	b := fakeBrasil{
		cep:      &brasilapi.CEP{CEP: "08295005", State: "SP", City: "São Paulo", Neighborhood: "Itaquera", Street: "Av. Miguel Ignácio Curi", Location: json.RawMessage(`{"type":"Point"}`)},
		feriados: []brasilapi.Feriado{{Date: "2026-11-02", Name: "Finados", Type: "national"}},
	}
	j := fakeJogos{lista: []jogos.Jogo{
		{Data: "2026-11-02", Horario: "19:00", Adversario: "Feriado FC", Competicao: "Copa do Brasil", Mando: "fora", Categoria: "masculino", Local: "Arena X"},
		{Data: "2026-11-08", Adversario: "Brabas FC", Competicao: "Campeonato Brasileiro Feminino", Mando: "casa", Categoria: "feminino"},
	}}
	s := New(data.New(), b, j, log).ComAcervo(acervotest.Novo(t))
	s.now = func() time.Time { return time.Date(2026, 10, 12, 12, 0, 0, 0, s.loc) }
	return s
}

// exemplo preenche os parâmetros de caminho com valores que existem no banco de teste.
func exemplo(caminho string) string {
	fem := strings.HasPrefix(caminho, "/v1/feminino")
	valores := map[string][2]string{ // masculino, feminino
		"wikidata": {"Q1", "Q2"},
		"data":     {"2025-03-01", "2025-05-05"},
	}
	slug := map[string][2]string{"titulos": {"mundial-de-clubes", "libertadores"}, "idolos": {"socrates", ""}, "idolas": {"", "gabi-zanotti"}}
	partes := strings.Split(caminho, "/")
	for i, p := range partes {
		if !strings.HasPrefix(p, "{") {
			continue
		}
		nome := strings.Trim(p, "{}")
		v := valores[nome]
		if nome == "slug" {
			v = slug[partes[i-1]]
		}
		if fem {
			partes[i] = v[1]
		} else {
			partes[i] = v[0]
		}
	}
	return strings.Join(partes, "/")
}

func TestRespostasBatemComOSchema(t *testing.T) {
	s := servidorCompleto(t)
	h := s.Handler()
	spec := s.openAPI()
	comps := spec["components"].(schema)["schemas"].(schema)

	if len(s.docs) != len(s.rotas)+1 { // + a raiz, que não entra na lista de rotas
		t.Fatalf("%d rotas documentadas para %d registradas", len(s.docs), len(s.rotas))
	}
	for _, r := range s.docs {
		if r.caminho == "/docs" || r.caminho == "/openapi.json" {
			continue
		}
		t.Run(r.caminho, func(t *testing.T) {
			url := exemplo(r.caminho)
			rec := get(t, h, url)
			var corpo any
			if err := json.Unmarshal(rec.Body.Bytes(), &corpo); err != nil {
				t.Fatalf("%s: JSON inválido: %v", url, err)
			}
			esperado := r.doc.resposta
			if rec.Code != http.StatusOK {
				if !slices.Contains(r.doc.erros, rec.Code) {
					t.Fatalf("%s: status %d não está documentado (documentados: %v)", url, rec.Code, r.doc.erros)
				}
				esperado = ref("Erro")
				t.Logf("%s respondeu %d (documentado)", url, rec.Code)
			}
			for _, e := range validar(esperado, corpo, "$", comps) {
				t.Errorf("%s: %s", url, e)
			}
		})
	}
}

func TestErrosDocumentadosBatemComOSchema(t *testing.T) {
	h := servidorCompleto(t).Handler()
	comps := componentes()
	for _, url := range []string{"/v1/partidas?resultado=goleada", "/v1/pessoas/Q999", "/nao-existe"} {
		var corpo any
		if err := json.Unmarshal(get(t, h, url).Body.Bytes(), &corpo); err != nil {
			t.Fatal(err)
		}
		for _, e := range validar(ref("Erro"), corpo, "$", comps) {
			t.Errorf("%s: %s", url, e)
		}
	}
}

func TestDocumentoOpenAPI(t *testing.T) {
	h := servidorCompleto(t).Handler()
	rec := get(t, h, "/openapi.json")
	var spec map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &spec); err != nil {
		t.Fatal(err)
	}
	if spec["openapi"] != "3.1.0" {
		t.Fatalf("versão do OpenAPI: %v", spec["openapi"])
	}
	paths := spec["paths"].(map[string]any)
	for _, p := range []string{"/", "/v1/partidas", "/v1/feminino/pessoas/{wikidata}", "/v1/feminino/idolas/aleatoria", "/v1/acervo"} {
		if _, ok := paths[p]; !ok {
			t.Errorf("faltou %s no documento", p)
		}
	}
	// Todo $ref aponta para um schema que existe.
	comps := spec["components"].(map[string]any)["schemas"].(map[string]any)
	var faltando []string
	var andar func(v any)
	andar = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if r, ok := x["$ref"].(string); ok {
				if _, ok := comps[strings.TrimPrefix(r, "#/components/schemas/")]; !ok {
					faltando = append(faltando, r)
				}
			}
			for _, y := range x {
				andar(y)
			}
		case []any:
			for _, y := range x {
				andar(y)
			}
		}
	}
	andar(spec)
	if len(faltando) > 0 {
		t.Fatalf("referências quebradas: %v", faltando)
	}
	// operationId único.
	ids := map[string]bool{}
	for p, item := range paths {
		for _, op := range item.(map[string]any) {
			id := op.(map[string]any)["operationId"].(string)
			if ids[id] {
				t.Errorf("operationId repetido: %s (%s)", id, p)
			}
			ids[id] = true
		}
	}

	pagina := get(t, h, "/docs")
	if !strings.Contains(pagina.Header().Get("Content-Type"), "text/html") || !strings.Contains(pagina.Body.String(), "/openapi.json") {
		t.Fatal("/docs deveria ser a página HTML que carrega /openapi.json")
	}
}

// validar confere v contra o schema, no subconjunto de JSON Schema usado em
// openapi_schemas.go. Objetos são fechados: campo não documentado é erro.
func validar(s schema, v any, caminho string, comps schema) []string {
	if r, ok := s["$ref"].(string); ok {
		alvo, ok := comps[strings.TrimPrefix(r, "#/components/schemas/")].(schema)
		if !ok {
			return []string{caminho + ": referência inexistente " + r}
		}
		return validar(alvo, v, caminho, comps)
	}
	var erros []string
	falhar := func(f string, a ...any) { erros = append(erros, caminho+": "+fmt.Sprintf(f, a...)) }

	if enum, ok := s["enum"].([]string); ok {
		if str, _ := v.(string); !slices.Contains(enum, str) {
			falhar("%v fora de %v", v, enum)
		}
	}
	tipo, _ := s["type"].(string)
	switch tipo {
	case "":
		// sem tipo: aceita qualquer valor
	case "string":
		str, ok := v.(string)
		if !ok {
			falhar("esperava texto, veio %T", v)
		} else if s["format"] == "date-time" {
			if _, err := time.Parse(time.RFC3339, str); err != nil {
				falhar("data-hora inválida %q", str)
			}
		}
	case "integer":
		n, ok := v.(float64)
		if !ok || n != math.Trunc(n) {
			falhar("esperava inteiro, veio %v", v)
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			falhar("esperava booleano, veio %T", v)
		}
	case "array":
		lista, ok := v.([]any)
		if !ok {
			falhar("esperava lista, veio %T", v)
			break
		}
		itens, _ := s["items"].(schema)
		for i, item := range lista {
			erros = append(erros, validar(itens, item, caminho+"["+strconv.Itoa(i)+"]", comps)...)
		}
	case "object":
		o, ok := v.(map[string]any)
		if !ok {
			falhar("esperava objeto, veio %T", v)
			break
		}
		props, _ := s["properties"].(schema)
		if obrig, ok := s["required"].([]string); ok {
			for _, k := range obrig {
				if _, ok := o[k]; !ok {
					falhar("faltou o campo obrigatório %q", k)
				}
			}
		}
		for k, val := range o {
			p, ok := props[k].(schema)
			if !ok {
				if props != nil {
					falhar("campo não documentado %q", k)
				}
				continue
			}
			erros = append(erros, validar(p, val, caminho+"."+k, comps)...)
		}
	default:
		falhar("tipo %q não suportado pelo validador", tipo)
	}
	return erros
}

func TestValidadorPegaCampoNaoDocumentado(t *testing.T) {
	// Tira um campo do schema de Pessoa: a resposta real passa a ter um campo a mais.
	comps := componentes()
	pessoa := comps["Pessoa"].(schema)
	props := schema{}
	for k, v := range pessoa["properties"].(schema) {
		if k != "elencos" {
			props[k] = v
		}
	}
	pessoa["properties"] = props

	var corpo any
	_ = json.Unmarshal(get(t, servidorCompleto(t).Handler(), "/v1/feminino/pessoas/Q2").Body.Bytes(), &corpo)
	erros := validar(ref("Pessoa"), corpo, "$", comps)
	if len(erros) != 1 || !strings.Contains(erros[0], `"elencos"`) {
		t.Fatalf("o validador deveria apontar o campo elencos, veio %v", erros)
	}
}

func TestValidador(t *testing.T) {
	comps := componentes()
	casos := []struct {
		nome  string
		v     string
		erros int
	}{
		{"erro válido", `{"erro":"x"}`, 0},
		{"faltou obrigatório", `{"dica":"x"}`, 1},
		{"campo a mais", `{"erro":"x","extra":1}`, 1},
		{"tipo errado", `{"erro":1}`, 1},
	}
	for _, c := range casos {
		var v any
		_ = json.Unmarshal([]byte(c.v), &v)
		if got := validar(ref("Erro"), v, "$", comps); len(got) != c.erros {
			t.Errorf("%s: %d erros (%v), esperava %d", c.nome, len(got), got, c.erros)
		}
	}
}
