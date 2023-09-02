CREATE EXTENSION pg_trgm;

create table pessoas (
    apelido varchar(32) primary key not null,
    uid uuid not null,
    nome  varchar(100) not null,
    nascimento date not null,
    stack varchar(32)[] null,
    search_terms text
);

CREATE index ix_uid on pessoas (uid);
CREATE INDEX ix_search_terms ON pessoas USING gist (search_terms gist_trgm_ops);

