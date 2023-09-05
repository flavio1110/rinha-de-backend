package pessoas

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

func NewRedisCache(ctx context.Context, redisAddr string) (*redisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "", // no password set
		DB:       0,  // use default DB

	})

	if status := client.Ping(ctx); status.Err() != nil {
		return nil, fmt.Errorf("pinging redis: %w", status.Err())
	}

	return &redisCache{
		client: client,
	}, nil
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
