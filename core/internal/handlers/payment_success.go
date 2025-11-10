package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type PaymentSuccessHandler struct{}

func NewPaymentSuccessHandler() *PaymentSuccessHandler {
	return &PaymentSuccessHandler{}
}

// ShowPaymentSuccess displays the payment success page
func (h *PaymentSuccessHandler) ShowPaymentSuccess(c *gin.Context) {
	// Get session_id from query params (provided by Stripe)
	sessionID := c.Query("session_id")

	templateData := GetTemplateData(c, gin.H{
		"CurrentPage": "payment-success",
		"PageTitle":   "Payment Successful",
		"SessionID":   sessionID,
	})

	c.HTML(http.StatusOK, "payment-success.html", templateData)
}
