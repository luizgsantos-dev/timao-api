package data

import "testing"

var categorias = []Categoria{Masculino, Feminino}

func TestSlugsDeIdolosUnicosPorCategoria(t *testing.T) {
	r := New()
	for _, c := range categorias {
		vistos := map[string]bool{}
		for _, i := range r.Idolos(c) {
			if vistos["idolo:"+i.Slug] {
				t.Errorf("%s: slug de ídolo repetido: %s", c, i.Slug)
			}
			vistos["idolo:"+i.Slug] = true
		}
	}
}

func TestHistoriaEmOrdemCronologica(t *testing.T) {
	r := New()
	for _, c := range categorias {
		marcos := r.Historia(c)
		if len(marcos) == 0 {
			t.Errorf("%s: história vazia", c)
		}
		for i := 1; i < len(marcos); i++ {
			if marcos[i].Ano < marcos[i-1].Ano {
				t.Errorf("%s: marco de %d vem depois de %d", c, marcos[i].Ano, marcos[i-1].Ano)
			}
		}
	}
}

func TestCategoriasSaoIndependentes(t *testing.T) {
	r := New()
	if r.Clube(Masculino).Nome == r.Clube(Feminino).Nome {
		t.Fatal("masculino e feminino não deveriam ter o mesmo nome de clube")
	}
	if _, ok := r.IdoloPorSlug(Masculino, "gabi-zanotti"); ok {
		t.Fatal("ídola do feminino apareceu no masculino")
	}
	if _, ok := r.IdoloPorSlug(Feminino, "socrates"); ok {
		t.Fatal("ídolo do masculino apareceu no feminino")
	}
}

func TestSlugSemDiferenciarMaiusculas(t *testing.T) {
	r := New()
	if _, ok := r.IdoloPorSlug(Feminino, "GABI-ZANOTTI"); !ok {
		t.Error("IdoloPorSlug deveria ignorar maiúsculas")
	}
}
