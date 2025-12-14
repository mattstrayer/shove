package redis

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/mattstrayer/shove/internal/queue"
	"github.com/redis/go-redis/v9"
)

const (
	// brPopTimeout is the timeout for BRPop operations
	// Using a shorter timeout allows us to check context cancellation and retry on connection errors
	brPopTimeout = 5 * time.Second
	// maxRetryDelay is the maximum delay between retries
	maxRetryDelay = 30 * time.Second
	// initialRetryDelay is the initial delay before first retry
	initialRetryDelay = 100 * time.Millisecond
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
	log.Printf("Connecting to Redis at: %s", opt.Addr)

	// Configure connection pool settings
	opt.PoolSize = 50                  // Increase from default 10
	opt.MinIdleConns = 10              // Maintain some minimum idle connections
	opt.PoolTimeout = time.Second * 30 // Increase timeout for getting connection from pool
	opt.ReadTimeout = 10 * time.Second  // Timeout for read operations
	opt.WriteTimeout = 10 * time.Second // Timeout for write operations
	opt.DialTimeout = 5 * time.Second   // Timeout for establishing connections

	client := redis.NewClient(opt)

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("Redis connection error: %v", err)
		panic(err)
	}

	// Log connection configuration
	poolStats := client.PoolStats()
	log.Printf("Successfully connected to Redis (pool: size=%d, min_idle=%d, read_timeout=%v, write_timeout=%v, dial_timeout=%v)",
		opt.PoolSize, opt.MinIdleConns, opt.ReadTimeout, opt.WriteTimeout, opt.DialTimeout)
	log.Printf("Redis connection pool stats: total=%d, idle=%d, stale=%d",
		poolStats.TotalConns, poolStats.IdleConns, poolStats.StaleConns)

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
	retryDelay := initialRetryDelay
	retryCount := 0
	wasRetrying := false
	for {
		// Check if context is cancelled
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Log connection status
		if wasRetrying {
			// Verify connection health before retrying
			pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			if err := q.client.Ping(pingCtx).Err(); err != nil {
				log.Printf("Redis connection health check failed for queue %s: %v", q.key, err)
				cancel()
			} else {
				log.Printf("Redis connection restored for queue %s after %d retry attempts", q.key, retryCount)
				wasRetrying = false
				retryCount = 0
				retryDelay = initialRetryDelay
			}
			cancel()
		}

		log.Printf("Waiting for message from queue: %s", q.key)

		// Use a timeout for BRPop to allow periodic context checks and connection health verification
		result := q.client.BRPop(ctx, brPopTimeout, q.key)

		if result.Err() != nil {
			// Check if context was cancelled
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}

			// Check if it's a connection error that we should retry
			err := result.Err()
			if isConnectionError(err) {
				retryCount++
				wasRetrying = true
				poolStats := q.client.PoolStats()
				log.Printf("Connection error getting message from queue %s: %v (retry %d, pool: total=%d idle=%d stale=%d, retrying in %v)",
					q.key, err, retryCount, poolStats.TotalConns, poolStats.IdleConns, poolStats.StaleConns, retryDelay)

				// Wait before retrying, respecting context cancellation
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(retryDelay):
					// Exponential backoff with max limit
					retryDelay = time.Duration(float64(retryDelay) * 1.5)
					if retryDelay > maxRetryDelay {
						retryDelay = maxRetryDelay
					}
					continue
				}
			}

			// For non-connection errors, return immediately
			log.Printf("Error getting message from queue %s: %v", q.key, err)
			return nil, err
		}

		// Reset retry state on success
		if wasRetrying {
			log.Printf("Successfully recovered connection for queue %s", q.key)
			wasRetrying = false
			retryCount = 0
		}
		retryDelay = initialRetryDelay

		values := result.Val()
		// BRPop returns empty slice on timeout (no messages), continue waiting
		if len(values) == 0 {
			continue
		}

		if len(values) != 2 {
			return nil, fmt.Errorf("unexpected redis response")
		}

		log.Printf("Received message from queue: %s", q.key)
		return &queuedMessage{
			data: []byte(values[1]),
			id:   values[1],
		}, nil
	}
}

// isConnectionError checks if an error is a connection-related error that should be retried
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "EOF") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "connection reset")
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
