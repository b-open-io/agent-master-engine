// Package redis provides a Redis-based storage implementation for the Agent Master Engine.
package redis

import (
	"context"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"
)

// Storage implements the engine.Storage interface using Redis
type Storage struct {
	client   *redis.Client
	ctx      context.Context
	prefix   string
	watchers map[string][]func([]byte)
	mu       sync.RWMutex
}

// New creates a new Redis-based storage
func New(client *redis.Client, prefix string) *Storage {
	return &Storage{
		client:   client,
		ctx:      context.Background(),
		prefix:   prefix,
		watchers: make(map[string][]func([]byte)),
	}
}

// Read reads data from Redis
func (s *Storage) Read(key string) ([]byte, error) {
	fullKey := s.prefix + ":" + key
	data, err := s.client.Get(s.ctx, fullKey).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	return data, err
}

// Write writes data to Redis
func (s *Storage) Write(key string, data []byte) error {
	fullKey := s.prefix + ":" + key
	err := s.client.Set(s.ctx, fullKey, data, 0).Err()
	
	// Notify watchers
	if err == nil {
		s.mu.RLock()
		if handlers, ok := s.watchers[key]; ok {
			for _, handler := range handlers {
				go handler(data)
			}
		}
		s.mu.RUnlock()
	}
	
	return err
}

// Delete removes data from Redis
func (s *Storage) Delete(key string) error {
	fullKey := s.prefix + ":" + key
	result := s.client.Del(s.ctx, fullKey)
	if result.Val() == 0 {
		return fmt.Errorf("key not found: %s", key)
	}
	return result.Err()
}

// List lists keys with given prefix
func (s *Storage) List(prefix string) ([]string, error) {
	pattern := s.prefix + ":" + prefix + "*"
	keys, err := s.client.Keys(s.ctx, pattern).Result()
	if err != nil {
		return nil, err
	}

	// Strip the storage prefix
	results := make([]string, 0, len(keys))
	prefixLen := len(s.prefix) + 1
	for _, key := range keys {
		if len(key) > prefixLen {
			results = append(results, key[prefixLen:])
		}
	}

	return results, nil
}

// Watch watches for changes to a key
func (s *Storage) Watch(key string, handler func([]byte)) (func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.watchers[key] = append(s.watchers[key], handler)

	// Return unsubscribe function
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		if handlers, ok := s.watchers[key]; ok {
			// Remove this handler
			for i, h := range handlers {
				if &h == &handler {
					s.watchers[key] = append(handlers[:i], handlers[i+1:]...)
					break
				}
			}

			// Clean up if no more handlers
			if len(s.watchers[key]) == 0 {
				delete(s.watchers, key)
			}
		}
	}, nil
}

// Close closes the Redis connection
func (s *Storage) Close() error {
	return s.client.Close()
}