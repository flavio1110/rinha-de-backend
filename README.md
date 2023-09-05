#### NB!: This is not the version I submitted to the challenge. You can find that version on the [branch v1](https://github.com/flavio1110/rinha-de-backend/tree/v1).

# rinha-de-backend

[![Lint Build Test](https://github.com/flavio1110/rinha-de-backend/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/flavio1110/rinha-de-backend/actions/workflows/ci.yml)

## My implementation of "Rinha de backend" challenge.
 

[Check out my blog post about my solution and official results](https://fsilva.me/rinha-de-backend.html).

<img src="https://fsilva.me/images/rooster_fight.png" />

[Check out my blog post about my solution and official results](https://fsilva.me/rinha-de-backend.html).

- [Challenge (PT-BR)](https://github.com/zanfranceschi/rinha-de-backend-2023-q3)
- [Instructions (PT-BR)](https://github.com/zanfranceschi/rinha-de-backend-2023-q3/blob/main/INSTRUCOES.md)

## Commands TL;DR
````
make run # Start only the APP
make up-deps # Start the app and dependencies (DB and Redis)
make tests # Run acceptance tests
make lint # Run linter

make compose-up # Start the app with docker-compose
````

## NB! This code should not be used as a reference for production software!
Some decisions here are intended just to make the code simpler and faster to write, but they are not ideal for production software. For example:

- Pre-allocating all DB connections available for the app. It speeds up the warm-up, but it's not ideal.
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
