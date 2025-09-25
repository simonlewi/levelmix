// core/internal/audio/queue.go (Updated)
package audio

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/hibiken/asynq"
)

func NewQueue(redisAddr string) *asynq.Client {
	return asynq.NewClient(asynq.RedisClientOpt{
		Addr:     redisAddr,
		Password: os.Getenv("REDIS_PASSWORD"),
	})
}

type QueueManager struct {
	client *asynq.Client
}

func NewQueueManager(redisAddr string) *QueueManager {
	return &QueueManager{
		client: asynq.NewClient(asynq.RedisClientOpt{
			Addr:     redisAddr,
			Password: os.Getenv("REDIS_PASSWORD"),
		}),
	}
}

// Shutdown gracefully closes the queue connection
func (qm *QueueManager) Shutdown() error {
	return qm.client.Close()
}

func (qm *QueueManager) EnqueueProcessing(ctx context.Context, task ProcessTask) error {
	payload, err := json.Marshal(task)
	if err != nil {
		return err
	}

	// Determine queue based on processing mode and user tier
	queueName := QueueStandard
	if task.ProcessingMode == ModeFast || task.FastMode {
		queueName = QueueFast // Fast processing gets its own queue
	} else if task.IsPremium {
		queueName = QueuePremium
	}

	// Fast mode gets shorter timeout
	timeout := 30 * time.Minute
	if task.ProcessingMode == ModeFast || task.FastMode {
		timeout = 10 * time.Minute
	}

	t := asynq.NewTask(TypeAudioProcess, payload)
	_, err = qm.client.EnqueueContext(ctx, t,
		asynq.Queue(queueName),
		asynq.Timeout(timeout),
		asynq.Retention(24*time.Hour),
		asynq.MaxRetry(3),
	)
	return err
}
