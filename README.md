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
You can see the evolution on the "results" folder.
e.g.

![2023-08-19 at 15.45.29 - increase worker connections on nginx.png](results%2F2023-08-19%20at%2015.45.29%20-%20increase%20worker%20connections%20on%20nginx.png)

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
