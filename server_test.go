package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func Test_Endpoints(t *testing.T) {
	ctx := context.Background()
	connString, terminate, err := startTestDB(ctx)
	require.NoError(t, err)
	defer terminate(t)

	dbPool, err := pgxpool.New(context.Background(), connString)
	require.NoError(t, err)
	defer dbPool.Close()
	migrateDB(ctx, dbPool)

	api := newServer(8888, dbPool)
	ts := httptest.NewServer(api.server.Handler)
	defer ts.Close()

	t.Run("status", func(t *testing.T) {
		resp, err := ts.Client().Get(ts.URL + "/status")
		assert.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
	})

	var locations []string

	t.Run("create person", func(t *testing.T) {
		tcs := map[string]struct {
			body               string
			expectedStatusCode int
		}{
			"success with stack": {
				body:               `{ "apelido" : "josé", "nome" : "José Roberto", "nascimento" : "2000-10-01", "stack" : ["C#", "Node", "Oracle"] }`,
				expectedStatusCode: 201,
			},
			"success without stack": {
				body:               `{ "apelido" : "ana", "nome" : "Ana Barbosa", "nascimento" : "1985-09-23", "stack" : null }`,
				expectedStatusCode: 201,
			},
			"failed duplicated apelido": {
				body:               `{ "apelido" : "josé", "nome" : "José Roberto", "nascimento" : "2000-10-01", "stack" : ["C#", "Node", "Oracle"] }`,
				expectedStatusCode: 400,
			},
			"failed null name": {
				body:               `{ "apelido" : "josé", "nome" : null, "nascimento" : "2000-10-01", "stack" : ["C#", "Node", "Oracle"] }`,
				expectedStatusCode: 400,
			},
			"failed null apelido": {
				body:               `{ "apelido" : null, "nome" : "José Roberto", "nascimento" : "2000-10-01", "stack" : ["C#", "Node", "Oracle"] }`,
				expectedStatusCode: 400,
			},
			"failed null nascimento": {
				body:               `{ "apelido" : "josé1", "nome" : "José Roberto", "nascimento" : null, "stack" : ["C#", "Node", "Oracle"] }`,
				expectedStatusCode: 400,
			},
			"failed nome as int": {
				body:               `{ "apelido" : "josé1", "nome" : 123, "nascimento" : "2000-10-01", "stack" : ["C#", "Node", "Oracle"] }`,
				expectedStatusCode: 400,
			},
			"failed stack item as int": {
				body:               `{ "apelido" : "josé1", "nome" : "José Roberto", "nascimento" : "2000-10-01", "stack" : ["C#", 123, "Oracle"] }`,
				expectedStatusCode: 400,
			},
		}
		for name, tc := range tcs {
			t.Run(name, func(t *testing.T) {
				resp, err := ts.Client().Post(ts.URL+"/pessoas", "application/json", strings.NewReader(tc.body))
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedStatusCode, resp.StatusCode)
				if resp.StatusCode == 201 {
					assert.Contains(t, resp.Header.Get("Location"), "/pessoas/")
					locations = append(locations, resp.Header.Get("Location"))
				}
			})
		}
	})

	for _, location := range locations {
		t.Run(fmt.Sprintf("get person - %s", location), func(t *testing.T) {
			resp, err := ts.Client().Get(ts.URL + location)
			assert.NoError(t, err)
			assert.Equal(t, 200, resp.StatusCode)

			var p pessoa
			err = json.NewDecoder(resp.Body).Decode(&p)
			assert.NoError(t, err)
			assert.Contains(t, location, p.UID.String())
			assert.NotEmpty(t, p.Apelido)
			assert.NotEmpty(t, p.Nome)
			assert.NotEmpty(t, p.Nascimento)
		})
	}

	t.Run("count pessoas", func(t *testing.T) {
		resp, err := ts.Client().Get(ts.URL + "/contagem-pessoas")
		assert.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("%d", len(locations)), string(body))
	})

	// TODO: make it case insensitive
	t.Run("search pessoas", func(t *testing.T) {
		tcs := map[string]struct {
			term          string
			expectedCount int
		}{
			"stack": {
				term:          "Node",
				expectedCount: 1,
			},
			"part of nome": {
				term:          "berto",
				expectedCount: 1,
			},
			"not matching": {
				term:          "Python",
				expectedCount: 0,
			},
		}
		for name, tc := range tcs {
			t.Run(name, func(t *testing.T) {
				resp, err := ts.Client().Get(ts.URL + "/pessoas?t=" + tc.term)
				assert.NoError(t, err)
				assert.Equal(t, 200, resp.StatusCode)
				var pessoas []pessoa
				err = json.NewDecoder(resp.Body).Decode(&pessoas)
				assert.NoError(t, err)
				assert.NotNil(t, pessoas)
				assert.Equal(t, tc.expectedCount, len(pessoas))
			})
		}
	})

}

func startTestDB(ctx context.Context) (string, func(t *testing.T), error) {
	var envVars = map[string]string{
		"POSTGRES_USER":     "user",
		"POSTGRES_PASSWORD": "super-secret",
		"POSTGRES_DB":       "people",
		"PORT":              "5432/tcp",
	}

	getConnString := func(host string, port nat.Port) string {
		return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
			envVars["POSTGRES_USER"],
			envVars["POSTGRES_PASSWORD"],
			host,
			port.Port(),
			envVars["POSTGRES_DB"])
	}

	req := testcontainers.ContainerRequest{
		Image:        "postgres:14",
		ExposedPorts: []string{envVars["PORT"]},
		Env:          envVars,
		WaitingFor:   wait.ForSQL(nat.Port(envVars["PORT"]), "pgx", getConnString).WithStartupTimeout(time.Second * 15),
	}
	pgC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return "", nil, fmt.Errorf("failed to start db container :%w", err)
	}
	port, err := pgC.MappedPort(ctx, "5432/tcp")
	if err != nil {
		return "", nil, fmt.Errorf("failed to get mapped port :%w", err)
	}
	host, err := pgC.Host(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get host :%w", err)
	}

	connString := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		envVars["POSTGRES_USER"],
		envVars["POSTGRES_PASSWORD"],
		host,
		port.Int(),
		envVars["POSTGRES_DB"])

	terminate := func(t *testing.T) {
		if err := pgC.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err.Error())
		}
	}
	return connString, terminate, nil
}

func migrateDB(ctx context.Context, dbPool *pgxpool.Pool) error {
	initContent, err := os.ReadFile("initdb.d/initdb.sql")
	if err != nil {
		log.Fatal("read init DB file: ", err)
	}

	_, err = dbPool.Exec(ctx, string(initContent))
	if err != nil {
		return fmt.Errorf("failed to migrate DB: %w", err)
	}
	return nil
}

func strPtr(v string) *string {
	return &v
}
