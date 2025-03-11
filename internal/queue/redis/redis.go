package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"gitlab.com/pennersr/shove/internal/queue"
)

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
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		panic(err)
	}

	client := redis.NewClient(opt)
	return &redisQueueFactory{client: client}
}

func (f *redisQueueFactory) NewQueue(id string) (queue.Queue, error) {
	return &redisQueue{
		client: f.client,
		key:    fmt.Sprintf("shove:queue:%s", id),
	}, nil
}

func (q *redisQueue) Queue(data []byte) error {
	ctx := context.Background()
	return q.client.LPush(ctx, q.key, data).Err()
}

func (q *redisQueue) Get(ctx context.Context) (queue.QueuedMessage, error) {
	result := q.client.BRPop(ctx, 0, q.key)
	if result.Err() != nil {
		return nil, result.Err()
	}

	values := result.Val()
	if len(values) != 2 {
		return nil, fmt.Errorf("unexpected redis response")
	}

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
