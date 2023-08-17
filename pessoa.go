package main

import (
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

type pessoaResource struct {
	store PessoasStore
}

type PessoasStore interface {
	Add(pessoa Pessoa) error
	Get(id string) (Pessoa, error)
	Count() (int, error)
	Search(term string) ([]Pessoa, error)
}

func (s *pessoaResource) postPessoa(w http.ResponseWriter, r *http.Request) {
	count, err := s.store.Count()
	if err != nil {
		log.Err(err).Msg("error counting pessoas")
		writeResponse(w, http.StatusInternalServerError, "")
		return
	}
	writeResponse(w, http.StatusOK, fmt.Sprintf("%d", count))
}

func (s *pessoaResource) getPessoa(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *pessoaResource) countPessoas(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *pessoaResource) searchPessoas(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

type Pessoa struct {
	Apelido    string   `json:"apelido"`
	Nome       string   `json:"nome"`
	Nascimento string   `json:"nascimento"`
	Stack      []string `json:"stack"`
}

type pessoaDBStore struct {
	dbPool *pgxpool.Pool
}

func (p pessoaDBStore) Add(pessoa Pessoa) error {
	//TODO implement me
	panic("implement me")
}

func (p pessoaDBStore) Get(id string) (Pessoa, error) {
	//TODO implement me
	panic("implement me")
}

func (p pessoaDBStore) Count() (int, error) {
	//TODO implement me
	panic("implement me")
}

func (p pessoaDBStore) Search(term string) ([]Pessoa, error) {
	//TODO implement me
	panic("implement me")
}
