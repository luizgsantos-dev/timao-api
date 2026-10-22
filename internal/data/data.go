// Package data guarda os dados de seed do Corinthians, masculino e feminino:
// clube, ídolos, história e estádio. Os títulos vêm do timao.db (pacote acervo).
//
// Tudo fica em memória para o esqueleto continuar simples. Quando o projeto
// crescer, troque o Repo por uma implementação com SQLite ou Postgres
// mantendo os mesmos métodos.
package data

import "strings"

// Categoria separa o time masculino do feminino.
type Categoria string

const (
	Masculino Categoria = "masculino"
	Feminino  Categoria = "feminino"
)

type Clube struct {
	Nome         string   `json:"nome"`
	NomeCompleto string   `json:"nome_completo"`
	Fundacao     string   `json:"fundacao"`
	Cidade       string   `json:"cidade"`
	Estado       string   `json:"estado"`
	Cores        []string `json:"cores"`
	Apelidos     []string `json:"apelidos"`
	Torcida      string   `json:"torcida"`
	Mascote      string   `json:"mascote"`
	Estadios     []string `json:"estadios,omitempty"`
	Bordao       string   `json:"bordao"`
}

type Estadio struct {
	Nome            string `json:"nome"`
	Bairro          string `json:"bairro"`
	Cidade          string `json:"cidade"`
	Estado          string `json:"estado"`
	CEP             string `json:"cep"`
	CapacidadeAprox int    `json:"capacidade_aprox"`
	Inauguracao     int    `json:"inauguracao"`
}

type Idolo struct {
	Slug     string `json:"slug"`
	Nome     string `json:"nome"`
	Apelido  string `json:"apelido,omitempty"`
	Posicao  string `json:"posicao,omitempty"`
	Destaque string `json:"destaque"`
}

type Marco struct {
	Ano       int    `json:"ano"`
	Titulo    string `json:"titulo"`
	Descricao string `json:"descricao"`
}

// Time reúne os dados de uma categoria.
type Time struct {
	Clube    Clube
	Idolos   []Idolo
	Historia []Marco
}

type Repo struct {
	estadio Estadio
	times   map[Categoria]Time
}

// New devolve um Repo já preenchido com os dados de seed.
// Confira e complete os dados antes de publicar a API.
func New() *Repo {
	return &Repo{
		estadio: Estadio{
			Nome:            "Neo Química Arena",
			Bairro:          "Itaquera",
			Cidade:          "São Paulo",
			Estado:          "SP",
			CEP:             "08295005",
			CapacidadeAprox: 49000,
			Inauguracao:     2014,
		},
		times: map[Categoria]Time{
			Masculino: timeMasculino(),
			Feminino:  timeFeminino(),
		},
	}
}

func timeMasculino() Time {
	return Time{
		Clube: Clube{
			Nome:         "Corinthians",
			NomeCompleto: "Sport Club Corinthians Paulista",
			Fundacao:     "1910-09-01",
			Cidade:       "São Paulo",
			Estado:       "SP",
			Cores:        []string{"preto", "branco"},
			Apelidos:     []string{"Timão", "Coringão"},
			Torcida:      "Fiel",
			Mascote:      "Mosqueteiro",
			Bordao:       "Vai, Corinthians!",
		},
		Idolos: []Idolo{
			{Slug: "socrates", Nome: "Sócrates", Apelido: "Doutor", Posicao: "meia", Destaque: "Símbolo da Democracia Corintiana no início dos anos 1980."},
			{Slug: "rivellino", Nome: "Roberto Rivellino", Apelido: "Reizinho do Parque", Posicao: "meia", Destaque: "Ídolo dos anos 1960 e 1970, famoso pelo chute de curva."},
			{Slug: "neto", Nome: "Neto", Apelido: "Craque Neto", Posicao: "meia", Destaque: "Ídolo da Fiel nos anos 1990, craque de personalidade forte."},
			{Slug: "marcelinho-carioca", Nome: "Marcelinho Carioca", Apelido: "Pé de Anjo", Posicao: "meia", Destaque: "Especialista em faltas e figura marcante nos títulos dos anos 1990."},
			{Slug: "ronaldo", Nome: "Ronaldo", Apelido: "Fenômeno", Posicao: "atacante", Destaque: "Defendeu o Timão entre 2009 e 2011, com Paulista e Copa do Brasil em 2009."},
			{Slug: "cassio", Nome: "Cássio", Apelido: "Gigante", Posicao: "goleiro", Destaque: "Herói da Libertadores e do Mundial de 2012."},
		},
		Historia: []Marco{
			{Ano: 1910, Titulo: "Fundação", Descricao: "O Sport Club Corinthians Paulista nasce no bairro do Bom Retiro, em São Paulo."},
			{Ano: 1977, Titulo: "Fim do jejum", Descricao: "Título paulista de 1977 encerra uma longa seca de conquistas, com gol de Basílio."},
			{Ano: 1982, Titulo: "Democracia Corintiana", Descricao: "Jogadores e dirigentes decidem tudo no voto, e o time conquista o Paulista de 1982 e 1983."},
			{Ano: 1990, Titulo: "Primeiro Brasileirão", Descricao: "Corinthians conquista o primeiro título nacional da sua história."},
			{Ano: 2000, Titulo: "Primeiro Mundial", Descricao: "Campeão do primeiro Mundial de Clubes da FIFA, disputado no Brasil, diante do Vasco."},
			{Ano: 2012, Titulo: "Libertadores e Mundial", Descricao: "Campeão invicto da Libertadores e, meses depois, do Mundial, com vitória sobre o Chelsea."},
			{Ano: 2014, Titulo: "Casa nova", Descricao: "Inauguração da arena em Itaquera, a atual Neo Química Arena."},
		},
	}
}

