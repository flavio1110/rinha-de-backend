package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

type Cache interface {
	Get(ctx context.Context, key string, dest any) (bool, error)
	Add(ctx context.Context, key string, value any, expiration int) error
}

type PessoaDBStore struct {
	dbPool       *pgxpool.Pool
	chSyncPessoa chan pessoa
	chSignStop   chan struct{}
	cache        Cache
}

func NewPessoaDBStore(dbPool *pgxpool.Pool, client *redis.Client) *PessoaDBStore {
	chPessoa := make(chan pessoa, 1000)
	return &PessoaDBStore{
		dbPool:       dbPool,
		chSyncPessoa: chPessoa,
		chSignStop:   make(chan struct{}, 1),
		cache:        NewRedisCache(client),
	}
}

func (p *PessoaDBStore) Add(ctx context.Context, pes pessoa) error {
	apelidoKey := fmt.Sprintf("apelido:%s", pes.Apelido)

	// try to add the apelido to the cache
	// if it already exists, we skip the insert
	if err := p.cache.Add(ctx, apelidoKey, true, 0); err != nil {
		return errAddSkipped
	}

	if err := p.cache.Add(ctx, pes.UID.String(), pes, 0); err != nil {
		return errAddSkipped
	}

	p.chSyncPessoa <- pes
	return nil
}

func (p *PessoaDBStore) Get(ctx context.Context, id uuid.UUID) (pessoa, error) {
	var pes pessoa

	if found, _ := p.cache.Get(ctx, id.String(), &pes); found {
		return pes, nil
	}

	query := "select Apelido, UID, Nome, to_char(Nascimento, 'YYYY-MM-DD'), Stack from pessoas where UID = $1;"
	err := p.dbPool.QueryRow(ctx, query, id).
		Scan(&pes.Apelido, &pes.UID, &pes.Nome, &pes.Nascimento, &pes.Stack)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return pessoa{}, errNotFound
		}

		return pessoa{}, fmt.Errorf("querying pessoa: %w", err)
	}

	return pes, nil
}

func (p *PessoaDBStore) Count(ctx context.Context) (int, error) {
	var count int
	err := p.dbPool.QueryRow(ctx, "select count(1) from pessoas;").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("querying count: %w", err)
	}
	return count, nil
}

func (p *PessoaDBStore) Search(ctx context.Context, term string) ([]pessoa, error) {
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

func (p *PessoaDBStore) StartSync(ctx context.Context) error {
	log.Info().Msg("Starting sync")

	go p.sync(ctx)
	return nil
}

func (p *PessoaDBStore) sync(ctx context.Context) {
	ctx = context.WithoutCancel(ctx)
	for {
		select {
		case <-p.chSignStop:
			log.Info().Msg("Sync Pessoas: force stopped")
		case pes, ok := <-p.chSyncPessoa:
			if !ok {
				log.Info().Msg("Sync Pessoas: stopped")
				return
			}

			terms := fmt.Sprintf("%s %s %s", strings.ToLower(pes.Apelido), strings.ToLower(pes.Nome), strings.ToLower(strings.Join(pes.Stack, " ")))
			insert := `INSERT INTO pessoas (Apelido, UID, Nome, Nascimento, Stack, search_terms) VALUES
    				($1, $2, $3, $4, $5, $6) ON CONFLICT DO NOTHING  returning uid;`

			_, err := p.dbPool.
				Exec(ctx, insert, pes.Apelido, pes.UID, pes.Nome, pes.Nascimento, pes.Stack, terms)

			if err != nil {
				log.Error().Err(err).Msgf("Sync Pessoas: %s", pes.UID)
			}
		}
	}
}

func (p *PessoaDBStore) StopSync(ctx context.Context) {
	log.Info().Msg("Stopping sync...waiting 5 seconds to finish sync")
	close(p.chSyncPessoa)
	time.Sleep(5 * time.Second)

	close(p.chSignStop)
}

func NewRedisCache(client *redis.Client) *redisCache {
	return &redisCache{
		client: client,
	}
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
