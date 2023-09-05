package pessoas

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

const maxBatchSize = 5000

type Cache interface {
	Get(ctx context.Context, key string, dest any) (bool, error)
	Add(ctx context.Context, key string, value any, expiration time.Duration) error
}

type PessoaDBStore struct {
	dbPool       *pgxpool.Pool
	chBulk       chan []pessoa
	chSyncPessoa chan pessoa
	chSignStop   chan struct{}
	syncInterval time.Duration
	cache        Cache
}

type DBConfig struct {
	DbURL   string
	MaxConn int32
	MinConn int32
}

func NewPessoaDBStore(
	config DBConfig,
	cache Cache,
	syncInterval time.Duration) (*PessoaDBStore, func(), error) {

	pgxConfig, err := pgxpool.ParseConfig(config.DbURL)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing db url: %w", err)
	}

	pgxConfig.MinConns = config.MinConn
	pgxConfig.MaxConns = config.MaxConn
	pgxConfig.MaxConnIdleTime = time.Minute * 3

	dbPool, err := pgxpool.NewWithConfig(context.Background(), pgxConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("creating connection pool: %w", err)
	}

	return &PessoaDBStore{
		dbPool:       dbPool,
		chSignStop:   make(chan struct{}, 1),
		chSyncPessoa: make(chan pessoa, maxBatchSize),
		chBulk:       make(chan []pessoa, 10),
		cache:        cache,
		syncInterval: syncInterval,
	}, dbPool.Close, nil
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

	var stack string
	var date time.Time

	query := "select Apelido, UID, Nome, Nascimento, Stack from pessoas where UID = $1;"
	err := p.dbPool.QueryRow(ctx, query, id).
		Scan(&pes.Apelido, &pes.UID, &pes.Nome, date, stack)

	if errors.Is(err, pgx.ErrNoRows) {
		return pessoa{}, errNotFound
	}

	if err != nil {
		return pessoa{}, fmt.Errorf("querying pessoa: %w", err)
	}

	pes.Stack = strings.Split(stack, "|")
	pes.Nascimento = date.Format("2006-01-02")

	// no need to check error here
	_ = p.cache.Add(ctx, pes.UID.String(), pes, 24*time.Hour)

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
	searhTermLower := strings.ToLower(term)
	if found, _ := p.cache.Get(ctx, searhTermLower, &pessoas); found {
		return pessoas, nil
	}

	query := `
	select Apelido, UID, Nome, Nascimento, Stack 
	   from pessoas
	     where search_terms like $1 limit 50;`

	rows, err := p.dbPool.Query(ctx, query, "%"+searhTermLower+"%")
	if err != nil {
		return nil, fmt.Errorf("querying pessoas: %w", err)
	}
	defer rows.Close()

	var date time.Time
	var stack string
	var pes pessoa

	for rows.Next() {
		err := rows.Scan(&pes.Apelido, &pes.UID, &pes.Nome, &date, &stack)
		if err != nil {
			return nil, fmt.Errorf("scanning pessoa: %w", err)
		}
		pes.Stack = strings.Split(stack, "|")
		pes.Nascimento = date.Format("2006-01-02")
		pessoas = append(pessoas, pes)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating over pessoas: %w", err)
	}
	if pessoas == nil {
		pessoas = []pessoa{}
	}

	_ = p.cache.Add(ctx, searhTermLower, pessoas, 60*time.Second)

	return pessoas, nil
}

func (p *PessoaDBStore) StartSync(ctx context.Context) error {
	log.Info().Msg("Starting sync")

	go p.processInserts()
	go p.syncBulks()
	return nil
}

func (p *PessoaDBStore) processInserts() {
	maxBatchSize := 5000
	bulk := make([]pessoa, 0, maxBatchSize)
	ticker := time.NewTicker(p.syncInterval)

	for {
		select {
		case <-p.chSignStop:
			close(p.chBulk)
			log.Info().Msg("Sync Pessoas: force stopped")
			return
		case pes := <-p.chSyncPessoa:
			bulk = append(bulk, pes)
		case <-ticker.C:
			if len(bulk) > 0 {
				p.chBulk <- bulk
				bulk = make([]pessoa, 0, maxBatchSize)
			}
		}
	}
}

func (p *PessoaDBStore) syncBulks() {
	for bulk := range p.chBulk {
		if err := p.bulkInsert(bulk); err != nil {
			log.Error().Err(err).Msg("Sync Pessoas: bulk insert")
		}
	}
}

func (p *PessoaDBStore) bulkInsert(bulk []pessoa) error {
	var inputRows [][]interface{}

	for i := range bulk {
		inputRows = append(inputRows, []interface{}{
			bulk[i].Apelido,
			bulk[i].UID,
			bulk[i].Nome,
			bulk[i].Nascimento,
			strings.Join(bulk[i].Stack, "|"),
			fmt.Sprintf("%s %s %s", strings.ToLower(bulk[i].Apelido), strings.ToLower(bulk[i].Nome), strings.ToLower(strings.Join(bulk[i].Stack, " "))),
		})
	}

	copyCount, err := p.dbPool.CopyFrom(context.Background(), pgx.Identifier{"pessoas"},
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
