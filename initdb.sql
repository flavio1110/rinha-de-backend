CREATE EXTENSION pg_trgm;

create table pessoas (
    apelido varchar(32) primary key not null,
    uid uuid not null,
    nome  varchar(100) not null,
    nascimento date not null,
    stack text null,
    search_terms text
);

CREATE INDEX ix_search_terms ON pessoas USING gist (search_terms gist_trgm_ops(siglen=32));

