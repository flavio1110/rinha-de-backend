create table pessoas (
    apelido varchar(32) primary key not null,
    uid uuid not null,
    nome  varchar(100) not null,
    nascimento date not null,
    stack varchar(32)[] null
);

CREATE index ix_uid on pessoas (uid);
--CREATE INDEX terms ON pessoas USING GIN (stack);
