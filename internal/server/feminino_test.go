package server

import (
	"net/http"
	"testing"

	"timao-api/internal/jogos"
)

func TestFemininoClube(t *testing.T) {
	rec := get(t, novoHandler(fakeBrasil{}), "/v1/feminino/clube")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body := decode(t, rec)
	if body["nome"] != "Corinthians Feminino" {
		t.Fatalf("nome inesperado: %v", body["nome"])
	}
	apelidos, _ := body["apelidos"].([]any)
	if len(apelidos) != 1 || apelidos[0] != "Brabas" {
		t.Fatalf("apelidos inesperados: %v", apelidos)
	}
}

func TestFemininoTitulos(t *testing.T) {
	h := handlerComAcervo(t)

	rec := get(t, h, "/v1/feminino/titulos?escopo=continental")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body := decode(t, rec)
	if body["total"] != float64(2) {
		t.Fatalf("total de títulos continentais do banco de teste esperado 2, veio %v", body["total"])
	}

	rec = get(t, h, "/v1/feminino/titulos/libertadores")
	if rec.Code != http.StatusOK {
		t.Fatalf("libertadores: status %d", rec.Code)
	}
	if decode(t, rec)["notas"] == nil {
		t.Fatal("a Libertadores deveria trazer a nota sobre o título de 2017 com o Audax")
	}

	if rec := get(t, h, "/v1/feminino/titulos/nao-existe"); rec.Code != http.StatusNotFound {
		t.Fatalf("título inexistente: status %d", rec.Code)
	}
}

func TestFemininoIdolas(t *testing.T) {
	h := novoHandler(fakeBrasil{})

	rec := get(t, h, "/v1/feminino/idolas")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if lista, _ := decode(t, rec)["idolas"].([]any); len(lista) == 0 {
		t.Fatal("a lista deveria vir na chave idolas e não estar vazia")
	}

	if rec := get(t, h, "/v1/feminino/idolas/aleatoria"); rec.Code != http.StatusOK {
		t.Fatalf("aleatória: status %d", rec.Code)
	}
	if rec := get(t, h, "/v1/feminino/idolas/gabi-zanotti"); rec.Code != http.StatusOK {
		t.Fatalf("gabi-zanotti: status %d", rec.Code)
	}
	if rec := get(t, h, "/v1/feminino/idolas/ninguem"); rec.Code != http.StatusNotFound {
		t.Fatalf("inexistente: status %d", rec.Code)
	}
}

func TestFemininoHistoria(t *testing.T) {
	rec := get(t, novoHandler(fakeBrasil{}), "/v1/feminino/historia")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if marcos, _ := decode(t, rec)["marcos"].([]any); len(marcos) == 0 {
		t.Fatal("história vazia")
	}
}

func TestMasculinoEFemininoNaoSeMisturam(t *testing.T) {
	h := novoHandler(fakeBrasil{})

	if got := decode(t, get(t, h, "/v1/clube"))["nome"]; got != "Corinthians" {
		t.Fatalf("/v1/clube: %v", got)
	}
	if rec := get(t, h, "/v1/idolos/gabi-zanotti"); rec.Code != http.StatusNotFound {
		t.Fatalf("ídola do feminino não deveria existir em /v1/idolos, status %d", rec.Code)
	}
	if rec := get(t, h, "/v1/feminino/idolas/socrates"); rec.Code != http.StatusNotFound {
		t.Fatalf("ídolo do masculino não deveria existir em /v1/feminino/idolas, status %d", rec.Code)
	}
	if rec := get(t, h, "/v1/feminino/rota-inexistente"); rec.Code != http.StatusNotFound {
		t.Fatalf("rota inexistente: status %d", rec.Code)
	}
}

var calendarioMisto = []jogos.Jogo{
	{Data: "2026-11-02", Adversario: "Masc FC", Competicao: "Copa do Brasil", Mando: "fora"},
	{Data: "2026-10-31", Adversario: "Fem FC", Competicao: "Campeonato Brasileiro Feminino", Mando: "casa", Categoria: jogos.Feminino},
	{Data: "2026-10-01", Adversario: "Fem Passado FC", Competicao: "Campeonato Brasileiro Feminino", Mando: "fora", Categoria: jogos.Feminino},
	{Data: "2026-11-08", Adversario: "Masc Explícito FC", Competicao: "Campeonato Brasileiro", Mando: "casa", Categoria: jogos.Masculino},
}

func TestJogosSeparadosPorCategoria(t *testing.T) {
	h := novoServidor(fakeBrasil{feriados: feriadosTeste}, fakeJogos{lista: calendarioMisto}).Handler()

	masc := jogosDe(t, get(t, h, "/v1/jogos"))
	if len(masc) != 2 {
		t.Fatalf("/v1/jogos deveria trazer só os 2 jogos masculinos, veio %d", len(masc))
	}

	fem := jogosDe(t, get(t, h, "/v1/feminino/jogos"))
	if len(fem) != 2 {
		t.Fatalf("/v1/feminino/jogos deveria trazer os 2 jogos femininos, veio %d", len(fem))
	}

	proximos := jogosDe(t, get(t, h, "/v1/feminino/jogos/proximos"))
	if len(proximos) != 1 || proximos[0]["adversario"] != "Fem FC" {
		t.Fatalf("próximos femininos inesperados: %v", proximos)
	}
	if proximos[0]["dia_semana"] != "sábado" || proximos[0]["fim_de_semana"] != true {
		t.Fatalf("31/10/2026 é sábado: %v", proximos[0])
	}
	if proximos[0]["cai_em_feriado"] != false {
		t.Fatalf("31/10/2026 não é feriado nacional: %v", proximos[0])
	}
}

func TestJogosFemininosComFiltroDeCompeticao(t *testing.T) {
	h := novoServidor(fakeBrasil{feriados: feriadosTeste}, fakeJogos{lista: calendarioMisto}).Handler()

	lista := jogosDe(t, get(t, h, "/v1/feminino/jogos?competicao=campeonato+brasileiro+feminino"))
	if len(lista) != 2 {
		t.Fatalf("esperava 2 jogos da competição, veio %d", len(lista))
	}
	if lista := jogosDe(t, get(t, h, "/v1/feminino/jogos?competicao=Copa+do+Brasil")); len(lista) != 0 {
		t.Fatalf("a Copa do Brasil do exemplo é masculina, não deveria aparecer no feminino: %v", lista)
	}
}
