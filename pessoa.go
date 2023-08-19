package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

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
	Add(ctx context.Context, pessoa pessoa) error
	Get(ctx context.Context, id uuid.UUID) (pessoa, error)
	Count(ctx context.Context) (int, error)
	Search(ctx context.Context, term string) ([]pessoa, error)
}

func (s *pessoaResource) postPessoa(w http.ResponseWriter, r *http.Request) {
	var pessoa pessoa
	if err := json.NewDecoder(r.Body).Decode(&pessoa); err != nil {
		writeResponse(w, http.StatusBadRequest, "")
		return
	}
	defer r.Body.Close()

	if !isNewPessoaValid(pessoa) {
		writeResponse(w, http.StatusUnprocessableEntity, "")
		return
	}

	pessoa.UID = uuid.New()

	if err := s.store.Add(r.Context(), pessoa); err != nil {
		if errors.Is(err, errAddSkipped) {
			writeResponse(w, http.StatusUnprocessableEntity, "")
			return
		}
		log.Err(err).Msg("error adding pessoa")
		writeResponse(w, http.StatusInternalServerError, "")
		return
	}
	w.Header().Add("Location", fmt.Sprintf("/pessoas/%s", pessoa.UID))
	writeJsonResponse(w, http.StatusCreated, pessoa)
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
		pessoas = []pessoa{}
	}
	writeJsonResponse(w, http.StatusOK, pessoas)
}

type pessoa struct {
	UID        uuid.UUID `json:"id"`
	Apelido    string    `json:"Apelido"`
	Nome       string    `json:"Nome"`
	Nascimento string    `json:"Nascimento"`
	Stack      []string  `json:"Stack"`
}

func isNewPessoaValid(p pessoa) bool {
	if p.UID != uuid.Nil {
		return false
	}
	if p.Apelido == "" || len(p.Apelido) > 32 {
		return false
	}
	if p.Nome == "" || len(p.Nome) > 100 {
		return false
	}

	if p.Stack != nil {
		for i := range p.Stack {
			if len(p.Stack[i]) > 32 {
				return false
			}
		}
	}

	if _, err := time.Parse("2006-01-02", p.Nascimento); err != nil {
		return false
	}

	return true
}

type pessoaDBStore struct {
	dbPool *pgxpool.Pool
}

func (p pessoaDBStore) Add(ctx context.Context, pes pessoa) error {
	insert := `INSERT INTO pessoas (Apelido, UID, Nome, Nascimento, Stack) VALUES
    ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING  returning uid;`

	err := p.dbPool.
		QueryRow(ctx, insert, pes.Apelido, pes.UID, pes.Nome, pes.Nascimento, pes.Stack).
		Scan(&pes.UID)

	if err != nil {
		if err == pgx.ErrNoRows {
			return errAddSkipped
		}
		return fmt.Errorf("inserting pessoa: %w", err)
	}
	return nil
}

func (p pessoaDBStore) Get(ctx context.Context, id uuid.UUID) (pessoa, error) {
	query := "select Apelido, UID, Nome, to_char(Nascimento, 'YYYY-MM-DD'), Stack from pessoas where UID = $1;"
	var pes pessoa
	err := p.dbPool.QueryRow(ctx, query, id).
		Scan(&pes.Apelido, &pes.UID, &pes.Nome, &pes.Nascimento, &pes.Stack)

	if err != nil {
		if err == pgx.ErrNoRows {
			return pessoa{}, errNotFound
		}

		return pessoa{}, fmt.Errorf("querying pessoa: %w", err)
	}
	return pes, nil
}

func (p pessoaDBStore) Count(ctx context.Context) (int, error) {
	var count int
	err := p.dbPool.QueryRow(ctx, "select count(1) from pessoas;").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("querying count: %w", err)
	}
	return count, nil
}

func (p pessoaDBStore) Search(ctx context.Context, term string) ([]pessoa, error) {
	var pessoas []pessoa
	query := `
	select Apelido, UID, Nome, to_char(Nascimento, 'YYYY-MM-DD'), Stack 
	   from pessoas 
	    where Nome ilike $1
	       or Apelido ilike $1
	       or $2=ANY(Stack)
	LIMIT 50;`

	rows, err := p.dbPool.Query(ctx, query, "%"+term+"%", term)
	if err != nil {
		return nil, fmt.Errorf("querying pessoas: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var pes pessoa
		err := rows.Scan(&pes.Apelido, &pes.UID, &pes.Nome, &pes.Nascimento, &pes.Stack)
		if err != nil {
			return nil, fmt.Errorf("scanning pessoa: %w", err)
		}
		pessoas = append(pessoas, pes)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating over pessoas: %w", err)
	}
	return pessoas, nil
}

var errNotFound = errors.New("not found")
var errAddSkipped = errors.New("skipped due to conflict")
