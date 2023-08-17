package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

type pessoaResource struct {
	store PessoasStore
}

type PessoasStore interface {
	Add(ctx context.Context, pessoa Pessoa) error
	Get(ctx context.Context, id uuid.UUID) (Pessoa, error)
	Count(ctx context.Context) (int, error)
	Search(ctx context.Context, term string) ([]Pessoa, error)
}

func (s *pessoaResource) postPessoa(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *pessoaResource) getPessoa(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, ok := vars["id"]
	if !ok {
		writeResponse(w, http.StatusBadRequest, "")
		return
	}
	uid, err := uuid.Parse(id)
	if err != nil {
		writeResponse(w, http.StatusNotFound, "")
		return
	}

	pessoa, err := s.store.Get(r.Context(), uid)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeResponse(w, http.StatusNotFound, "")
			return
		}
		log.Err(err).Msg("error getting pessoa")
		writeResponse(w, http.StatusInternalServerError, "")
		return
	}

	writeJsonResponse(w, http.StatusOK, pessoa)
}

func (s *pessoaResource) countPessoas(w http.ResponseWriter, r *http.Request) {
	count, err := s.store.Count(r.Context())
	if err != nil {
		log.Err(err).Msg("error counting pessoas")
		writeResponse(w, http.StatusInternalServerError, "")
		return
	}
	writeResponse(w, http.StatusOK, fmt.Sprintf("%d", count))

}

func (s *pessoaResource) searchPessoas(w http.ResponseWriter, r *http.Request) {
	term := r.URL.Query().Get("t")
	if term == "" {
		writeResponse(w, http.StatusBadRequest, "")
		return
	}
	pessoas, err := s.store.Search(r.Context(), term)
	if err != nil {
		log.Err(err).Msg("error searching pessoas")
		writeResponse(w, http.StatusInternalServerError, "")
		return
	}
	if pessoas == nil {
		pessoas = []Pessoa{}
	}
	writeJsonResponse(w, http.StatusOK, pessoas)
}

type Pessoa struct {
	UID        uuid.UUID `json:"id"`
	Apelido    string    `json:"apelido"`
	Nome       string    `json:"nome"`
	Nascimento string    `json:"nascimento"`
	Stack      []string  `json:"stack"`
}

type pessoaDBStore struct {
	dbPool *pgxpool.Pool
}

func (p pessoaDBStore) Add(ctx context.Context, pessoa Pessoa) error {
	insert := `INSERT INTO pessoas (apelido, uid, nome, nascimento, stack) VALUES
    ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING;`

	res, err := p.dbPool.Exec(ctx, insert, pessoa.Apelido, pessoa.UID, pessoa.Nome, pessoa.Nascimento, pessoa.Stack)
	if err != nil {
		return fmt.Errorf("inserting pessoa: %w", err)
	}

	if res.RowsAffected() == 0 {
		return fmt.Errorf("pessoa already exists")
	}

	return nil
}

func (p pessoaDBStore) Get(ctx context.Context, id uuid.UUID) (Pessoa, error) {
	query := "select apelido, uid, nome, to_char(nascimento, 'YYYY-MM-DD'), stack from pessoas where uid = $1;"
	var pessoa Pessoa
	err := p.dbPool.QueryRow(ctx, query, id).
		Scan(&pessoa.Apelido, &pessoa.UID, &pessoa.Nome, &pessoa.Nascimento, &pessoa.Stack)

	if err != nil {
		if err == pgx.ErrNoRows {
			return Pessoa{}, errNotFound
		}

		return Pessoa{}, fmt.Errorf("querying pessoa: %w", err)
	}
	return pessoa, nil
}

func (p pessoaDBStore) Count(ctx context.Context) (int, error) {
	var count int
	err := p.dbPool.QueryRow(context.Background(), "select count(1) from pessoas;").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("querying count: %w", err)
	}
	return count, nil
}

func (p pessoaDBStore) Search(ctx context.Context, term string) ([]Pessoa, error) {
	var pessoas []Pessoa
	query := "select apelido, uid, nome, to_char(nascimento, 'YYYY-MM-DD'), stack from pessoas where nome ilike $1;"
	rows, err := p.dbPool.Query(ctx, query, "%"+term+"%")
	if err != nil {
		return nil, fmt.Errorf("querying pessoas: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var pessoa Pessoa
		err := rows.Scan(&pessoa.Apelido, &pessoa.UID, &pessoa.Nome, &pessoa.Nascimento, &pessoa.Stack)
		if err != nil {
			return nil, fmt.Errorf("scanning pessoa: %w", err)
		}
		pessoas = append(pessoas, pessoa)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating over pessoas: %w", err)
	}
	return pessoas, nil
}

var errNotFound = errors.New("not found")
