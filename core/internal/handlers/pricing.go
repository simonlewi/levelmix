package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type PricingHandler struct{}

func NewPricingHandler() *PricingHandler {
	return &PricingHandler{}
}

func (h *PricingHandler) ShowPricing(c *gin.Context) {
	templateData := gin.H{
		"CurrentPage": "pricing",
		"PageTitle":   "Pricing",
	}

	// IMPORTANT: Use GetTemplateData to add common variables like IsLoggedIn
	templateData = GetTemplateData(c, templateData)

	c.HTML(http.StatusOK, "pricing.html", templateData)
}
