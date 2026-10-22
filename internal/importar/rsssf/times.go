package rsssf

import (
	"regexp"
	"strings"
)

var (
	// "Corinthians (SP)", "Sergipe(SE)", "Corinthians (São Paulo)"
	reSufixoParenteses = regexp.MustCompile(`\s*\(([^()]*)\)\s*$`)
	// "Náutico-PE", "Atlético/MG", "Atlético MG"
	reSufixoUF = regexp.MustCompile(`(?:\s*[-/]\s*|\s+)([A-Za-z]{2})$`)
	ufs        = map[string]bool{"AC": true, "AL": true, "AP": true, "AM": true, "BA": true, "CE": true, "DF": true,
		"ES": true, "GO": true, "MA": true, "MT": true, "MS": true, "MG": true, "PA": true, "PB": true, "PR": true,
		"PE": true, "PI": true, "RJ": true, "RN": true, "RS": true, "RO": true, "RR": true, "SC": true, "SP": true,
		"SE": true, "TO": true}

	// Nomes que existem em mais de um estado: a sigla fica no nome, a não ser
	// quando é o estado do clube mais conhecido ("" = a sigla fica sempre).
	homonimos = map[string]string{
		"Atlético": "", "América": "", "Nacional": "", "Operário": "", "Ferroviário": "",
		"Democrata": "", "Rio Branco": "", "Independente": "", "São José": "", "Comercial": "",
		"Corinthians": "", "Coríntians": "", "Juventus": "", "Vila Nova": "", "São Raimundo": "",
		"Botafogo": "RJ", "Flamengo": "RJ", "Fluminense": "RJ", "Vasco": "RJ", "Vasco da Gama": "RJ",
		"Internacional": "RS", "Inter": "RS", "Grêmio": "RS", "Juventude": "RS", "Guarani": "SP",
		"Palmeiras": "SP", "Santos": "SP", "São Paulo": "SP", "Portuguesa": "SP", "Sport": "PE",
		"Santa Cruz": "PE", "Cruzeiro": "MG", "Paraná": "PR", "Coritiba": "PR", "Bahia": "BA",
	}
	// Abreviações das páginas do Paulista.
	apelidos = map[string]string{
		"Portuguesa/San":   "Portuguesa Santista",
		"XV de Novembro/P": "XV de Piracicaba",
		"XV de Novembro/J": "XV de Jaú",
		"Inter":            "Internacional",
		"Vasco":            "Vasco da Gama",
	}
	nomesCorinthians = map[string]bool{
		"corinthians": true, "corinthians paulista": true, "sc corinthians paulista": true,
		"sport club corinthians paulista": true, "sc corinthians p": true, "corinthians p": true,
		"corinthians sp": true,
	}
)

// separarUF separa a sigla do estado do nome, quando há: "Náutico-PE" ->
// ("Náutico", "PE"). Parênteses que não são sigla (cidade, apelido) também saem
// do nome e voltam em uf, para quem quiser conferir.
func separarUF(nome string) (base, uf string) {
	nome = strings.TrimSpace(reEspacos.ReplaceAllString(nome, " "))
	if m := reSufixoParenteses.FindStringSubmatchIndex(nome); m != nil && m[0] > 0 {
		return strings.TrimSpace(nome[:m[0]]), strings.TrimSpace(nome[m[2]:m[3]])
	}
	if m := reSufixoUF.FindStringSubmatchIndex(nome); m != nil && m[0] > 0 {
		s, sep := nome[m[2]:m[3]], nome[m[0]:m[2]]
		// "Atlético-Go" vale; "Fulano de" (minúsculas, sem hífen) não é sigla
		if ufs[strings.ToUpper(s)] && (s == strings.ToUpper(s) || strings.ContainsAny(sep, "-/")) {
			return strings.TrimSpace(nome[:m[0]]), strings.ToUpper(s)
		}
	}
	return nome, ""
}

// ehCorinthians diz se o nome é o Sport Club Corinthians Paulista. Ficam de fora
// os homônimos de outros estados e cidades: "Corinthians (AL)", "Corinthians
// Alagoano", "Corinthians/PP", "Corinthians (PR)", "Coríntians-RN" etc.
func ehCorinthians(nome string) bool {
	base, uf := separarUF(nome)
	if !nomesCorinthians[strings.ToLower(base)] {
		return false
	}
	switch strings.ToLower(uf) {
	case "", "sp", "são paulo", "sao paulo", "capital":
		return true
	}
	return false
}

// ehAnotacao diz se o conteúdo dos parênteses é uma anotação da página, e não
// parte do nome do clube (cidade ou sigla, como "Jaú" ou "SBC").
func ehAnotacao(s string) bool {
	s = strings.ToLower(s)
	return strings.Contains(s, "*") || strings.Contains(s, "played") || strings.Contains(s, "abandon") ||
		len(strings.Fields(s)) >= 3
}

// nomeAdversario limpa o nome: sem sigla do estado, salvo nos homônimos.
func nomeAdversario(nome string) string {
	nome = strings.TrimSpace(reEspacos.ReplaceAllString(nome, " "))
	if a, ok := apelidos[nome]; ok {
		return a
	}
	base, uf := separarUF(nome)
	if a, ok := apelidos[base]; ok {
		base = a
	}
	if uf == "" {
		return base
	}
	if !ufs[uf] {
		if ehAnotacao(uf) {
			return base // "(*)", "(played in Barueri)", "(Corinthians abandoned the league...)"
		}
		// parênteses com cidade ou apelido: "São Bento (SCS)"
		return base + " (" + uf + ")"
	}
	if padrao, ok := homonimos[base]; ok && padrao != uf {
		return base + "-" + uf
	}
	return base
}
