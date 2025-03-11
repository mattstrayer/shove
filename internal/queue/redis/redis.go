package redis

import (
	"context"
	"fmt"
	"log"

	"github.com/mattstrayer/shove/internal/queue"
	"github.com/redis/go-redis/v9"
)

// ListName returns the Redis list name for a given service ID
func ListName(id string) string {
	return fmt.Sprintf("shove:%s", id)
}

type redisQueue struct {
	client *redis.Client
	key    string
}

type redisQueueFactory struct {
	client *redis.Client
}

type queuedMessage struct {
	data []byte
	id   string
}

func (m queuedMessage) Message() []byte {
	return m.data
}

// NewQueueFactory creates a new Redis queue factory
func NewQueueFactory(redisURL string) queue.QueueFactory {
	log.Printf("Connecting to Redis at: %s", redisURL)
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		panic(err)
	}

	client := redis.NewClient(opt)

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("Redis connection error: %v", err)
		panic(err)
	}
	log.Printf("Successfully connected to Redis")

	return &redisQueueFactory{client: client}
}

func (f *redisQueueFactory) NewQueue(id string) (queue.Queue, error) {
	key := fmt.Sprintf("shove:%s", id)
	log.Printf("Creating new Redis queue with key: %s", key)
	return &redisQueue{
		client: f.client,
		key:    key,
	}, nil
}

func (q *redisQueue) Queue(data []byte) error {
	ctx := context.Background()
	log.Printf("Pushing message to queue: %s", q.key)
	return q.client.LPush(ctx, q.key, data).Err()
}

func (q *redisQueue) Get(ctx context.Context) (queue.QueuedMessage, error) {
	log.Printf("Waiting for message from queue: %s", q.key)
	result := q.client.BRPop(ctx, 0, q.key)
	if result.Err() != nil {
		log.Printf("Error getting message from queue %s: %v", q.key, result.Err())
		return nil, result.Err()
	}

	values := result.Val()
	if len(values) != 2 {
		return nil, fmt.Errorf("unexpected redis response")
	}

	log.Printf("Received message from queue: %s", q.key)
	return &queuedMessage{
		data: []byte(values[1]),
		id:   values[1],
	}, nil
}

func (q *redisQueue) Remove(msg queue.QueuedMessage) error {
	// Message is already removed by BRPop
	return nil
}

func (q *redisQueue) Requeue(msg queue.QueuedMessage) error {
	ctx := context.Background()
	return q.client.LPush(ctx, q.key, msg.Message()).Err()
}

func (q *redisQueue) Shutdown() error {
	return nil
}
