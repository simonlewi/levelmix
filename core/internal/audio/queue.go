package audio

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
)

const (
	TypeAudioProcess = "audio:process"
	TypeAudioAnalyze = "audio:analyze"
	QueuePremium     = "audio_premium"
	QueueStandard    = "audio_standard"
)

type ProcessTask struct {
	JobID      string  `json:"job_id"`
	FileID     string  `json:"file_id"`
	TargetLUFS float64 `json:"target_lufs"`
	UserID     string  `json:"user_id"`
	IsPremium  bool    `json:"is_premium"`
}

func NewQueue(redisAddr string) *asynq.Client {
	return asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})
}

func EnqueueProcessing(ctx context.Context, client *asynq.Client, task ProcessTask) error {
	payload, err := json.Marshal(task)
	if err != nil {
		return err
	}

	queueName := QueueStandard
	if task.IsPremium {
		queueName = QueuePremium
	}

	opts := []asynq.Option{
		asynq.Queue(queueName),
		asynq.Timeout(30 * time.Minute),
		asynq.Retention(24 * time.Hour),
		asynq.MaxRetry(3),
	}

	_, err = client.EnqueueContext(ctx, asynq.NewTask(TypeAudioProcess, payload), opts...)
	return err
}
