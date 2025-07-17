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
		
		// Set default template data
		c.Set("IsLoggedIn", isLoggedIn)
		
		// Override HTML render to include common data
		originalHTML := c.HTML
		c.HTML = func(code int, name string, obj interface{}) {
			if data, ok := obj.(gin.H); ok {
				// Add common data if not already present
				if _, exists := data["IsLoggedIn"]; !exists {
					data["IsLoggedIn"] = isLoggedIn
				}
			}
			originalHTML(code, name, obj)
		}
		
		c.Next()
	}
}

// Usage in main.go:
// r.Use(handlers.TemplateContext())
// This ensures all templates have access to IsLoggedIn status