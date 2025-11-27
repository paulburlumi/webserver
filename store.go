package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"
)

var ErrKeyNotFound = errors.New("key not found")

type store struct {
	kv jetstream.KeyValue
}

func newStore(ctx context.Context, js jetstream.JetStream, cfg jetstream.KeyValueConfig) (*store, error) {
	kv, err := js.CreateOrUpdateKeyValue(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("could not create or update key-value store: %w", err)
	}
	return &store{kv: kv}, nil
}

func (s *store) Get(ctx context.Context, key string) ([]byte, error) {
	entry, err := s.kv.Get(ctx, key)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return nil, ErrKeyNotFound
		}
		return nil, fmt.Errorf("could not get entry from kv: %w", err)
	}
	return entry.Value(), nil
}

func (s *store) Put(ctx context.Context, key string, value []byte) error {
	if _, err := s.kv.Put(ctx, key, value); err != nil {
		return fmt.Errorf("could not put entry to kv: %w", err)
	}
	return nil
}

func (s *store) Watch(ctx context.Context, key string, callback func([]byte) error) error {
	watcher, err := s.kv.Watch(ctx, key)
	if err != nil {
		return fmt.Errorf("could not create watcher: %w", err)
	}
	defer func() { _ = watcher.Stop() }()

	for {
		select {
		case <-ctx.Done():
			return nil
		case entry, ok := <-watcher.Updates():
			if !ok {
				return nil
			}
			if entry == nil {
				continue
			}
			if err := callback(entry.Value()); err != nil {
				return fmt.Errorf("could not handle update: %w", err)
			}
		}
	}
}
