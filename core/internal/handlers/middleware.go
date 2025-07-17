package handlers

import (
	"github.com/gin-gonic/gin"
)

// TemplateContext adds common template variables to all requests
func TemplateContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if user is logged in
		userID, err := c.Cookie("user_id")
		isLoggedIn := err == nil && userID != ""
		
		// Set default template data in context
		c.Set("IsLoggedIn", isLoggedIn)
		c.Set("UserID", userID)
		
		c.Next()
	}
}

// GetTemplateData creates a gin.H with common template variables
func GetTemplateData(c *gin.Context, data gin.H) gin.H {
	// If data is nil, create new map
	if data == nil {
		data = gin.H{}
	}
	
	// Add common variables from context
	if isLoggedIn, exists := c.Get("IsLoggedIn"); exists {
		data["IsLoggedIn"] = isLoggedIn
	}
	
	if userID, exists := c.Get("UserID"); exists && userID != "" {
		data["UserID"] = userID
	}
	
	return data
}