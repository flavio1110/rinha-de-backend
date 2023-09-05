package internal

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
	"github.com/redis/go-redis/v9"
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

	redisAddr, terminate, err := startTestRedis(ctx)
	require.NoError(t, err)
	defer terminate(t)

	dbPool, err := pgxpool.New(context.Background(), connString)
	require.NoError(t, err)
	defer dbPool.Close()
	err = migrateDB(ctx, dbPool)
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	store := NewPessoaDBStore(dbPool, client, 1*time.Second)

	err = store.StartSync(ctx)
	assert.NoError(t, err)

	api := NewServer(8888, store, true)
	ts := httptest.NewServer(api.server.Handler)
	defer ts.Close()

	t.Run("status", func(t *testing.T) {
		resp, err := ts.Client().Get(ts.URL + "/status")
		assert.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
	})

	var locations []string

	t.Run("create person", func(t *testing.T) {
		tcs := []tcAdd{
			{
				name:               "success with stack",
				body:               `{ "apelido" : "josé", "nome" : "José Roberto", "nascimento" : "2000-10-01", "stack" : ["C#", "Node", "Oracle"] }`,
				expectedStatusCode: 201,
			},
			{
				name:               "success without stack",
				body:               `{ "apelido" : "ana", "nome" : "Ana Barbosa", "nascimento" : "1985-09-23", "stack" : null }`,
				expectedStatusCode: 201,
			},
			{
				name:               "failed duplicated apelido",
				body:               `{ "apelido" : "josé", "nome" : "José Roberto", "nascimento" : "2000-10-01", "stack" : ["C#", "Node", "Oracle"] }`,
				expectedStatusCode: 422,
			},
			{
				name:               "failed null name",
				body:               `{ "apelido" : "josé", "nome" : null, "nascimento" : "2000-10-01", "stack" : ["C#", "Node", "Oracle"] }`,
				expectedStatusCode: 422,
			},
			{
				name:               "failed null apelido",
				body:               `{ "apelido" : null, "nome" : "José Roberto", "nascimento" : "2000-10-01", "stack" : ["C#", "Node", "Oracle"] }`,
				expectedStatusCode: 422,
			},
			{
				name:               "failed null nascimento",
				body:               `{ "apelido" : "josé1", "nome" : "José Roberto", "nascimento" : null, "stack" : ["C#", "Node", "Oracle"] }`,
				expectedStatusCode: 422,
			},
			{
				name:               "failed nome as int",
				body:               `{ "apelido" : "josé1", "nome" : 123, "nascimento" : "2000-10-01", "stack" : ["C#", "Node", "Oracle"] }`,
				expectedStatusCode: 400,
			},
			{
				name:               "failed stack item as int",
				body:               `{ "apelido" : "josé1", "nome" : "José Roberto", "nascimento" : "2000-10-01", "stack" : ["C#", 123, "Oracle"] }`,
				expectedStatusCode: 400,
			},
		}
		for _, tc := range tcs {
			t.Run(tc.name, func(t *testing.T) {
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
		assert.Eventuallyf(t, func() bool {
			resp, err := ts.Client().Get(ts.URL + "/contagem-pessoas")
			assert.NoError(t, err)
			assert.Equal(t, 200, resp.StatusCode)
			body, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)
			return assert.Equal(t, fmt.Sprintf("%d", len(locations)), string(body))
		}, 30*time.Second, 5*time.Second, "timeout waiting for count")

	})

	t.Run("search pessoas", func(t *testing.T) {
		tcs := map[string]struct {
			term          string
			expectedCount int
		}{
			"stack": {
				term:          "node",
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
				assert.Eventuallyf(t, func() bool {
					resp, err := ts.Client().Get(ts.URL + "/pessoas?t=" + tc.term)
					assert.NoError(t, err)
					assert.Equal(t, 200, resp.StatusCode)
					var pessoas []pessoa
					err = json.NewDecoder(resp.Body).Decode(&pessoas)
					assert.NoError(t, err)
					assert.NotNil(t, pessoas)
					assert.Equal(t, tc.expectedCount, len(pessoas))
					return tc.expectedCount == len(pessoas)
				}, 5*time.Second, 1*time.Second, "timeout waiting for search")
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
		Image:        "postgres:15",
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

	connString := getConnString(host, port)

	terminate := func(t *testing.T) {
		if err := pgC.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err.Error())
		}
	}
	return connString, terminate, nil
}

func startTestRedis(ctx context.Context) (string, func(t *testing.T), error) {
	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections tcp"),
	}
	redis, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return "", nil, fmt.Errorf("failed to start db container :%w", err)
	}
	port, err := redis.MappedPort(ctx, "6379/tcp")
	if err != nil {
		return "", nil, fmt.Errorf("failed to get mapped port :%w", err)
	}
	host, err := redis.Host(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get host :%w", err)
	}

	terminate := func(t *testing.T) {
		if err := redis.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err.Error())
		}
	}
	return fmt.Sprintf("%s:%d", host, port.Int()), terminate, nil
}

func migrateDB(ctx context.Context, dbPool *pgxpool.Pool) error {
	initContent, err := os.ReadFile("../initdb.sql")
	if err != nil {
		log.Fatal("read init DB file: ", err)
	}

	_, err = dbPool.Exec(ctx, string(initContent))
	if err != nil {
		return fmt.Errorf("failed to migrate DB: %w", err)
	}
	return nil
}

type tcAdd struct {
	name               string
	body               string
	expectedStatusCode int
}
