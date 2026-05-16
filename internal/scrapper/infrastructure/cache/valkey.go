package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type ValkeyClient struct {
	scanCount int64
	client    redis.UniversalClient
	ttl       time.Duration
}

func NewValkeyClient(addresses []string, password string, ttl time.Duration, poolSize int,
	maxRetries int, minRetryBackoff, maxRetryBackoff, pingTime time.Duration, clusterMode bool, scanCount int64) (*ValkeyClient, error) {
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
		client:    client,
		ttl:       ttl,
		scanCount: scanCount,
	}, nil
}

func (c *ValkeyClient) GetLinks(ctx context.Context, chatID int64, limit, offset int) ([]domain.Link, error) {
	key := c.getKey(chatID, limit, offset)

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return []domain.Link{}, nil
		}
		return nil, fmt.Errorf("failed to get from cache: %w", err)
	}

	var links []domain.Link

	//nolint:musttag // Для кэша быстрее использовать сразу доменную модель иначе придется создавать промежуточную и переносить данные, на это уходит много ресурсов
	if err = json.Unmarshal(data, &links); err != nil {
		return nil, fmt.Errorf("cannot unmarshal data: %w", err)
	}

	return links, nil
}

// SetLinks сохраняет ссылки в кэш
func (c *ValkeyClient) SetLinks(ctx context.Context, chatID int64, limit, offset int, links []domain.Link) error {
	key := c.getKey(chatID, limit, offset)

	//nolint:musttag // Для кэша быстрее использовать сразу доменную модель иначе придется создавать промежуточную и переносить данные, на это уходит много ресурсов
	dataBytes, err := json.Marshal(links)
	if err != nil {
		return fmt.Errorf("cannot unmarshal data: %w", err)
	}

	err = c.client.Set(ctx, key, dataBytes, c.ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to set cache: %w", err)
	}

	setKey := fmt.Sprintf("chat:%d:keys", chatID)
	c.client.SAdd(ctx, setKey, key)
	c.client.Expire(ctx, setKey, c.ttl)

	return nil
}

// InvalidateLinks удаляет кэш для чата
func (c *ValkeyClient) InvalidateLinks(ctx context.Context, chatID int64) error {
	setKey := fmt.Sprintf("chat:%d:keys", chatID)

	keys, err := c.client.SMembers(ctx, setKey).Result()
	if err != nil {
		return fmt.Errorf("failed to get keys: %w", err)
	}

	if len(keys) > 0 {
		if err := c.client.Del(ctx, keys...).Err(); err != nil {
			return fmt.Errorf("failed to delete keys: %w", err)
		}
		if err := c.client.Del(ctx, setKey).Err(); err != nil {
			return fmt.Errorf("failed to delete set: %w", err)
		}
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

func (c *ValkeyClient) getKey(chatID int64, limit, offset int) string {
	key := strings.Builder{}

	key.WriteString(strconv.FormatInt(chatID, 10))
	key.WriteString("_")
	key.WriteString(strconv.Itoa(limit))
	key.WriteString("_")
	key.WriteString(strconv.Itoa(offset))

	return key.String()
}

// getPattern возвращает паттерн для поиска всех ключей чата
func (c *ValkeyClient) getPattern(chatID int64) string {
	return strconv.FormatInt(chatID, 10) + "_*"
}
