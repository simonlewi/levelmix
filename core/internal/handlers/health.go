package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
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

	// Check database connectivity
	dbHealthy := true
	if err := h.checkDatabase(ctx); err != nil {
		dbHealthy = false
		health["checks"].(gin.H)["database"] = gin.H{
			"status": "unhealthy",
			"error":  err.Error(),
		}
	} else {
		health["checks"].(gin.H)["database"] = gin.H{
			"status": "healthy",
		}
	}

	// Add more checks as needed (Redis, S3, etc.)

	if !dbHealthy {
		health["status"] = "unhealthy"
		c.JSON(http.StatusServiceUnavailable, health)
		return
	}

	c.JSON(http.StatusOK, health)
}

func (h *HealthHandler) checkDatabase(ctx context.Context) error {
	// Simple database connectivity check
	// You might want to implement a Ping method in your storage interface
	_, err := h.metadata.GetUser(ctx, "health-check-non-existent-user")
	if err != nil && err.Error() != "sql: no rows in result set" {
		return err
	}
	return nil
}
