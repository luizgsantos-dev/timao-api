package importar

import (
	"testing"

	"timao-api/internal/banco"
)

func n(v int) *int { return &v }

func TestMesmaPartida(t *testing.T) {
	vizinhas := []banco.Vizinha{
		{Data: "2026-10-30", Adversario: "Mirassol"},
		{Data: "2012-04-27", Adversario: "Emelec", GolsPro: n(0), GolsContra: n(0)},
		{Data: "2012-05-16", Adversario: "Vasco da Gama", GolsPro: n(0), GolsContra: n(0)},
	}
	casos := []struct {
		nome string
		p    banco.Partida
		want bool
	}{
		{"um dia de diferença, adversário com outro nome", banco.Partida{Data: "2026-10-31", Adversario: "Qualquer"}, true},
		{"jogo futuro com data provisória", banco.Partida{Data: "2026-10-28", Adversario: "Mirassol"}, true},
		{"mesmo adversário e placar, data divergente", banco.Partida{Data: "2012-05-02", Adversario: "Emelec", GolsPro: n(0), GolsContra: n(0)}, true},
		{"jogo de volta: mesmo adversário, outro placar", banco.Partida{Data: "2012-05-21", Adversario: "Vasco", GolsPro: n(1), GolsContra: n(0)}, false},
		{"outro adversário a 3 dias", banco.Partida{Data: "2026-10-27", Adversario: "Santos"}, false},
	}
	for _, c := range casos {
		if _, got := mesmaPartida(c.p, vizinhas); got != c.want {
			t.Errorf("%s: %v, esperava %v", c.nome, got, c.want)
		}
	}
}
