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
	Add(ctx context.Context, key string, value any, expiration time.Duration) error
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
	if err := p.cache.Add(ctx, apelidoKey, true, 24*time.Hour); err != nil {
		return errAddSkipped
	}

	if err := p.cache.Add(ctx, pes.UID.String(), pes, 24*time.Hour); err != nil {
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

	if found, _ := p.cache.Get(ctx, term, &pessoas); found {
		return pessoas, nil
	}

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
	if pessoas == nil {
		pessoas = []pessoa{}
	}

	_ = p.cache.Add(ctx, term, pessoas, 60*time.Second)

	return pessoas, nil
}

func (p *PessoaDBStore) StartSync(ctx context.Context) error {
	log.Info().Msg("Starting sync")

	go p.sync(ctx)
	return nil
}

func (p *PessoaDBStore) sync(ctx context.Context) {
	ctx = context.WithoutCancel(ctx)
	var bulk []pessoa

	insertBulk := func(batch []pessoa) {
		if err := p.bulkInsert(ctx, batch); err != nil {
			log.Error().Err(err).Msg("Sync Pessoas: batch insert")
		}
	}

	ticker := time.NewTicker(5 * time.Second)

	for {
		select {
		case <-p.chSignStop:
			if len(bulk) > 0 {
				go insertBulk(bulk)
			}
			log.Info().Msg("Sync Pessoas: force stopped")
			return
		case pes, ok := <-p.chSyncPessoa:
			if !ok {
				if len(bulk) > 0 {
					go insertBulk(bulk)
				}

				log.Info().Msg("Sync Pessoas: stopped")
				return
			}
			bulk = append(bulk, pes)
			if len(bulk) >= 100 {
				go insertBulk(bulk)
				bulk = nil
			}
		case <-ticker.C:
			if len(bulk) > 0 {
				go insertBulk(bulk)
				bulk = nil
			}
		}
	}
}

func (p *PessoaDBStore) bulkInsert(ctx context.Context, bulk []pessoa) error {
	var inputRows [][]interface{}

	for i := range bulk {
		inputRows = append(inputRows, []interface{}{
			bulk[i].Apelido,
			bulk[i].UID,
			bulk[i].Nome,
			bulk[i].Nascimento,
			bulk[i].Stack,
			fmt.Sprintf("%s %s %s", strings.ToLower(bulk[i].Apelido), strings.ToLower(bulk[i].Nome), strings.ToLower(strings.Join(bulk[i].Stack, " "))),
		})
	}

	copyCount, err := p.dbPool.CopyFrom(ctx, pgx.Identifier{"pessoas"},
		[]string{"apelido", "uid", "nome", "nascimento", "stack", "search_terms"},
		pgx.CopyFromRows(inputRows))
	if err != nil {
		return fmt.Errorf("CopyFrom: %w", err)
	}
	if int(copyCount) != len(inputRows) {
		return fmt.Errorf("CopyFrom: expected %d rows to be copied, got %d", len(inputRows), copyCount)
	}
	return nil
}

func (p *PessoaDBStore) StopSync() {
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

func (r redisCache) Add(ctx context.Context, key string, value any, expiration time.Duration) error {
	val, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshaling value: %w", err)
	}

	added := r.client.SetNX(ctx, key, val, expiration).Val()
	if !added {
		return errors.New("value not added")
	}

	return nil
}