// timeFeminino reúne os dados das Brabas. Fontes consultadas em maio de 2026:
// títulos de 2026 em diante não estão incluídos.
func timeFeminino() Time {
	return Time{
		Clube: Clube{
			Nome:         "Corinthians Feminino",
			NomeCompleto: "Sport Club Corinthians Paulista (futebol feminino)",
			Fundacao:     "1997", // ano de fundação do departamento; reaberto em 2016
			Cidade:       "São Paulo",
			Estado:       "SP",
			Cores:        []string{"preto", "branco"},
			Apelidos:     []string{"Brabas"},
			Torcida:      "Fiel",
			Mascote:      "Mosqueteiro",
			Estadios: []string{
				"Parque São Jorge (Estádio Alfredo Schürig)",
				"Neo Química Arena (jogos de maior apelo)",
			},
			Bordao: "Vai, Corinthians!",
		},
		Idolos: []Idolo{
			{Slug: "milene-domingues", Nome: "Milene Domingues", Posicao: "meia", Destaque: "Capitã aos 15 anos do time de futsal que deu origem ao futebol feminino do clube e principal jogadora dos primeiros anos, até 2001."},
			{Slug: "gabi-zanotti", Nome: "Gabi Zanotti", Posicao: "meia", Destaque: "Camisa 10 e capitã do time em 2026, no clube desde 2018. Eleita craque do Brasileirão Feminino em 2025."},
			{Slug: "vic-albuquerque", Nome: "Vic Albuquerque", Destaque: "Considerada, ao lado de Gabi Zanotti, a maior campeã da história do futebol feminino do clube."},
			{Slug: "lele", Nome: "Lelê", Posicao: "goleira", Destaque: "Decisiva na conquista da Libertadores de 2017, a primeira do futebol feminino do clube."},
		},
		Historia: []Marco{
			{Ano: 1994, Titulo: "Time de futsal de modelos", Descricao: "O Corinthians assume um time promocional de futsal formado por modelos, capitaneado por Milene Domingues, então com 15 anos."},
			{Ano: 1997, Titulo: "Fundação do departamento", Descricao: "É fundado o departamento de futebol feminino do clube."},
			{Ano: 2009, Titulo: "Time desativado", Descricao: "O departamento é desativado após mais de uma década de existência."},
			{Ano: 2016, Titulo: "Volta com o Audax", Descricao: "O futebol feminino é reaberto em janeiro, em parceria com o Grêmio Osasco Audax, e o time (Audax/Corinthians) conquista a Copa do Brasil."},
			{Ano: 2017, Titulo: "Primeira Libertadores", Descricao: "Audax/Corinthians é campeão da Libertadores Feminina, com o Colo-Colo como vice."},
			{Ano: 2018, Titulo: "Time próprio", Descricao: "Com o fim da parceria, o Corinthians assume o time com gestão própria, tendo Arthur Elias como técnico, e conquista o primeiro Brasileirão Feminino."},
			{Ano: 2019, Titulo: "Libertadores e primeiro Paulista", Descricao: "O time conquista a Libertadores e o primeiro Campeonato Paulista Feminino do clube."},
			{Ano: 2023, Titulo: "Ano de quatro títulos", Descricao: "Supercopa, Paulista, Brasileirão e Libertadores no mesmo ano. Arthur Elias é anunciado como técnico da Seleção Brasileira em setembro e deixa o clube ao fim da Libertadores."},
			{Ano: 2025, Titulo: "Tri da Libertadores e sétimo Brasileirão", Descricao: "O time vence a Libertadores pela terceira vez seguida (2023, 2024 e 2025) e conquista o sétimo Brasileirão Feminino, com vitória por 1 a 0 sobre o Cruzeiro na Neo Química Arena."},
		},
	}
}

func (r *Repo) Estadio() Estadio { return r.estadio }

func (r *Repo) Clube(c Categoria) Clube { return r.times[c].Clube }

func (r *Repo) Idolos(c Categoria) []Idolo {
	return append([]Idolo(nil), r.times[c].Idolos...)
}

// IdoloPorSlug busca um ídolo pelo slug, sem diferenciar maiúsculas de minúsculas.
func (r *Repo) IdoloPorSlug(c Categoria, slug string) (Idolo, bool) {
	for _, i := range r.times[c].Idolos {
		if strings.EqualFold(i.Slug, slug) {
			return i, true
		}
	}
	return Idolo{}, false
}

func (r *Repo) Historia(c Categoria) []Marco {
	return append([]Marco(nil), r.times[c].Historia...)
}
