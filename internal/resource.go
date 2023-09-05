package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
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
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if !isNewPessoaValid(pessoa) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	pessoa.UID = uuid.New()

	if err := s.store.Add(r.Context(), pessoa); err != nil {
		if errors.Is(err, errAddSkipped) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		log.Err(err).Msg("error adding pessoa")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Add("Location", fmt.Sprintf("/pessoas/%s", pessoa.UID))
	w.WriteHeader(http.StatusCreated)
}

func (s *pessoaResource) getPessoa(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, ok := vars["id"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	uid, err := uuid.Parse(id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	pessoa, err := s.store.Get(r.Context(), uid)
	if err != nil {
		if errors.Is(err, errNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		log.Err(err).Msg("error getting pessoa")
		w.WriteHeader(http.StatusInternalServerError)
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
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	pessoas, err := s.store.Search(r.Context(), term)
	if err != nil {
		log.Err(err).Msg("error searching pessoas")
		w.WriteHeader(http.StatusInternalServerError)
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

var errNotFound = errors.New("not found")
var errAddSkipped = errors.New("skipped due to conflict")
