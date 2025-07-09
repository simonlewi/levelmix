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
	c.HTML(http.StatusOK, "about.html", gin.H{
		"CurrentPage": "about",
	})
}
