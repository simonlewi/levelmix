package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/simonlewi/levelmix/pkg/storage"
)

type PaymentSuccessHandler struct{}

func NewPaymentSuccessHandler() *PaymentSuccessHandler {
	return &PaymentSuccessHandler{}
}

// ShowPaymentSuccess displays the payment success page
func (h *PaymentSuccessHandler) ShowPaymentSuccess(c *gin.Context) {
	// Get session_id from query params (provided by Stripe)
	sessionID := c.Query("session_id")

	// Get user data to determine subscription tier
	tierName := "Premium" // Default to Premium
	if userInterface, exists := c.Get("user"); exists {
		user := userInterface.(*storage.User)
		tierName = getTierName(user.SubscriptionTier)
	}

	templateData := GetTemplateData(c, gin.H{
		"CurrentPage": "payment-success",
		"PageTitle":   "Payment Successful",
		"SessionID":   sessionID,
		"TierName":    tierName,
	})

	c.HTML(http.StatusOK, "payment-success.html", templateData)
}
