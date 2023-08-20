package main

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	_ "net/http/pprof"

	"github.com/flavio1110/rinha-de-backend/internal"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	dbURL := os.Getenv("DB_URL")
	port, err := strconv.Atoi(os.Getenv("HTTP_PORT"))
	if err != nil {
		log.Fatal().Err(err).Msgf("Unable to parse HTTP_PORT %q", os.Getenv("HTTP_PORT"))
	}

	maxConnections, err := strconv.Atoi(os.Getenv("DB_MAX_CONNECTIONS"))
	if err != nil {
		log.Fatal().Err(err).Msgf("Unable to parse DB_MAX_CONNECTIONS %q", os.Getenv("DB_MAX_CONNECTIONS"))
	}

	isLocal := os.Getenv("LOCAL_ENV") == "true"

	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to parse config")
	}
	config.MaxConns = int32(maxConnections)
	// Gimme all connections. I'm a greedy bastard.
	config.MinConns = int32(maxConnections)
	config.MaxConnIdleTime = time.Minute * 3

	dbPool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to create connection pool")
	}
	defer dbPool.Close()

	server := internal.NewServer(port, dbPool, isLocal)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.Start(ctx); err != nil {
			log.Fatal().Err(err).Msg("Start http server")
		}
	}()

	log.Info().Msg("Server started - waiting for signal to stop")
	<-sig
	log.Info().Msg("Server shutting down")
	cancel()

	if err := server.Stop(ctx); err != nil {
		log.Fatal().Err(err).Msg("Stop server")
	}

	log.Info().Msg("Server stopped")
}
