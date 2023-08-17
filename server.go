package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

type apiServer struct {
	server http.Server
}

func newServer(port int, dbPool *pgxpool.Pool) *apiServer {
	r := mux.NewRouter()
	api := &apiServer{
		server: http.Server{
			Addr:    fmt.Sprintf("localhost:%d", port),
			Handler: r,
		},
	}

	resource := pessoaResource{
		store: pessoaDBStore{
			dbPool: dbPool,
		},
	}
	r.Use(setJSONContentType)
	r.HandleFunc("/pessoas", resource.postPessoa).Methods(http.MethodPost)
	r.HandleFunc("/pessoas/{id}", resource.getPessoa).Methods(http.MethodGet)
	r.HandleFunc("/contagem-pessoas", resource.countPessoas).Methods(http.MethodGet)
	r.HandleFunc("/pessoas", resource.searchPessoas).Methods(http.MethodGet)

	return api
}

func (s *apiServer) start() error {
	log.Info().Msgf("Listening HTTP on address %s", s.server.Addr)

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("listening HTTP: %w", err)
	}

	return nil
}

func (s *apiServer) Stop(ctx context.Context) error {
	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown server: %w", err)
	}
	return nil
}

func writeResponse(w http.ResponseWriter, status int, body string) {
	w.WriteHeader(status)
	_, err := w.Write([]byte(body))
	if err != nil {
		log.Err(err).Msg("error writing response")
	}
}

func writeJsonResponse(w http.ResponseWriter, status int, body any) {
	bodyJson, err := json.Marshal(body)
	if err != nil {
		log.Err(err).Msg("error marshalling response")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, err = w.Write(bodyJson)
	if err != nil {
		log.Err(err).Msg("error writing response")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if status != http.StatusOK {
		w.WriteHeader(status)
	}
}

func setJSONContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		next.ServeHTTP(w, req)
	})
}
