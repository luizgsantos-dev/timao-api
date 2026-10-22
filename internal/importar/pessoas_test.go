package importar

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"timao-api/internal/banco"
)

func TestGravarPassagensDerivadas(t *testing.T) {
	ctx := context.Background()
	esc, err := banco.Criar(ctx, filepath.Join(t.TempDir(), "timao.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer esc.Descartar()
	fonte, err := esc.Fonte(ctx, banco.Fonte{Nome: "t", URL: "u", Licenca: "l", ColetadoEm: time.Now()})
	if err != nil {
		t.Fatal(err)
	}

	reg := novoRegistro(fonte)
	for _, p := range []banco.Pessoa{
		{Wikidata: "Q1", Nome: "Saiu Antes", WikiEN: "Saiu Antes"},
		{Wikidata: "Q2", Nome: "Ainda Aqui", WikiEN: "Ainda Aqui"},
		{Wikidata: "Q3", Nome: "Substituído", WikiEN: "Substituído"},
		{Wikidata: "Q4", Nome: "Já no Wikidata", WikiEN: "Já no Wikidata"},
		{Wikidata: "Q5", Nome: "Só na Categoria", WikiEN: "Só na Categoria"},
	} {
		if err := reg.gravarPessoa(ctx, esc, p); err != nil {
			t.Fatal(err)
		}
	}
	if err := reg.gravarPassagem(ctx, esc, reg.porQID["Q4"], banco.Passagem{Categoria: banco.Feminino, Funcao: "jogador"}, fonte); err != nil {
		t.Fatal(err)
	}

	aps := []aparicao{
		{"Saiu Antes", banco.Feminino, "jogador", 2022, fonte, false},
		{"Saiu Antes", banco.Feminino, "jogador", 2024, fonte, false},
		{"Ainda Aqui", banco.Feminino, "jogador", 2025, fonte, false},
		{"Ainda Aqui", banco.Feminino, "jogador", 2026, fonte, false},
		{"Substituído", banco.Feminino, "tecnico", 2026, fonte, true},
		{"Já no Wikidata", banco.Feminino, "jogador", 2026, fonte, false},
		{"Só na Categoria", banco.Feminino, "jogador", 0, fonte, false},
		{"Sem Pessoa", banco.Feminino, "jogador", 2026, fonte, false},
	}
	n, err := gravarPassagensDerivadas(ctx, esc, reg, aps, map[string]int{banco.Feminino: 2026})
	if err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Fatalf("esperava 4 passagens novas (não repete a do Wikidata nem cria sem pessoa), vieram %d", n)
	}
	esperadas := map[string]string{"Q1": "jogador", "Q2": "jogador", "Q3": "tecnico", "Q5": "jogador"}
	for q, funcao := range esperadas {
		if !reg.passagens[chavePassagem(reg.porQID[q], banco.Feminino, funcao)] {
			t.Errorf("faltou a passagem de %s como %s", q, funcao)
		}
	}
}

func TestPeriodoDaPassagem(t *testing.T) {
	// Confere as regras de início e fim lendo do banco depois de concluir.
	ctx := context.Background()
	destino := filepath.Join(t.TempDir(), "timao.db")
	esc, err := banco.Criar(ctx, destino)
	if err != nil {
		t.Fatal(err)
	}
	fonte, _ := esc.Fonte(ctx, banco.Fonte{Nome: "t", URL: "u", Licenca: "l", ColetadoEm: time.Now()})
	reg := novoRegistro(fonte)
	for _, q := range []string{"Q1", "Q2", "Q3"} {
		if err := reg.gravarPessoa(ctx, esc, banco.Pessoa{Wikidata: q, Nome: q, WikiEN: q}); err != nil {
			t.Fatal(err)
		}
	}
	aps := []aparicao{
		{"Q1", banco.Feminino, "jogador", 2022, fonte, false},
		{"Q1", banco.Feminino, "jogador", 2024, fonte, false},
		{"Q2", banco.Feminino, "jogador", 2026, fonte, false},
		{"Q3", banco.Feminino, "tecnico", 2024, fonte, false},
		{"Q3", banco.Feminino, "tecnico", 2026, fonte, true},
	}
	if _, err := gravarPassagensDerivadas(ctx, esc, reg, aps, map[string]int{banco.Feminino: 2026}); err != nil {
		t.Fatal(err)
	}
	if err := esc.Concluir(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	db, err := banco.Abrir(destino)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	want := map[string][2]string{"Q1": {"2022", "2024"}, "Q2": {"2026", ""}, "Q3": {"2024", "2026"}}
	rows, err := db.Query(`SELECT pe.wikidata, pa.inicio, COALESCE(pa.fim, '') FROM passagem pa JOIN pessoa pe ON pe.id = pa.pessoa_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var q, ini, fim string
		if err := rows.Scan(&q, &ini, &fim); err != nil {
			t.Fatal(err)
		}
		if w := want[q]; ini != w[0] || fim != w[1] {
			t.Errorf("%s: %s–%s, esperava %s–%s", q, ini, fim, w[0], w[1])
		}
	}
}
