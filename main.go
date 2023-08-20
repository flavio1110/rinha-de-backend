package main

import (
	"context"
	"os"
	"os/signal"
	"runtime/trace"
	"strconv"
	"syscall"
	"time"

	_ "net/http/pprof"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

func main() {
	ctx := context.Background()
	traceEnabled := os.Getenv("TRACE_ENABLED") == "true"
	if traceEnabled {

		f, err := os.Create("trace.out")
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create trace output file")
		}
		defer func() {
			if err := f.Close(); err != nil {
				log.Fatal().Err(err).Msg("failed to close trace output file")
			}
		}()

		if err := trace.Start(f); err != nil {
			log.Fatal().Err(err).Msg("failed to start trace")
		}
		defer trace.Stop()
	}
	start(ctx)
}

func start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
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
	config.MinConns = int32(maxConnections)
	config.MaxConnIdleTime = time.Minute * 3

	dbPool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to create connection pool")
	}
	defer dbPool.Close()

	server := newServer(port, dbPool, isLocal)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.start(ctx); err != nil {
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
