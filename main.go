package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	_ "net/http/pprof"

	"github.com/flavio1110/rinha-de-backend/internal/pessoas"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	ctx, cancel := context.WithCancel(context.Background())

	redisClient, err := pessoas.NewRedisCache(ctx, os.Getenv("REDIS_ADDR"))
	if err != nil {
		log.Fatal().Err(err).Msg("configure redis")
	}

	dbConfig, err := getDBConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("configure db")
	}

	store, terminateDBPool, err := pessoas.NewPessoaDBStore(dbConfig, redisClient, 10*time.Second)
	if err != nil {
		log.Fatal().Err(err).Msg("configure db store")
	}
	defer terminateDBPool()

	isLocal := os.Getenv("LOCAL_ENV") == "true"
	port, err := strconv.Atoi(os.Getenv("HTTP_PORT"))
	if err != nil {
		log.Fatal().Err(err).Msgf("Unable to parse HTTP_PORT %q", os.Getenv("HTTP_PORT"))
	}

	server := pessoas.NewServer(port, store, isLocal)

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

	store.StopSync()
	log.Info().Msg("Server stopped")
}

func getDBConfig() (pessoas.DBConfig, error) {
	dbURL := os.Getenv("DB_URL")
	maxConnections, err := strconv.Atoi(os.Getenv("DB_MAX_CONNECTIONS"))
	if err != nil {
		return pessoas.DBConfig{}, fmt.Errorf("unable to parse DB_MAX_CONNECTIONS %q", os.Getenv("DB_MAX_CONNECTIONS"))
	}

	minConnections, err := strconv.Atoi(os.Getenv("DB_MIN_CONNECTIONS"))
	if err != nil {
		return pessoas.DBConfig{}, fmt.Errorf("unable to parse DB_MIN_CONNECTIONS %q", os.Getenv("DB_MIN_CONNECTIONS"))
	}

	return pessoas.DBConfig{
		DbURL:   dbURL,
		MaxConn: int32(maxConnections),
		MinConn: int32(minConnections),
	}, nil
}
