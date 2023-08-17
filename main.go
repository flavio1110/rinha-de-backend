package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

func main() {
	ctx := context.Background()

	// TODO: move to env var
	dbURL := "postgres://user:super-secret@localhost:5432/people?sslmode=disable"
	port := 9999

	dbPool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to create connection pool")
	}
	defer dbPool.Close()

	server := newServer(port, dbPool)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.start(); err != nil {
			log.Fatal().Err(err).Msg("Start http server")
		}
	}()

	log.Info().Msg("Server started - waiting for signal to stop")
	<-sig
	log.Info().Msg("Server shutting down")

	if err := server.Stop(ctx); err != nil {
		log.Fatal().Err(err).Msg("Stop server")
	}

	log.Info().Msg("Server stopped")
}
