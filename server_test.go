package main

import (
	"context"
	"fmt"
	"log"
	"net/http/httptest"
	"os"
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
