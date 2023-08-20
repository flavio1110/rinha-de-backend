# rinha-de-backend
[![Lint Build Test](https://github.com/flavio1110/rinha-de-backend/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/flavio1110/rinha-de-backend/actions/workflows/ci.yml)

## My implementation of "Rinha de backend" challenge. 

- [Challenge (PT-BR)](https://github.com/zanfranceschi/rinha-de-backend-2023-q3)
- [Instructions (PT-BR)](https://github.com/zanfranceschi/rinha-de-backend-2023-q3/blob/main/INSTRUCOES.md)


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
