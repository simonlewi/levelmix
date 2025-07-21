package handlers

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func AccessControlMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		accessPassword := os.Getenv("BETA_KEY")
		if accessPassword == "" {
			// No access control configured, allow through
			c.Next()
			return
		}

		// Check if user has already provided the password (stored in session/cookie)
		hasAccess, err := c.Cookie("access_granted")
		if err == nil && hasAccess == "true" {
			c.Next()
			return
		}

		// Check if this is the access password submission
		if c.Request.Method == "POST" && c.Request.URL.Path == "/access" {
			providedPassword := c.PostForm("password")
			if providedPassword == accessPassword {
				// Set access cookie (valid for 24 hours)
				c.SetCookie("access_granted", "true", 86400, "/", "", false, true)

				redirect := c.PostForm("redirect")
				if redirect == "" {
					redirect = "/"
				}
				c.Redirect(http.StatusSeeOther, redirect)
				c.Abort()
				return
			} else {
				// Wrong password, show access form with error
				c.HTML(http.StatusOK, "access.html", gin.H{
					"CurrentPage": "access",
					"error":       "Invalid password",
					"redirect":    c.PostForm("redirect"),
				})
				c.Abort()
				return
			}
		}

		// Show access form
		c.HTML(http.StatusOK, "access.html", gin.H{
			"CurrentPage": "access",
			"redirect":    c.Request.URL.Path,
		})
		c.Abort()
	}
}

// ShowAccessForm displays the password entry form
func ShowAccessForm(c *gin.Context) {
	c.HTML(http.StatusOK, "access.html", gin.H{
		"CurrentPage": "access",
		"redirect":    c.Query("redirect"),
	})
}
