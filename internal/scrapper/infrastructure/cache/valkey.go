package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type ValkeyClient struct {
	client redis.UniversalClient
	ttl    time.Duration
}

func NewValkeyClient(addresses []string, password string, ttl time.Duration, poolSize int,
	maxRetries int, minRetryBackoff, maxRetryBackoff, pingTime time.Duration, clusterMode bool) (*ValkeyClient, error) {
	var client redis.UniversalClient
	if clusterMode {
		client = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:           addresses,
			Password:        password,
			PoolSize:        poolSize,
			MaxRetries:      maxRetries,
			MinRetryBackoff: minRetryBackoff,
			MaxRetryBackoff: maxRetryBackoff,
		})
	} else {
		client = redis.NewClient(&redis.Options{
			Addr:            addresses[0],
			Password:        password,
			PoolSize:        poolSize,
			MaxRetries:      maxRetries,
			MinRetryBackoff: minRetryBackoff,
			MaxRetryBackoff: maxRetryBackoff,
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), pingTime)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Valkey: %w", err)
	}

	return &ValkeyClient{
		client: client,
		ttl:    ttl,
	}, nil
}

func (c *ValkeyClient) GetLinks(ctx context.Context, chatID int64) ([]domain.Link, error) {
	key := c.getKey(chatID)

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return []domain.Link{}, nil
		}
		return nil, fmt.Errorf("failed to get from cache: %w", err)
	}

	var links []domain.Link
	if err = json.Unmarshal(data, &links); err != nil {
		return nil, fmt.Errorf("cannot unmarshal data: %w", err)
	}

	return links, nil
}

// SetLinks сохраняет ссылки в кэш
func (c *ValkeyClient) SetLinks(ctx context.Context, chatID int64, links []domain.Link) error {
	key := c.getKey(chatID)

	dataBytes, err := json.Marshal(links)
	if err != nil {
		return fmt.Errorf("cannot unmarshal data: %w", err)
	}

	err = c.client.Set(ctx, key, dataBytes, c.ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to set cache: %w", err)
	}

	return nil
}

// InvalidateLinks удаляет кэш для чата
func (c *ValkeyClient) InvalidateLinks(ctx context.Context, chatID int64) error {
	key := c.getKey(chatID)

	err := c.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to invalidate cache: %w", err)
	}

	return nil
}

func (c *ValkeyClient) Close() error {
	err := c.client.Close()
	if err != nil {
		return fmt.Errorf("cannot close valkey client: %w", err)
	}

	return nil
}

func (c *ValkeyClient) getKey(chatID int64) string {
	return strconv.FormatInt(chatID, 10)
}
