package shove

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client ...
type Client interface {
	PushRaw(serviceID string, data []byte) (err error)
}

type redisClient struct {
	client *redis.Client
}

// NewRedisClient ...
func NewRedisClient(redisURL string) Client {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		panic(err)
	}

	// Configure connection timeouts
	opt.ReadTimeout = 10 * time.Second  // Timeout for read operations
	opt.WriteTimeout = 10 * time.Second // Timeout for write operations
	opt.DialTimeout = 5 * time.Second   // Timeout for establishing connections

	client := redis.NewClient(opt)
	return &redisClient{
		client: client,
	}
}

func queueName(id string) string {
	return fmt.Sprintf("shove:%s", id)
}

// PushRaw ...
func (rc *redisClient) PushRaw(id string, data []byte) (err error) {
	waitingList := queueName(id)
	ctx := context.Background()
	return rc.client.LPush(ctx, waitingList, data).Err()
}
