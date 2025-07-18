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
	c.HTML(http.StatusOK, "pricing.html", gin.H{
		"CurrentPage": "pricing",
		"PageTitle":   "Pricing",
	})
}
