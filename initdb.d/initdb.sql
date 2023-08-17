create table pessoas (
    apelido varchar(32) primary key not null,
    uid uuid not null,
    nome  varchar(100) not null,
    nascimento date not null,
    stack varchar(32)[] null,
    search_content_pt  tsvector
        GENERATED ALWAYS AS (to_tsvector('portuguese', nome || ' ' || apelido || coalesce(array_to_string(stack, ' ', ''), '')))
        STORED
);

CREATE INDEX terms ON pessoas USING GIN (search_content_pt);
