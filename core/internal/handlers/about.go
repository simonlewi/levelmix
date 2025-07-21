package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type AboutHandler struct{}

func NewAboutHandler() *AboutHandler {
	return &AboutHandler{}
}

func (h *AboutHandler) ShowAbout(c *gin.Context) {
	templateData := gin.H{
		"CurrentPage": "about",
		"PageTitle":   "About Us",
	}

	// IMPORTANT: Use GetTemplateData to add common variables like IsLoggedIn
	templateData = GetTemplateData(c, templateData)

	c.HTML(http.StatusOK, "about.html", templateData)
}
