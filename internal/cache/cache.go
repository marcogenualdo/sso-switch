package cache

import (
	"context"
	"errors"
	"time"

	"github.com/marcogenualdo/sso-switch/internal/config"
)

var ErrNotFound = errors.New("key not found")

type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	Close() error
}

func New(cfg config.CacheConfig) (Cache, error) {
	switch cfg.Type {
	case "memory":
		return NewMemoryCache(), nil
	case "redis":
		if cfg.Redis == nil {
			return nil, errors.New("redis config is required for redis cache type")
		}
		return NewRedisCache(*cfg.Redis)
	default:
		return nil, errors.New("unsupported cache type: " + cfg.Type)
	}
}
