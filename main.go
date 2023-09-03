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
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	dbPool := setupDB()
	defer dbPool.Close()
	if err := dbPool.Ping(ctx); err != nil {
		log.Fatal().Err(err).Msg("Unable to ping database")
	}

	client := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"),
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	if status := client.Ping(ctx); status.Err() != nil {
		log.Fatal().Err(status.Err()).Msg("Unable to ping redis")
	}

	isLocal := os.Getenv("LOCAL_ENV") == "true"
	port, err := strconv.Atoi(os.Getenv("HTTP_PORT"))
	if err != nil {
		log.Fatal().Err(err).Msgf("Unable to parse HTTP_PORT %q", os.Getenv("HTTP_PORT"))
	}

	store := internal.NewPessoaDBStore(dbPool, client)
	server := internal.NewServer(port, store, isLocal)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := store.StartSync(ctx); err != nil {
			log.Fatal().Err(err).Msg("Start sync")
		}
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

	store.StopSync(ctx)
	log.Info().Msg("Server stopped")
}

func setupDB() *pgxpool.Pool {
	dbURL := os.Getenv("DB_URL")
	maxConnections, err := strconv.Atoi(os.Getenv("DB_MAX_CONNECTIONS"))
	if err != nil {
		log.Fatal().Err(err).Msgf("Unable to parse DB_MAX_CONNECTIONS %q", os.Getenv("DB_MAX_CONNECTIONS"))
	}

	minConnections, err := strconv.Atoi(os.Getenv("DB_MIN_CONNECTIONS"))
	if err != nil {
		log.Fatal().Err(err).Msgf("Unable to parse DB_MIN_CONNECTIONS %q", os.Getenv("DB_MAX_CONNECTIONS"))
	}

	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to parse config")
	}
	config.MinConns = int32(minConnections)
	config.MaxConns = int32(maxConnections)
	config.MaxConnIdleTime = time.Minute * 3

	dbPool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to create connection pool")
	}
	return dbPool
}
