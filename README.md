# rinha-de-backend

[![Lint Build Test](https://github.com/flavio1P110/rinha-de-backend/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/flavio1110/rinha-de-backend/actions/workflows/ci.yml)

## My implementation of "Rinha de backend" challenge.

[Check out my blog post about my solution and official results](https://fsilva.me/rinha-de-backend.html).

<img src="https://fsilva.me/images/rooster_fight.png" />

[Check out my blog post about my solution and official results](https://fsilva.me/rinha-de-backend.html).

- [Challenge (PT-BR)](https://github.com/zanfranceschi/rinha-de-backend-2023-q3)
- [Instructions (PT-BR)](https://github.com/zanfranceschi/rinha-de-backend-2023-q3/blob/main/INSTRUCOES.md)

Some decisions were made to simplify the challenge, such as:

- Only using in-memory cache. It's fine for the tested lod, but if it increases and more instances are needed, a distributed cache would be better.
  Pre-allocating all DB connections available for the app. It speeds up the warm-up, but it's not ideal.
- Missing graceful shutdown for syncing the read table. If the app is killed, the read table will be out of sync.
- Duplicating the same table with the sole purpose of reading. There are better ways to do it, the idea here is to have a write table with just a single PK index, to speed up the writes.
- No batching of writes of the replica table. Despite adding them one by one, we could batch the items and save many DB roundtrips.
- Not enforcing uniqueness of UUID on the write table. The likelihood of collision is very low, but it's not zero.
- etc.. :)

Feel free to explore, fork and send PRs. :)

## Stack

- [Go](https://golang.org/)
- [PostgreSQL](https://www.postgresql.org/)
- [Nginx](https://www.nginx.com/)
- [Docker](https://www.docker.com/)
- [Gatling](https://gatling.io/)

## Results

The load tests are executed on [CI](https://github.com/flavio1110/rinha-de-backend/actions/workflows/ci.yml) and results are published in Github Pages and can be seen [here as an example](https://fsilva.me/rinha-de-backend/rinhabackendsimulation-20230820170604710).

## How to run

```
make compose-up
```

### Sample request

```
curl -v "http://localhost:9999/contagem-pessoas"
```

## Acceptance tests

These tests are based on the challenge instructions and ensure the application works.
They run against a test server without Nginx, but using a real database that are automatically started using [testconainers](https://www.testcontainers.org/).

```
make tests
```
