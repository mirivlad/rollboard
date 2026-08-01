package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const redisRoomEventsChannel = "rollboard:room-events:v1"

// RedisBackplane distributes room events to every Rollboard application replica.
type RedisBackplane struct {
	client *redis.Client

	mu         sync.Mutex
	pubsub     *redis.PubSub
	closeOnce  sync.Once
	subscribed bool
}

// NewRedisBackplane verifies that Redis is available before the application
// begins accepting realtime room connections.
func NewRedisBackplane(ctx context.Context, rawURL string) (*RedisBackplane, error) {
	options, err := redis.ParseURL(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse Redis URL: %w", err)
	}
	client := redis.NewClient(options)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping Redis: %w", err)
	}
	return &RedisBackplane{client: client}, nil
}

func (b *RedisBackplane) Publish(ctx context.Context, event BackplaneEvent) error {
	raw, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode backplane event: %w", err)
	}
	if err := b.client.Publish(ctx, redisRoomEventsChannel, raw).Err(); err != nil {
		return fmt.Errorf("publish Redis room event: %w", err)
	}
	return nil
}

func (b *RedisBackplane) Subscribe(ctx context.Context, callback func(BackplaneEvent)) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subscribed {
		return fmt.Errorf("Redis room event subscription already started")
	}
	pubsub := b.client.Subscribe(ctx, redisRoomEventsChannel)
	if _, err := pubsub.Receive(ctx); err != nil {
		_ = pubsub.Close()
		return fmt.Errorf("subscribe to Redis room events: %w", err)
	}
	b.pubsub = pubsub
	b.subscribed = true
	go b.receive(ctx, pubsub, callback)
	return nil
}

func (b *RedisBackplane) receive(ctx context.Context, pubsub *redis.PubSub, callback func(BackplaneEvent)) {
	for {
		message, err := pubsub.ReceiveMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("receive Redis room event: %v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(250 * time.Millisecond):
			}
			continue
		}
		var event BackplaneEvent
		if err := json.Unmarshal([]byte(message.Payload), &event); err != nil {
			log.Printf("discard malformed Redis room event: %v", err)
			continue
		}
		callback(event)
	}
}

func (b *RedisBackplane) Close() error {
	if b == nil {
		return nil
	}
	var closeErr error
	b.closeOnce.Do(func() {
		b.mu.Lock()
		if b.pubsub != nil {
			closeErr = b.pubsub.Close()
		}
		b.mu.Unlock()
		if err := b.client.Close(); err != nil && closeErr == nil {
			closeErr = err
		}
	})
	return closeErr
}
