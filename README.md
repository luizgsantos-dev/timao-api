# timao-api

API em Go com endpoints sobre o Sport Club Corinthians Paulista, masculino e feminino (as Brabas). O servidor usa a biblioteca padrão (`net/http` com o roteador do Go 1.22+) mais um driver SQLite em Go puro (`modernc.org/sqlite`, sem CGO) e consome a [Brasil API](https://brasilapi.com.br) para dados complementares (CEP e feriados). Os dados históricos vêm de fontes abertas e são importados para um arquivo SQLite (veja [Banco de dados](#banco-de-dados)).

## Como rodar

```bash
go run ./cmd/api          # sobe em :8080
go test ./...             # roda os testes

# opcional
ADDR=":9090" go run ./cmd/api
BRASILAPI_URL="https://brasilapi.com.br" go run ./cmd/api
JOGOS_FILE="meus-jogos.json" go run ./cmd/api   # seu calendário (sem isso, usa o de exemplo)
DB_FILE="dados/timao.db" go run ./cmd/api        # outro caminho para o banco (padrão: ./timao.db)
ATUALIZAR_A_CADA=24h go run ./cmd/api            # atualiza o banco todo dia (padrão: 168h, uma semana; 0 desliga)

# com Docker
docker build -t timao-api .
docker run -d -p 8080:8080 -v timao-dados:/dados --name timao-api timao-api
```

Documentação navegável (Swagger UI): <http://localhost:8080/docs>. O documento OpenAPI 3.1 fica em `/openapi.json`.

Teste rápido:

```bash
curl localhost:8080/
curl localhost:8080/v1/idolos/aleatorio
curl "localhost:8080/v1/titulos?escopo=nacional"
curl localhost:8080/v1/estadio/endereco
curl "localhost:8080/v1/jogos/proximos?limite=3"
curl localhost:8080/v1/feminino/idolas/aleatoria
curl "localhost:8080/v1/feminino/titulos?escopo=continental"
curl "localhost:8080/v1/feminino/jogos/proximos?limite=3"
```

## Banco de dados

O `timao.db` é um arquivo SQLite gerado do zero pelo importador, a partir de fontes abertas. Não há servidor de banco: o arquivo tem menos de 1 MB.

```bash
go run ./cmd/importar        # gera ./timao.db (leva uns 10 minutos, com pausas para não sobrecarregar a Wikipédia)
go run ./cmd/importar -v     # mostra também as caixas de jogo ignoradas
```

| Fonte | O que vem dela | Licença |
|-------|----------------|---------|
| [Wikidata](https://www.wikidata.org) | Jogadores(as), técnicos(as) e passagens (período, jogos, gols); detalhes (nascimento, foto, posição) de quem vem das fontes abaixo | CC0 |
| Wikipédia em inglês, seção "Honours" de [SC Corinthians Paulista](https://en.wikipedia.org/wiki/SC_Corinthians_Paulista) e [SC Corinthians Paulista (women)](https://en.wikipedia.org/wiki/SC_Corinthians_Paulista_(women)) | Títulos oficiais e não oficiais, uma linha por edição, com as notas de rodapé (ex.: a parceria com o Audax) | CC BY-SA 4.0 |
| [Wikipédia em inglês](https://en.wikipedia.org), páginas de temporada | Partidas (2008, 2012, 2013, 2015–2026 no masculino; 2022–2026 no feminino) com placar, gols e minutos, estádio, público e árbitro | CC BY-SA 4.0 |
| [openfootball/south-america](https://github.com/openfootball/south-america) e [openfootball/club-worldcup](https://github.com/openfootball/club-worldcup) | Brasileirão (2018 em diante), Copa do Brasil (2025), Libertadores e Sul-Americana (2012 em diante) e Mundial de Clubes (2000 e 2005 em diante): placar, fase e horário. Completa jogos que faltam e confere placares | CC0 |
| Wikipédia em inglês, páginas de temporada | Elenco de cada temporada (número, posição, nacionalidade) e técnico(a) do quadro de informações | CC BY-SA 4.0 |
| Wikipédia em inglês, categorias de [jogadoras das Brabas](https://en.wikipedia.org/wiki/Category:SC_Corinthians_Paulista_(women)_players), [técnicos(as) das Brabas](https://en.wikipedia.org/wiki/Category:SC_Corinthians_Paulista_(women)_managers) e [técnicos do masculino](https://en.wikipedia.org/wiki/Category:SC_Corinthians_Paulista_managers) | Quem passou pelo clube e tem artigo, mesmo sem estar no Wikidata | CC BY-SA 4.0 |
| Wikipédia em inglês e em português, páginas de edição das competições (Libertadores e subpáginas de fase, Sul-Americana, Recopa, Mundial, Copa do Brasil, Paulista, Brasileirão Feminino, Supercopa) | Jogos dos anos sem página de temporada, nas duas categorias, com gols: em geral o mata-mata e as fases de grupos das copas continentais | CC BY-SA 4.0 |
| [RSSSF](https://www.rsssf.org) e [RSSSF Brasil](https://www.rsssfbrasil.com): Brasileirão (2003–2014), Copa do Brasil (1989–2014) e Paulista (1902–2014), por ano | Jogos do masculino desde 1913, com gols quando a página traz. Cada página credita o seu autor, e a API repete o crédito na `fonte` de cada partida | Cópia permitida com crédito ao autor; demais direitos reservados |
| [Pacote brasileirao](https://github.com/williamorim/brasileirao), de William Amorim | Brasileirão de 2003 em diante: data, mando e placar. Hoje serve para conferir os placares do RSSSF (894 jogos em comum, nenhum divergente) | MIT |

Tabelas: `pessoa`, `passagem`, `titulo`, `partida`, `gol`, `fonte` e `meta`. Toda linha aponta para a `fonte` de onde veio. No caso da Wikipédia, a fonte é o link da revisão exata lida, para dar crédito (exigido pela CC BY-SA) e permitir conferência. O esquema está em [internal/banco/schema.sql](internal/banco/schema.sql) e a pesquisa das fontes em [docs/pesquisa-fontes.md](docs/pesquisa-fontes.md).

A ordem da tabela é a de prioridade: quando duas fontes trazem a mesma partida, vale a primeira, que é a mais rica. A fonte seguinte não sobrescreve nada, mas completa: se a partida ainda não tem gols e o placar das duas bate, os gols dela entram (só se somam o placar); horário, fase, estádio, público e árbitro vazios são preenchidos. A mesma partida é reconhecida quando há outra da categoria a até 1 dia (o Corinthians não joga em dias seguidos), ou contra o mesmo adversário a até 5 dias, sem placar ou com o mesmo placar (datas provisórias de jogos futuros). Quando as fontes discordam do placar, a primeira vale e a divergência aparece no `-v` do importador.

O importador corrige sozinho alguns problemas das fontes: separa as jogadoras que o Wikidata ligou ao item do time masculino, une técnicos cadastrados duas vezes, ignora títulos da base e vices, traduz competições e fases, reconcilia gols contra listados do lado errado e corrige anos digitados errado nas páginas de temporada (a página de 2012, por exemplo, data a Libertadores de 2012 como 2011; o openfootball confirmou os 10 placares depois da correção). O que a fonte erra de verdade (placar diferente da lista de gols, por exemplo) entra como está.

Limites conhecidos:

- **Títulos:** a base é a tabela da Wikipédia, que pode demorar a ser atualizada depois de uma conquista. Para não esperar, o importador lê também o campeão no quadro de informações das páginas das edições do ano atual e do anterior; um título achado ali e ainda ausente da tabela entra com uma nota dizendo isso. As notas de rodapé vêm no idioma da fonte (inglês).
- **Partidas do masculino:** de 1913 a 2026. Antes de 2003 vêm quase só do Paulista e da Copa do Brasil (RSSSF), com Libertadores, Mundial e afins pelas páginas de edição; gols só onde a página do RSSSF traz (falta no Paulista antigo, na Copa do Brasil de 2002 e no Brasileirão de 2014). Jogos que a fonte lista sem data (123 no RSSSF, quase todos da Copa do Brasil antes de 2000) ficam de fora. As páginas do Paulista no RSSSF trazem também as divisões de baixo, onde joga outro "Corinthians" (o de Presidente Prudente); a leitura para no começo delas.
- **Partidas das Brabas:** 2022 a 2026 por completo; de 2016 a 2021 (e um jogo de 2001) só os jogos que aparecem em caixas de jogo das páginas das competições, que costumam ser o mata-mata e a Libertadores. As fases de pontos corridos dessas páginas vêm em tabela de resultados, sem data, e ficam de fora. A única fonte com tudo (CBF) proíbe reprodução nos termos de uso.
- **Pessoas e elencos:** quem aparece num elenco ou numa categoria sem passagem no Wikidata ganha uma passagem calculada pelas temporadas em que aparece (da primeira à última; aberta se está na mais recente). Integrantes de elenco sem item no Wikidata ficam só no elenco, sem página de pessoa. Quando a página de temporada tem a tabela de estatísticas ("Squad statistics"), cada integrante do elenco ganha os jogos e gols da temporada, e quem só aparece ali (quem saiu no meio do ano) entra também. Não há elencos das Brabas antes de 2022 em fonte aberta.
- **Divergências conhecidas:** em 2013, a Wikipédia dá Corinthians 2 x 0 Tijuana, mas lista 3 gols, e o openfootball dá 3 x 0. A página de 2008 tem dois jogos em dias seguidos (Grêmio Barueri em 06/02 e Paulista em 07/02). Em 21/09/2026, a página de temporada das Brabas dá 0 x 1 no Bahia e lista um gol do Corinthians; a página do Brasileirão Feminino dá 1 x 0.
- **Autores de gols:** cerca de 38% dos gols do Corinthians têm ligação com uma pessoa do Wikidata (via artigo da Wikipédia em inglês). Os demais ficam só com o nome.

### Atualização automática

A API mantém o banco em dia sozinha, sem cron nem agendador do sistema: a cada `ATUALIZAR_A_CADA` (padrão: uma semana, contada a partir de quando o banco atual foi gerado) ela roda a importação em segundo plano, com a API no ar.

1. O banco novo é gravado num arquivo à parte, ao lado do principal: `timao-20261022T110200.db`. Nada é sobrescrito enquanto está aberto, o que o Windows não permitiria.
2. Ele é conferido: se alguma contagem (pessoas, passagens, títulos, partidas de cada categoria, gols, elenco) ficar abaixo de 90% da do banco atual, ele é recusado. Isso protege contra uma fonte fora do ar ou uma página vandalizada.
3. Aprovado, ele passa a ser usado na hora. O anterior fica aberto por mais um minuto, para as requisições em andamento terminarem, e o arquivo é apagado na atualização seguinte. O banco principal (`timao.db`) nunca é apagado.
4. Se a importação falhar, o banco atual continua no ar e a API tenta de novo em até 6 horas.

Ao subir, a API usa o mais recente entre o banco principal e essas gerações. Sem nenhum banco, ela sobe mesmo assim: as rotas do acervo respondem 503 com a dica de como gerar o arquivo, as demais funcionam normalmente, e a primeira importação começa na hora.

`GET /v1/acervo` mostra o arquivo em uso, quando ele foi gerado, a próxima atualização, se há uma em andamento e o erro da última tentativa, se houve.

O comando `go run ./cmd/importar` continua servindo para gerar o banco à mão. No Windows, rode-o com a API parada, porque ele substitui o `timao.db`, que fica aberto enquanto a API usa esse arquivo.

## Rotas

| Rota | O que devolve |
|------|---------------|
| `GET /` | Nome, versão, bordão e lista de rotas |
| `GET /healthz` | Health check |
| `GET /docs` | Documentação navegável (Swagger UI), nas cores do clube |
| `GET /openapi.json` | Documento OpenAPI 3.1 de todas as rotas |
| `GET /v1/acervo` | Situação do banco: arquivo em uso, quando foi gerado e quando será atualizado |
| `GET /v1/clube` | Dados gerais do clube |
| `GET /v1/idolos` | Lista de ídolos |
| `GET /v1/idolos/aleatorio` | Um ídolo sorteado |
| `GET /v1/idolos/{slug}` | Um ídolo específico (ex.: `socrates`) |
| `GET /v1/historia` | Marcos históricos em ordem cronológica |
| `GET /v1/estadio` | Dados da Neo Química Arena |
| `GET /v1/estadio/endereco` | Endereço completo da arena, via CEP na Brasil API |
| `GET /v1/feriados?ano=2026` | Feriados nacionais do ano, com dia da semana, via Brasil API |
| `GET /v1/jogos?competicao=` | Calendário completo, com dia da semana e se o jogo cai em feriado |
| `GET /v1/jogos/proximos?limite=` | Jogos de hoje em diante (padrão 5), com o mesmo cruzamento |

### Futebol feminino (as Brabas)

Mesma ideia do masculino, com o prefixo `/v1/feminino` e "ídolas" no feminino:

| Rota | O que devolve |
|------|---------------|
| `GET /v1/feminino/clube` | Dados gerais do time feminino |
| `GET /v1/feminino/idolas` | Lista de ídolas |
| `GET /v1/feminino/idolas/aleatoria` | Uma ídola sorteada |
| `GET /v1/feminino/idolas/{slug}` | Uma ídola específica (ex.: `gabi-zanotti`) |
| `GET /v1/feminino/historia` | Marcos históricos, de 1994 a 2025 |
| `GET /v1/feminino/jogos?competicao=` | Calendário feminino, com dia da semana e feriado |
| `GET /v1/feminino/jogos/proximos?limite=` | Próximos jogos femininos |

`/v1/estadio`, `/v1/estadio/endereco` e `/v1/feriados` são compartilhados.

### Acervo (vem do `timao.db`)

As mesmas rotas existem nas duas categorias: troque `/v1` por `/v1/feminino` para as Brabas.

| Rota | O que devolve |
|------|---------------|
| `GET /v1/pessoas` | Jogadores(as) e técnicos(as) com passagens pelo clube. Filtros: `funcao` (`jogador`, `tecnico`), `nome` e `posicao` (busca parcial, `nome` ignora acentos) |
| `GET /v1/pessoas/{wikidata}` | Uma pessoa pelo código do Wikidata (ex.: `Q102331`, Sócrates), com os gols registrados nas partidas do acervo |
| `GET /v1/pessoas/{wikidata}/gols` | Cada gol da pessoa pelo Corinthians: data, adversário, competição e minuto |
| `GET /v1/partidas` | Partidas, das mais recentes para as mais antigas, com um `resumo` (vitórias, empates, derrotas, gols) do filtro inteiro. Filtros: `temporada`, `competicao`, `adversario`, `mando` (`casa`, `fora`), `resultado` (`vitoria`, `empate`, `derrota`), `de` e `ate` (AAAA-MM-DD), `ordem` (`asc`, `desc`) |
| `GET /v1/partidas/{data}` | A partida do dia (AAAA-MM-DD), com os gols |
| `GET /v1/artilharia` | Artilharia nas partidas do acervo. Filtros: `temporada`, `competicao`, `limite` |
| `GET /v1/temporadas` | Temporadas com partidas no acervo e o resumo de cada uma |
| `GET /v1/elenco` | Elenco de uma temporada (`temporada`; padrão: a mais recente), com número, posição, nacionalidade, jogos e gols na temporada (quando a fonte tem) e o código do Wikidata de quem está no acervo |
| `GET /v1/titulos` | Títulos por competição, com quantidade, anos e notas, e o `total`. Filtros: `escopo` (`mundial`, `continental`, `nacional`, `interestadual`, `estadual`, `internacional`, `amistoso`, `outros`) e `tipo` (`oficial`, o padrão; `outros`, para os torneios não oficiais; `todos`) |
| `GET /v1/titulos/{slug}` | Uma competição (ex.: `libertadores`, `paulista`, `brasileirao`), com cada edição e o link do artigo dela |

As listas de pessoas e partidas são paginadas com `pagina` e `limite` (padrão 50, máximo 200) e trazem `total`. Todo item traz a `fonte` (nome, link e licença), para dar o crédito exigido pela CC BY-SA.

Exemplos:

```bash
curl "localhost:8080/v1/partidas?adversario=palmeiras&limite=5"     # retrospecto no Derby
curl "localhost:8080/v1/feminino/partidas?temporada=2025&resultado=vitoria"
curl "localhost:8080/v1/feminino/artilharia?limite=10"
curl "localhost:8080/v1/pessoas?nome=socrates"
```

Os slugs de título são os mesmos nas duas categorias: `/v1/titulos/libertadores` e `/v1/feminino/titulos/libertadores`.

Toda resposta leva o header `X-Vai-Corinthians: sempre`. Erros sempre saem em JSON (`erro` e `dica`): rota inexistente dá 404 e método errado numa rota existente dá 405, com o header `Allow`. Os slugs não diferenciam maiúsculas de minúsculas.

## Documentação (OpenAPI)

O documento OpenAPI é montado pela própria API a partir do registro das rotas: cada rota é registrada junto com a sua descrição (`s.handle(mux, padrão, exemplo, doc{...}, handler)`), então não existe rota sem documentação. Os schemas das respostas ficam em [internal/server/openapi_schemas.go](internal/server/openapi_schemas.go).

O teste `TestRespostasBatemComOSchema` chama cada rota com o banco de teste e confere a resposta contra o schema documentado, recusando campos não documentados e status de erro que a rota não declara. Mudou o formato de uma resposta? O teste avisa até a documentação ser atualizada.

A página `/docs` usa o Swagger UI carregado do jsDelivr (`swagger-ui-dist@5.17.14`), então precisa de internet no navegador de quem a abre.

## Estrutura

```
cmd/api/main.go              ponto de entrada, config por variável de ambiente, shutdown gracioso
internal/data/               dados de seed em memória (masculino e feminino: clube, ídolos, história; e o estádio)
internal/banco/               esquema do timao.db, escrita (importador) e leitura (API)
internal/importar/           a importação completa (Executar) e os importadores de cada fonte: coleta (HTTP), nomes (tradução de competições e fases), wikidata, wikipedia, openfootball, rsssf e brasileirao
internal/atualizador/        atualização automática do banco com a API no ar
cmd/importar/main.go         gera o timao.db à mão
internal/brasilapi/          cliente da Brasil API com cache em memória (sucesso: 1 h; falha: 30 s) e requisições simultâneas compartilhadas
internal/jogos/              calendário de jogos lido de um arquivo JSON (com validação)
internal/server/             rotas, handlers, middlewares (log, recover, header) e testes
internal/acervo/             consultas ao timao.db (pessoas, partidas, gols, artilharia, títulos)
```

A `BrasilAPI` é uma interface em `internal/server`, então os handlers são testados com um fake, sem acesso à rede.

## Calendário de jogos

Os jogos vêm de um arquivo JSON, lido quando o servidor inicia. Sem a variável `JOGOS_FILE`, a API usa um calendário de exemplo embutido, com adversários fictícios, só para você testar. Para usar o seu, crie um arquivo assim e aponte `JOGOS_FILE` para ele:

```json
[
  {
    "data": "2026-10-25",
    "horario": "16:00",
    "adversario": "Nome do adversário",
    "competicao": "Campeonato Brasileiro",
    "mando": "casa",
    "local": "Neo Química Arena"
  }
]
```

- `data` (AAAA-MM-DD), `adversario`, `competicao` e `mando` (`casa` ou `fora`) são obrigatórios. `horario` (HH:MM, horário de Brasília) e `local` são opcionais.
- `categoria` é opcional: `masculino` (padrão, quando o campo falta) ou `feminino`. O mesmo arquivo serve às duas categorias: `/v1/jogos` mostra os masculinos e `/v1/feminino/jogos` os femininos.
- A ordem no arquivo não importa: a API ordena por data e horário. Se algum campo for inválido, o servidor não sobe e diz qual item tem problema.
- Cada jogo volta com `dia_semana`, `fim_de_semana` e `cai_em_feriado` (com o nome do `feriado`), cruzando a data com os feriados nacionais da Brasil API. Feriados municipais, como o aniversário de São Paulo, não entram.
- Se a Brasil API estiver fora do ar, os jogos continuam saindo, sem `cai_em_feriado`, e a resposta traz um campo `aviso`.
- `/jogos/proximos` mostra só jogos de hoje em diante (padrão 5, mude com `?limite=`) e `/jogos` lista todos. Os dois aceitam `?competicao=`, tanto em `/v1` quanto em `/v1/feminino`.

## Sobre os dados

Títulos, pessoas, partidas e gols vêm do `timao.db` (veja [Banco de dados](#banco-de-dados)). O que ainda é escrito à mão fica em `internal/data/data.go`: dados do clube, ídolos e ídolas, marcos da história e o estádio. Confira capacidade e CEP da arena em fontes oficiais antes de publicar.

A Libertadores Feminina de 2017 e a Copa do Brasil Feminina de 2016 foram conquistadas na parceria com o Audax (Audax/Corinthians). A Wikipédia conta os dois títulos e explica a parceria numa nota de rodapé, que a API repete no campo `notas`.

## Próximos passos

- **Jogos automáticos:** trocar o calendário manual por football-data.org ou API-Football. Basta implementar a interface `JogosProvider` (um método, `Jogos(ctx)`) e passá-la no `main.go`; os handlers não mudam. Vale checar se o plano grátis cobre Copa do Brasil e Libertadores.
- **Tabela de classificação:** rota nova com a posição do Corinthians, vinda da mesma API esportiva.
- **Mais Brasil API:** `GET /v1/clube/cnpj` com dados cadastrais via endpoint de CNPJ.
- **Mais fontes:** tabelas da Wikipédia em português (artilheiros, recordes, técnicos); partidas anteriores a 2003 e das Brabas antes de 2022, se aparecer uma fonte aberta.
- **Operação:** CORS, rate limit e deploy (o Dockerfile já existe).
