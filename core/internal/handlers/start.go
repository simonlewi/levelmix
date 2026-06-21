package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type StartHandler struct{}

func NewStartHandler() *StartHandler {
	return &StartHandler{}
}

func (h *StartHandler) ShowStart(c *gin.Context) {
	templateData := gin.H{
		"CurrentPage": "start",
		"PageTitle":   "No More Volume Jumps",
	}

	// IMPORTANT: Use GetTemplateData to add common variables like AppVersion
	templateData = GetTemplateData(c, templateData)

	c.HTML(http.StatusOK, "start.html", templateData)
}
