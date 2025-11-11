package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type HowToUseHandler struct{}

func NewHowToUseHandler() *HowToUseHandler {
	return &HowToUseHandler{}
}

func (h *HowToUseHandler) ShowHowToUse(c *gin.Context) {
	templateData := gin.H{
		"CurrentPage": "how-to-use",
		"PageTitle":   "How To Use",
	}

	// IMPORTANT: Use GetTemplateData to add common variables like IsLoggedIn
	templateData = GetTemplateData(c, templateData)

	c.HTML(http.StatusOK, "how-to-use.html", templateData)
}
