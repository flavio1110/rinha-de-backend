package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Cache interface {
	Get(ctx context.Context, key string, dest any) (bool, error)
	Add(ctx context.Context, key string, value any, expiration int) error
}

func NewRedisCache(client *redis.Client) *redisCache {
	return &redisCache{
		client: client,
	}
}

type pessoaDBStore struct {
	dbPool           *pgxpool.Pool
	chSyncPessoaRead chan pessoa
	cache            Cache
}

func NewPessoaDBStore(dbPool *pgxpool.Pool, client *redis.Client) *pessoaDBStore {
	chPessoa := make(chan pessoa, 1000)
	return &pessoaDBStore{
		dbPool:           dbPool,
		chSyncPessoaRead: chPessoa,
		cache:            NewRedisCache(client),
	}
}

func (p *pessoaDBStore) Add(ctx context.Context, pes pessoa) error {
	apelidoKey := fmt.Sprintf("apelido:%s", pes.Apelido)

	// try to add the apelido to the cache
	// if it already exists, we skip the insert
	if err := p.cache.Add(ctx, apelidoKey, true, 0); err != nil {
		return errAddSkipped
	}

	if err := p.cache.Add(ctx, pes.UID.String(), pes, 0); err != nil {
		return errAddSkipped
	}

	terms := fmt.Sprintf("%s %s %s", strings.ToLower(pes.Apelido), strings.ToLower(pes.Nome), strings.ToLower(strings.Join(pes.Stack, " ")))
	insert := `INSERT INTO pessoas (Apelido, UID, Nome, Nascimento, Stack, search_terms) VALUES
    ($1, $2, $3, $4, $5, $6) ON CONFLICT DO NOTHING  returning uid;`

	_, err := p.dbPool.
		Exec(ctx, insert, pes.Apelido, pes.UID, pes.Nome, pes.Nascimento, pes.Stack, terms)

	if err != nil {
		return fmt.Errorf("inserting pessoa: %w", err)
	}

	return nil
}

func (p *pessoaDBStore) Get(ctx context.Context, id uuid.UUID) (pessoa, error) {
	var pes pessoa

	if found, _ := p.cache.Get(ctx, id.String(), &pes); found {
		return pes, nil
	}

	query := "select Apelido, UID, Nome, to_char(Nascimento, 'YYYY-MM-DD'), Stack from pessoas where UID = $1;"
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

type redisCache struct {
	client *redis.Client
}

func (r redisCache) Get(ctx context.Context, key string, dest any) (bool, error) {
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("getting key %s: %w", key, err)
	}
	if err := json.Unmarshal([]byte(val), dest); err != nil {
		return false, fmt.Errorf("unmarshaling value: %w", err)
	}

	return true, nil
}

func (r redisCache) Add(ctx context.Context, key string, value any, expiration int) error {
	val, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshaling value: %w", err)
	}

	added := r.client.SetNX(ctx, key, val, 0).Val()
	if !added {
		return errors.New("value not added")
	}

	return nil
}
