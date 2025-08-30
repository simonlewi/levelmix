package handlers

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/simonlewi/levelmix/pkg/storage"
)

type HealthHandler struct {
	metadata storage.MetadataStorage
	redis    string
}

func NewHealthHandler(metadata storage.MetadataStorage, redisURL string) *HealthHandler {
	return &HealthHandler{
		metadata: metadata,
		redis:    redisURL,
	}
}

func (h *HealthHandler) HealthCheck(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	health := gin.H{
		"status": "healthy",
		"checks": gin.H{},
	}

	overallHealthy := true

	// Check database connectivity
	if err := h.checkDatabase(ctx); err != nil {
		overallHealthy = false
		health["checks"].(gin.H)["database"] = gin.H{
			"status": "unhealthy",
			"error":  err.Error(),
		}
	} else {
		health["checks"].(gin.H)["database"] = gin.H{
			"status": "healthy",
		}
	}

	if err := h.checkRedis(ctx); err != nil {
		overallHealthy = false
		health["checks"].(gin.H)["redis"] = gin.H{
			"status": "unhealthy",
			"error":  err.Error(),
		}
	} else {
		health["checks"].(gin.H)["redis"] = gin.H{
			"status": "healthy",
		}
	}

	// Add more checks as needed (S3, etc.)

	if !overallHealthy {
		health["status"] = "unhealthy"
		c.JSON(http.StatusServiceUnavailable, health)
		return
	}

	c.JSON(http.StatusOK, health)
}

func (h *HealthHandler) checkDatabase(ctx context.Context) error {
	_, err := h.metadata.GetUser(ctx, "health-check-non-existent-user")
	if err != nil && err.Error() != "sql: no rows in result set" {
		return err
	}
	return nil
}

func (h *HealthHandler) checkRedis(ctx context.Context) error {
	addr := h.redis
	if addr == "" {
		addr = "localhost:6379"
	}

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})
	defer client.Close()

	// Test connection with PING
	_, err := client.Ping(ctx).Result()
	return err
}
