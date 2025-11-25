package presence

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Store coordinates online presence info using Redis so every gateway replica
// can share state.
type Store struct {
	client    *redis.Client
	gatewayID string
	ttl       time.Duration
}

// NewStore dials Redis using the url (redis://host:port/db) and prepares a presence store.
func NewStore(redisURL, gatewayID string, ttl time.Duration) (*Store, error) {
	if redisURL == "" {
		return nil, fmt.Errorf("redis url is required for presence store")
	}
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis url: %w", err)
	}

	client := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return &Store{client: client, gatewayID: gatewayID, ttl: ttl}, nil
}

// MarkOnline stores the gateway ID for the given user and refreshes TTL.
func (s *Store) MarkOnline(user string) error {
	if s == nil || s.client == nil || user == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.client.Set(ctx, s.key(user), s.gatewayID, s.ttl).Err()
}

// MarkOffline removes the user presence entry.
func (s *Store) MarkOffline(user string) error {
	if s == nil || s.client == nil || user == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.client.Del(ctx, s.key(user)).Err()
}

// GetGateway returns which gateway currently hosts the user (if any).
func (s *Store) GetGateway(user string) (string, error) {
	if s == nil || s.client == nil || user == "" {
		return "", nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	res, err := s.client.Get(ctx, s.key(user)).Result()
	if err == redis.Nil {
		return "", nil
	}
	return res, err
}

// Close releases the redis client.
func (s *Store) Close() {
	if s == nil || s.client == nil {
		return
	}
	_ = s.client.Close()
}

func (s *Store) key(user string) string {
	return "presence:" + strings.ToLower(strings.TrimSpace(user))
}
