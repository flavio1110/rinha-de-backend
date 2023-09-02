package internal

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/patrickmn/go-cache"
	"github.com/redis/go-redis/v9"
)

type pessoaDBStore struct {
	dbPool           *pgxpool.Pool
	cacheApelido     *cache.Cache
	cacheByUID       *cache.Cache
	cacheSearch      *cache.Cache
	chSyncPessoaRead chan pessoa
	client           *redis.Client
}

func NewPessoaDBStore(dbPool *pgxpool.Pool, client *redis.Client) *pessoaDBStore {
	c1 := cache.New(5*time.Minute, 10*time.Minute)
	c2 := cache.New(5*time.Minute, 10*time.Minute)
	c3 := cache.New(30*time.Second, 10*time.Minute)
	chPessoa := make(chan pessoa, 1000)
	return &pessoaDBStore{
		dbPool:           dbPool,
		cacheApelido:     c1,
		cacheByUID:       c2,
		cacheSearch:      c3,
		chSyncPessoaRead: chPessoa,
		client:           client,
	}
}

func (p *pessoaDBStore) Add(ctx context.Context, pes pessoa) error {
	apelidoKey := fmt.Sprintf("apelido:%s", pes.Apelido)
	if _, found := p.cacheApelido.Get(apelidoKey); found {
		return errAddSkipped
	}

	terms := fmt.Sprintf("%s %s %s", strings.ToLower(pes.Apelido), strings.ToLower(pes.Nome), strings.ToLower(strings.Join(pes.Stack, " ")))
	insert := `INSERT INTO pessoas (Apelido, UID, Nome, Nascimento, Stack, search_terms) VALUES
    ($1, $2, $3, $4, $5, $6) ON CONFLICT DO NOTHING  returning uid;`

	res, err := p.dbPool.
		Exec(ctx, insert, pes.Apelido, pes.UID, pes.Nome, pes.Nascimento, pes.Stack, terms)

	if err != nil {
		return fmt.Errorf("inserting pessoa: %w", err)
	}

	// if no rows were affected, it means the pessoa already exists
	if res.RowsAffected() == 0 {
		// discarding error because we don't want to retry
		_ = p.cacheApelido.Add(apelidoKey, true, cache.DefaultExpiration)
		return errAddSkipped
	}

	// discarding error because we don't want to retry
	_ = p.cacheApelido.Add(apelidoKey, true, cache.DefaultExpiration)
	_ = p.cacheByUID.Add(pes.UID.String(), pes, cache.DefaultExpiration)

	return nil
}

func (p *pessoaDBStore) Get(ctx context.Context, id uuid.UUID) (pessoa, error) {
	if pes, found := p.cacheByUID.Get(id.String()); found {
		return pes.(pessoa), nil
	}

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
	// discarding error because we don't want to retry
	_ = p.cacheByUID.Add(pes.UID.String(), pes, cache.DefaultExpiration)
	return pes, nil
}

func (p *pessoaDBStore) Count(ctx context.Context) (int, error) {
	var count int
	err := p.dbPool.QueryRow(ctx, "select count(1) from pessoas;").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("querying count: %w", err)
	}
	return count, nil
}

func (p *pessoaDBStore) Search(ctx context.Context, term string) ([]pessoa, error) {
	var pessoas []pessoa
	query := `
	select Apelido, UID, Nome, to_char(Nascimento, 'YYYY-MM-DD'), Stack 
	   from pessoas
	     where search_terms ilike $1 limit 50;`

	rows, err := p.dbPool.Query(ctx, query, "%"+term+"%")
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
