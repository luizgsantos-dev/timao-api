# Pesquisa de fontes de dados

Levantamento feito em 22/10/2026, consultando cada fonte diretamente. Os números são do dia da consulta.

## Resumo

| Fonte | Masculino | Feminino | Licença | Acesso |
|---|---|---|---|---|
| Wikidata | Forte: 835 jogadores, 67 títulos, técnicos | Fraco: ~38 jogadoras, 12 títulos | CC0 | SPARQL, sem chave |
| Wikipédia EN (páginas de temporada) | Partidas de 2008, 2012, 2013 e 2015 a 2026 | **Partidas de 2022 a 2026** | CC BY-SA (citar) | API, sem chave |
| Wikipédia PT | Treinadores, artilheiros, presidentes, recordes, públicos, clássicos | Página do time e lista de temporadas | CC BY-SA (citar) | API, sem chave |
| openfootball | Brasileirão 2018–2026, Libertadores e Sul-Americana 2012–2026, Copa do Brasil 2025 | Nada | CC0 | Arquivos no GitHub |
| Pacote brasileirao (williamorim) | Brasileirão 2003–2026 (data, mando, placar) | Nada | MIT | CSV no GitHub |
| adaoduque/Brasileirao_Dataset | Brasileirão 2003–2024 com gols, cartões e estatísticas | Nada | **Sem licença** | Não dá para redistribuir |
| football-data.org | Brasileirão Série A no plano grátis | Nada | Termos próprios | Chave grátis (cadastro) |
| ESPN (API não oficial) | Agenda e resultados: Brasileirão, Copa do Brasil, Libertadores, Paulista | Só a Copa dos Campeões feminina da FIFA | Não documentada | Sem chave, pode mudar sem aviso |

## Wikidata

- Itens: masculino `Q35933`, feminino `Q28681033`.
- Masculino: 835 jogadores (`P54`), 825 com posição e 320 com foto. Dos 927 vínculos, 665 têm data de início e cerca de 475 têm jogos e gols (`P1350`/`P1351`). Técnicos: 15 (`P286`). Títulos: 67 competições com o clube como vencedor (`P1346`).
- Feminino: 12 jogadoras ligadas ao item do time feminino e **26 ligadas por engano ao item do masculino**. A importação precisa filtrar por gênero (`P21`). Títulos: 12.
- Temporadas (`P5138`) existem para os dois times, mas com pouco conteúdo estruturado.

## Wikipédia

- As páginas em inglês "AAAA Sport Club Corinthians Paulista season" e "(women) season" usam a predefinição `{{football box collapsible}}`. Cada partida traz data, hora, adversário, placar, gols com minuto, estádio, cidade, público, árbitro e resultado. São 53 partidas em 2025 no feminino e 74 no masculino.
- **É a melhor fonte de partidas do feminino.** Nenhuma API esportiva cobre as Brabas.
- As páginas em português têm tabelas ricas (treinadores, artilheiros com 50+ gols, presidentes, maiores públicos, recordes, confrontos por rival), mas em formatos variados. Dá para extrair, com mais trabalho e mais risco de quebrar.

## Atualização: mais partidas (22/10/2026)

- **openfootball/south-america** (CC0), em texto Football.TXT: só cobre anos que a Wikipédia já tem, e sem gols. Serve para completar jogos que faltam e conferir placares: das 470 partidas do Corinthians, 467 bateram com o banco e só uma tinha placar diferente. A versão em JSON (`football.json`) só tem 2019, 2020, 2025 e 2026.
- **Pacote brasileirao** (MIT): todos os jogos do Brasileirão desde 2003. Trouxe 400 partidas, a maior parte de anos sem página de temporada na Wikipédia. Nos 494 jogos em comum com a Wikipédia, nenhum placar divergiu.
- **Páginas de temporada em português:** só existem para 2023–2025, já cobertos pelas páginas em inglês.
- **Feminino antes de 2022:** nenhuma fonte aberta com jogos foi encontrada.

## Atualização: Brabas (22/10/2026)

- **Páginas de edição das competições femininas** (Wikipédia em inglês): as caixas de jogo trazem 62 partidas que faltavam, de 2001 e 2016–2021 (Libertadores 2017, 2019–2021; mata-mata do Brasileirão 2020–2021 e do Paulista 2016–2021). Nas 78 que já existiam, só um placar divergiu.
- **Elencos** ({{Fs player}}) das páginas de temporada: cerca de 32 jogadoras por temporada de 2022 a 2026, todas com artigo.
- **Categoria de jogadoras das Brabas:** 86 artigos (o Wikidata tinha 41 ligadas ao clube). Com elencos e categorias, as jogadoras com passagem foram para 98.
- **Técnicos(as):** 3 na categoria das Brabas (Arthur Elias, Lucas Piccinato, Emily Lima) e 95 na do masculino (o Wikidata tinha 34).
- **Páginas em português** das competições femininas não usam caixas de jogo legíveis, e a tabela "List of SC Corinthians Paulista (women) seasons" só tem o resumo de cada temporada.
- **Limite de requisições:** a Wikipédia responde 429 quando muitas páginas são pedidas em sequência. O importador espera entre páginas e repete quando recebe 429.

## Atualização: segunda rodada (22/10/2026)

- **RSSSF e RSSSF Brasil:** termos "You are free to copy this document in whole or part provided that proper acknowledgement is given to the author", com "All rights reserved". Usado com crédito ao autor de cada página (repositório privado); antes de abrir a API ao público, vale pedir autorização ao RSSSF. Rendeu cerca de 2.600 partidas do masculino de 1913 a 2014, com gols onde a página traz.
- **Páginas de edição na Wikipédia:** as subpáginas da Libertadores em inglês ("2010 Copa Libertadores second stage") têm caixas de jogo completas; as páginas em português do Paulista e da Copa do Brasil trazem o mata-mata em {{footballbox}} com campos em português. Rendeu cerca de 170 partidas nas duas categorias.
- **Elencos do masculino:** as páginas de 2013 e 2015–2019 usam {{Fs player}} numa subseção ("===Squad===" dentro de "Background"); 2008 e 2015–2019 têm a tabela "Squad statistics" com jogos e gols por jogador.
- **Títulos recentes:** o quadro de informações das páginas de edição traz o campeão (campos champion, champion_other, winners ou campeão). O Wikidata (P1346) está quase vazio nas edições recentes.
- **Correção:** o Paulista Feminino de 2025 foi do Palmeiras (5 x 2 no agregado); o Corinthians venceu só a volta.
- **Sem saída aberta:** CBF (termos proíbem reprodução), Federação Paulista (bloqueia acesso automático), elencos e técnicos das Brabas antes de 2018.

## Descartadas

- **ogol, Transfermarkt, Sofascore, site oficial:** os termos de uso proíbem coleta automática.
- **adaoduque/Brasileirao_Dataset:** sem licença, então todos os direitos são reservados. Só dá para usar com autorização do autor.
- **API-Football:** boa cobertura, mas 100 requisições/dia e exige cadastro. Fica como alternativa.
