package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"timao-api/internal/brasilapi"
	"timao-api/internal/data"
	"timao-api/internal/jogos"
)

type fakeJogos struct {
	lista []jogos.Jogo
	err   error
}

func (f fakeJogos) Jogos(ctx context.Context) ([]jogos.Jogo, error) {
	return f.lista, f.err
}

// novoServidor monta um servidor com "hoje" fixo em 12/10/2026.
func novoServidor(b fakeBrasil, j fakeJogos) *Server {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := New(data.New(), b, j, log)
	s.now = func() time.Time { return time.Date(2026, 10, 12, 12, 0, 0, 0, s.loc) }
	return s
}

var calendarioTeste = []jogos.Jogo{
	{Data: "2026-10-01", Adversario: "Passado FC", Competicao: "Campeonato Brasileiro", Mando: "casa"},
	{Data: "2026-11-02", Horario: "19:00", Adversario: "Feriado FC", Competicao: "Copa do Brasil", Mando: "fora"},
	{Data: "2026-11-15", Horario: "18:30", Adversario: "Domingo FC", Competicao: "Campeonato Brasileiro", Mando: "casa"},
}

var feriadosTeste = []brasilapi.Feriado{
	{Date: "2026-11-02", Name: "Finados", Type: "national"},
	{Date: "2026-11-15", Name: "Proclamação da República", Type: "national"},
}

func jogosDe(t *testing.T, rec *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	lista, _ := decode(t, rec)["jogos"].([]any)
	out := make([]map[string]any, 0, len(lista))
	for _, v := range lista {
		out = append(out, v.(map[string]any))
	}
	return out
}

func TestJogosProximosCruzaComFeriados(t *testing.T) {
	h := novoServidor(fakeBrasil{feriados: feriadosTeste}, fakeJogos{lista: calendarioTeste}).Handler()

	rec := get(t, h, "/v1/jogos/proximos")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	lista := jogosDe(t, rec)
	if len(lista) != 2 {
		t.Fatalf("esperava 2 jogos futuros, veio %d", len(lista))
	}

	primeiro := lista[0]
	if primeiro["adversario"] != "Feriado FC" {
		t.Fatalf("primeiro jogo inesperado: %v", primeiro["adversario"])
	}
	if primeiro["dia_semana"] != "segunda-feira" || primeiro["fim_de_semana"] != false {
		t.Fatalf("dia da semana errado: %v", primeiro)
	}
	if primeiro["cai_em_feriado"] != true || primeiro["feriado"] != "Finados" {
		t.Fatalf("deveria cair em Finados: %v", primeiro)
	}

	segundo := lista[1]
	if segundo["dia_semana"] != "domingo" || segundo["fim_de_semana"] != true {
		t.Fatalf("15/11/2026 é domingo: %v", segundo)
	}
}

func TestJogosTodosEFiltroPorCompeticao(t *testing.T) {
	h := novoServidor(fakeBrasil{feriados: feriadosTeste}, fakeJogos{lista: calendarioTeste}).Handler()

	if n := len(jogosDe(t, get(t, h, "/v1/jogos"))); n != 3 {
		t.Fatalf("/v1/jogos deveria trazer os 3 jogos, veio %d", n)
	}
	if n := len(jogosDe(t, get(t, h, "/v1/jogos?competicao=copa+do+brasil"))); n != 1 {
		t.Fatalf("filtro por competição deveria trazer 1 jogo, veio %d", n)
	}
}

func TestJogosLimite(t *testing.T) {
	h := novoServidor(fakeBrasil{feriados: feriadosTeste}, fakeJogos{lista: calendarioTeste}).Handler()

	if n := len(jogosDe(t, get(t, h, "/v1/jogos/proximos?limite=1"))); n != 1 {
		t.Fatalf("limite=1 deveria trazer 1 jogo, veio %d", n)
	}
	for _, path := range []string{"/v1/jogos/proximos?limite=abc", "/v1/jogos/proximos?limite=0"} {
		if rec := get(t, h, path); rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: status %d, esperava 400", path, rec.Code)
		}
	}
}

func TestJogosSemFeriadosQuandoBrasilAPIFalha(t *testing.T) {
	h := novoServidor(fakeBrasil{feriadosErr: errors.New("fora do ar")}, fakeJogos{lista: calendarioTeste}).Handler()

	rec := get(t, h, "/v1/jogos/proximos")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, os jogos deveriam sair mesmo assim", rec.Code)
	}
	if decode(t, rec)["aviso"] == nil {
		t.Fatal("esperava um campo aviso")
	}
	lista := jogosDe(t, rec)
	if len(lista) != 2 {
		t.Fatalf("esperava 2 jogos, veio %d", len(lista))
	}
	if _, existe := lista[0]["cai_em_feriado"]; existe {
		t.Fatal("sem feriados, o campo cai_em_feriado não deveria aparecer")
	}
}

func TestJogosErroNoCalendario(t *testing.T) {
	h := novoServidor(fakeBrasil{}, fakeJogos{err: errors.New("arquivo corrompido")}).Handler()

	if rec := get(t, h, "/v1/jogos"); rec.Code != http.StatusInternalServerError {
		t.Fatalf("status %d, esperava 500", rec.Code)
	}
}
