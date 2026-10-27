package server

// Schemas do OpenAPI. Eles precisam bater com o JSON que os handlers devolvem;
// o teste TestRespostasBatemComOSchema confere isso rota por rota.

type schema = map[string]any

func ref(nome string) schema { return schema{"$ref": "#/components/schemas/" + nome} }

func arr(itens schema) schema { return schema{"type": "array", "items": itens} }

func str(descricao string) schema { return schema{"type": "string", "description": descricao} }

func strEnum(descricao string, valores ...string) schema {
	return schema{"type": "string", "description": descricao, "enum": valores}
}

func inteiro(descricao string) schema { return schema{"type": "integer", "description": descricao} }

func booleano(descricao string) schema { return schema{"type": "boolean", "description": descricao} }

func dataHora(descricao string) schema {
	return schema{"type": "string", "format": "date-time", "description": descricao}
}

// obj monta um objeto fechado: só as propriedades listadas, e obrigatorias sempre presentes.
func obj(descricao string, props schema, obrigatorias ...string) schema {
	s := schema{"type": "object", "properties": props, "additionalProperties": false}
	if descricao != "" {
		s["description"] = descricao
	}
	if len(obrigatorias) > 0 {
		s["required"] = obrigatorias
	}
	return s
}

func componentes() schema {
	return schema{
		"Erro": obj("Todo erro sai neste formato.", schema{
			"erro": str("O que deu errado."),
			"dica": str("Como resolver, quando há uma dica."),
		}, "erro"),

		"Raiz": obj("", schema{
			"nome":   str("Nome da API."),
			"versao": str("Versão da API."),
			"bordao": str("Bordão do clube."),
			"rotas":  arr(str("Rota com um exemplo de uso.")),
		}, "nome", "versao", "bordao", "rotas"),
		"Saude": obj("", schema{"status": strEnum("", "ok")}, "status"),

		"Clube": obj("Dados gerais do clube ou do time feminino.", schema{
			"nome":          str(""),
			"nome_completo": str(""),
			"fundacao":      str("Data (AAAA-MM-DD) ou ano de fundação."),
			"cidade":        str(""),
			"estado":        str("Sigla da UF."),
			"cores":         arr(str("")),
			"apelidos":      arr(str("")),
			"torcida":       str(""),
			"mascote":       str(""),
			"estadios":      arr(str("Estádios onde o time manda os jogos (só no feminino).")),
			"bordao":        str(""),
		}, "nome", "nome_completo", "fundacao", "cidade", "estado", "cores", "apelidos", "torcida", "mascote", "bordao"),
		"Idolo": obj("", schema{
			"slug":     str("Identificador usado na URL."),
			"nome":     str(""),
			"apelido":  str(""),
			"posicao":  str(""),
			"destaque": str("Por que é ídolo(a)."),
		}, "slug", "nome", "destaque"),
		"Marco": obj("Um marco da história.", schema{
			"ano":       inteiro(""),
			"titulo":    str(""),
			"descricao": str(""),
		}, "ano", "titulo", "descricao"),
		"Estadio": obj("", schema{
			"nome":             str(""),
			"bairro":           str(""),
			"cidade":           str(""),
			"estado":           str(""),
			"cep":              str("Só dígitos."),
			"capacidade_aprox": inteiro(""),
			"inauguracao":      inteiro("Ano."),
		}, "nome", "bairro", "cidade", "estado", "cep", "capacidade_aprox", "inauguracao"),
		"Endereco": obj("Endereço da arena, vindo da Brasil API.", schema{
			"estadio": str(""),
			"endereco": obj("", schema{
				"cep":          str(""),
				"state":        str("UF."),
				"city":         str(""),
				"neighborhood": str(""),
				"street":       str(""),
				"location":     schema{"description": "Coordenadas, no formato da Brasil API."},
			}, "cep", "state", "city", "neighborhood", "street"),
		}, "estadio", "endereco"),
		"Feriados": obj("", schema{
			"ano": inteiro(""),
			"feriados": arr(obj("", schema{
				"data":       str("AAAA-MM-DD."),
				"nome":       str(""),
				"tipo":       str("Tipo segundo a Brasil API (ex.: national)."),
				"dia_semana": str("Ex.: segunda-feira."),
			}, "data", "nome", "tipo", "dia_semana")),
		}, "ano", "feriados"),
		"Calendario": obj("Jogos do calendário (arquivo JOGOS_FILE), cruzados com os feriados nacionais.", schema{
			"jogos": arr(obj("", schema{
				"data":           str("AAAA-MM-DD."),
				"horario":        str("HH:MM, horário de Brasília."),
				"adversario":     str(""),
				"competicao":     str(""),
				"mando":          strEnum("", "casa", "fora"),
				"categoria":      strEnum("", "masculino", "feminino"),
				"local":          str(""),
				"dia_semana":     str(""),
				"fim_de_semana":  booleano(""),
				"cai_em_feriado": booleano("Ausente quando a Brasil API não respondeu."),
				"feriado":        str("Nome do feriado, quando cai em um."),
			}, "data", "adversario", "competicao", "mando", "dia_semana", "fim_de_semana")),
			"aviso": str("Presente quando os feriados não puderam ser consultados."),
		}, "jogos"),

		"Fonte": obj("De onde veio o dado. A Wikipédia (CC BY-SA) exige crédito.", schema{
			"nome":    str(""),
			"url":     str("Para a Wikipédia, o link da revisão exata lida."),
			"licenca": str("Ex.: CC0 1.0, CC BY-SA 4.0, MIT."),
		}, "nome", "url", "licenca"),
		"Passagem": obj("Passagem pelo clube na categoria.", schema{
			"funcao": strEnum("", "jogador", "tecnico"),
			"inicio": str("AAAA, AAAA-MM ou AAAA-MM-DD, conforme a precisão da fonte."),
			"fim":    str("Idem; ausente se a passagem está aberta ou não tem data."),
			"jogos":  inteiro(""),
			"gols":   inteiro(""),
		}, "funcao"),
		"ElencoDaPessoa": obj("", schema{
			"temporada": inteiro(""),
			"numero":    inteiro("Número da camisa."),
			"posicao":   str(""),
			"jogos":     inteiro("Jogos na temporada, quando a fonte tem estatísticas."),
			"gols":      inteiro(""),
		}, "temporada"),
		"Pessoa": obj("Jogador(a) ou técnico(a).", schema{
			"wikidata":         str("Código do Wikidata, usado na URL. Ex.: Q102331."),
			"nome":             str("Nome de uso, o mesmo das escalações e dos gols. Ex.: Dida."),
			"nome_completo":    str("Quando difere do nome de uso. Ex.: Nelson de Jesus Silva."),
			"genero":           strEnum("", "masculino", "feminino"),
			"nascimento":       str("AAAA, AAAA-MM ou AAAA-MM-DD."),
			"falecimento":      str("Idem."),
			"nacionalidade":    arr(str("")),
			"posicao":          arr(str("")),
			"foto_url":         str("Wikimedia Commons."),
			"wikipedia_pt":     str(""),
			"wikipedia_en":     str(""),
			"passagens":        arr(ref("Passagem")),
			"gols_em_partidas": inteiro("Só no detalhe: gols nas partidas do acervo (exige artigo na Wikipédia em inglês). Inclui amistosos e temporadas sem elenco publicado, então pode passar da soma dos elencos."),
			"elencos":          arr(ref("ElencoDaPessoa")),
			"fonte":            ref("Fonte"),
		}, "wikidata", "nome", "passagens", "fonte"),
		"ListaPessoas": obj("", schema{
			"total":   inteiro("Total de pessoas no filtro, em todas as páginas."),
			"pagina":  inteiro(""),
			"limite":  inteiro(""),
			"pessoas": arr(ref("Pessoa")),
		}, "total", "pagina", "limite", "pessoas"),
		"Placar": obj("Placar visto pelo Corinthians.", schema{
			"corinthians":          inteiro(""),
			"adversario":           inteiro(""),
			"penaltis_corinthians": inteiro("Só quando houve disputa de pênaltis."),
			"penaltis_adversario":  inteiro(""),
		}, "corinthians", "adversario"),
		"JogoResumido": obj("", schema{
			"data":       str("AAAA-MM-DD."),
			"adversario": str(""),
			"competicao": str(""),
		}, "data", "adversario"),
		"Gol": obj("", schema{
			"minuto":   str("Ex.: 45+2."),
			"autor":    str(""),
			"wikidata": str("Da pessoa autora, quando ligada (nunca em gol contra)."),
			"tipo":     strEnum("", "normal", "penalti", "contra"),
			"time":     strEnum("Quem ganhou o gol.", "corinthians", "adversario"),
			"partida":  ref("JogoResumido"),
		}, "tipo", "time"),
		"Partida": obj("", schema{
			"data":       str("AAAA-MM-DD; identifica a partida na URL."),
			"horario":    str("HH:MM, como na fonte."),
			"temporada":  inteiro(""),
			"competicao": str(""),
			"fase":       str(""),
			"adversario": str(""),
			"mando":      strEnum("casa = Corinthians listado como mandante; neutro = competição em sede única.", "casa", "fora", "neutro"),
			"placar":     ref("Placar"),
			"resultado":  strEnum("Pelo tempo normal e prorrogação; pênaltis não mudam.", "vitoria", "empate", "derrota"),
			"estadio":    str(""),
			"cidade":     str(""),
			"publico":    inteiro(""),
			"arbitro":    str(""),
			"gols":       arr(ref("Gol")),
			"fonte":      ref("Fonte"),
		}, "data", "temporada", "adversario", "mando", "fonte"),
		"Resumo": obj("Desempenho nas partidas já disputadas.", schema{
			"jogos":       inteiro(""),
			"vitorias":    inteiro(""),
			"empates":     inteiro(""),
			"derrotas":    inteiro(""),
			"gols_pro":    inteiro(""),
			"gols_contra": inteiro(""),
		}, "jogos", "vitorias", "empates", "derrotas", "gols_pro", "gols_contra"),
		"ListaPartidas": obj("", schema{
			"total":    inteiro(""),
			"pagina":   inteiro(""),
			"limite":   inteiro(""),
			"resumo":   ref("Resumo"),
			"partidas": arr(ref("Partida")),
		}, "total", "pagina", "limite", "resumo", "partidas"),
		"GolsDaPessoa": obj("", schema{
			"total": inteiro(""),
			"gols":  arr(ref("Gol")),
		}, "total", "gols"),
		"Artilharia": obj("", schema{
			"aviso": str(""),
			"artilharia": arr(obj("", schema{
				"nome":     str(""),
				"wikidata": str(""),
				"gols":     inteiro(""),
				"penaltis": inteiro(""),
			}, "nome", "gols", "penaltis")),
		}, "aviso", "artilharia"),
		"Temporadas": obj("", schema{
			"temporadas": arr(obj("", schema{
				"temporada": inteiro(""),
				"resumo":    ref("Resumo"),
				"fontes":    arr(ref("Fonte")),
			}, "temporada", "resumo", "fontes")),
		}, "temporadas"),
		"Elenco": obj("", schema{
			"temporada":              inteiro(""),
			"temporadas_disponiveis": arr(inteiro("")),
			"elenco": arr(obj("", schema{
				"nome":          str(""),
				"wikidata":      str("Quando a pessoa está no acervo."),
				"numero":        inteiro(""),
				"posicao":       str(""),
				"nacionalidade": str("Código FIFA, ex.: BRA."),
				"jogos":         inteiro("Jogos na temporada, quando a fonte tem estatísticas."),
				"gols":          inteiro(""),
			}, "nome")),
			"fonte": ref("Fonte"),
		}, "temporada", "temporadas_disponiveis", "elenco"),
		"Conquista": obj("Títulos de uma competição.", schema{
			"competicao": str(""),
			"slug":       str("Identificador usado na URL; o mesmo nas duas categorias."),
			"escopo":     strEnum("", "mundial", "continental", "nacional", "interestadual", "estadual", "internacional", "amistoso", "outros"),
			"oficial":    booleano(""),
			"quantidade": inteiro(""),
			"anos":       arr(inteiro("")),
			"notas": arr(obj("Nota de rodapé da fonte, no idioma dela.", schema{
				"ano":   inteiro(""),
				"texto": str(""),
			}, "ano", "texto")),
			"edicoes": arr(obj("Só no detalhe.", schema{
				"ano":          inteiro(""),
				"wikipedia_en": str(""),
				"nota":         str(""),
			}, "ano")),
			"fonte": ref("Fonte"),
		}, "competicao", "slug", "escopo", "oficial", "quantidade", "anos", "fonte"),
		"Titulos": obj("", schema{
			"total":   inteiro("Soma das quantidades."),
			"titulos": arr(ref("Conquista")),
		}, "total", "titulos"),
		"StatusAcervo": obj("", schema{
			"disponivel":             booleano("Se há banco em uso."),
			"arquivo":                str(""),
			"gerado_em":              dataHora(""),
			"atualizacao_automatica": booleano(""),
			"periodo":                str("Ex.: 168h0m0s."),
			"proxima_atualizacao":    dataHora(""),
			"atualizando":            booleano(""),
			"ultima_tentativa":       dataHora(""),
			"ultimo_erro":            str(""),
		}, "disponivel", "atualizacao_automatica", "atualizando"),
	}
}
