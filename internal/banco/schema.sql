-- Esquema do timao.db. Gerado do zero a cada importação (cmd/importar).
-- Toda linha aponta para a fonte de onde veio, para dar crédito e permitir conferência.

CREATE TABLE meta (
    chave TEXT PRIMARY KEY,
    valor TEXT NOT NULL
);

CREATE TABLE fonte (
    id          INTEGER PRIMARY KEY,
    nome        TEXT NOT NULL,
    url         TEXT NOT NULL UNIQUE,
    licenca     TEXT NOT NULL,
    coletado_em TEXT NOT NULL -- RFC 3339
);

-- Jogadores, jogadoras e técnicos.
CREATE TABLE pessoa (
    id            INTEGER PRIMARY KEY,
    wikidata      TEXT UNIQUE,      -- ex.: Q312937
    nome          TEXT NOT NULL,
    nome_busca    TEXT NOT NULL,    -- nome sem acentos e em minúsculas, para busca
    genero        TEXT CHECK (genero IN ('masculino', 'feminino')),
    nascimento    TEXT,             -- AAAA, AAAA-MM ou AAAA-MM-DD, conforme a precisão da fonte
    falecimento   TEXT,
    nacionalidade TEXT,             -- separadas por vírgula quando há mais de uma
    posicao       TEXT,             -- idem
    foto_url      TEXT,
    wiki_pt       TEXT,             -- título do artigo na Wikipédia em português
    wiki_en       TEXT,             -- idem, em inglês (usado para ligar autores de gols)
    fonte_id      INTEGER NOT NULL REFERENCES fonte(id)
);

-- Passagem de uma pessoa pelo clube, como jogador(a) ou técnico(a).
CREATE TABLE passagem (
    id        INTEGER PRIMARY KEY,
    pessoa_id INTEGER NOT NULL REFERENCES pessoa(id),
    categoria TEXT NOT NULL CHECK (categoria IN ('masculino', 'feminino')),
    funcao    TEXT NOT NULL CHECK (funcao IN ('jogador', 'tecnico')),
    inicio    TEXT,
    fim       TEXT,
    jogos     INTEGER,
    gols      INTEGER,
    fonte_id  INTEGER NOT NULL REFERENCES fonte(id)
);
CREATE INDEX passagem_pessoa ON passagem(pessoa_id);
CREATE INDEX passagem_categoria ON passagem(categoria, funcao);

-- Edições de competições vencidas pelo clube, uma linha por conquista.
CREATE TABLE titulo (
    id             INTEGER PRIMARY KEY,
    categoria      TEXT NOT NULL CHECK (categoria IN ('masculino', 'feminino')),
    competicao     TEXT NOT NULL,
    slug           TEXT NOT NULL,      -- identificador estável da competição, ex.: libertadores
    escopo         TEXT NOT NULL,      -- mundial, continental, nacional, interestadual, estadual, internacional, amistoso ou outros
    oficial        INTEGER NOT NULL CHECK (oficial IN (0, 1)),
    ano            INTEGER NOT NULL,
    edicao_wiki_en TEXT,               -- artigo da edição na Wikipédia em inglês, quando há link
    nota           TEXT,               -- observação da fonte sobre a conquista
    fonte_id       INTEGER NOT NULL REFERENCES fonte(id),
    UNIQUE (categoria, slug, ano)
);
CREATE INDEX titulo_categoria ON titulo(categoria, slug);

-- Elenco de cada temporada, das páginas de temporada da Wikipédia.
CREATE TABLE elenco (
    id            INTEGER PRIMARY KEY,
    categoria     TEXT NOT NULL CHECK (categoria IN ('masculino', 'feminino')),
    temporada     INTEGER NOT NULL,
    pessoa_id     INTEGER REFERENCES pessoa(id), -- nulo quando a pessoa não tem item no Wikidata
    nome          TEXT NOT NULL,
    numero        INTEGER,
    posicao       TEXT,
    nacionalidade TEXT,                          -- código FIFA, ex.: BRA
    jogos         INTEGER,                       -- na temporada, quando a página tem estatísticas
    gols          INTEGER,
    fonte_id      INTEGER NOT NULL REFERENCES fonte(id),
    UNIQUE (categoria, temporada, nome)
);
CREATE INDEX elenco_pessoa ON elenco(pessoa_id);

CREATE TABLE partida (
    id              INTEGER PRIMARY KEY,
    categoria       TEXT NOT NULL CHECK (categoria IN ('masculino', 'feminino')),
    temporada       INTEGER NOT NULL,
    data            TEXT NOT NULL,  -- AAAA-MM-DD
    horario         TEXT,           -- HH:MM, como aparece na fonte
    competicao      TEXT,
    fase            TEXT,
    adversario      TEXT NOT NULL,
    adversario_busca TEXT NOT NULL, -- idem, para busca
    mando           TEXT NOT NULL CHECK (mando IN ('casa', 'fora')), -- casa = Corinthians listado como mandante
    gols_pro        INTEGER,        -- nulo quando o jogo ainda não aconteceu
    gols_contra     INTEGER,
    penaltis_pro    INTEGER,
    penaltis_contra INTEGER,
    estadio         TEXT,
    cidade          TEXT,
    publico         INTEGER,
    arbitro         TEXT,
    fonte_id        INTEGER NOT NULL REFERENCES fonte(id),
    UNIQUE (categoria, data, adversario)
);
CREATE INDEX partida_data ON partida(categoria, data);

CREATE TABLE gol (
    id            INTEGER PRIMARY KEY,
    partida_id    INTEGER NOT NULL REFERENCES partida(id),
    lado          TEXT NOT NULL CHECK (lado IN ('pro', 'contra')), -- pro = gol do Corinthians
    autor         TEXT,
    autor_wiki_en TEXT,           -- título do artigo do autor na Wikipédia em inglês, quando há link
    minuto        TEXT,           -- ex.: 45+2
    tipo          TEXT NOT NULL CHECK (tipo IN ('normal', 'penalti', 'contra'))
);
CREATE INDEX gol_partida ON gol(partida_id);
CREATE INDEX gol_autor ON gol(autor_wiki_en);
