// handlers/cookies.go
package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/simonlewi/levelmix/pkg/storage"
)

type CookieHandler struct {
	metadata storage.MetadataStorage
}

func NewCookieHandler(metadata storage.MetadataStorage) *CookieHandler {
	return &CookieHandler{
		metadata: metadata,
	}
}

type CookieConsent struct {
	Essential  bool      `json:"essential"`
	Analytics  bool      `json:"analytics"`
	Functional bool      `json:"functional"`
	Timestamp  time.Time `json:"timestamp"`
	Version    string    `json:"version"`
	UserAgent  string    `json:"user_agent,omitempty"`
	IPAddress  string    `json:"ip_address,omitempty"`
}

// HandleCookieConsent stores the user's cookie preferences
func (h *CookieHandler) HandleCookieConsent(c *gin.Context) {
	var consent CookieConsent
	if err := c.ShouldBindJSON(&consent); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid consent data"})
		return
	}

	// Add metadata
	consent.UserAgent = c.GetHeader("User-Agent")
	consent.IPAddress = c.ClientIP()
	consent.Timestamp = time.Now()

	// Ensure version is set
	if consent.Version == "" {
		consent.Version = "1.0"
	}

	// Store consent in cookie for client-side access
	consentJSON, _ := json.Marshal(consent)
	c.SetCookie("cookie_consent", string(consentJSON), 365*24*3600, "/", "", false, false) // Not HttpOnly so JS can read it

	// Store in database for compliance tracking
	var userID *string
	if userInterface, exists := c.Get("user"); exists {
		if user, ok := userInterface.(*storage.User); ok {
			userID = &user.ID
		}
	}

	// Create consent record for database storage
	record := storage.CookieConsentRecord{
		ID:             generateConsentID(),
		UserID:         userID,
		Essential:      consent.Essential,
		Analytics:      consent.Analytics,
		Functional:     consent.Functional,
		ConsentVersion: consent.Version,
		UserAgent:      consent.UserAgent,
		IPAddress:      consent.IPAddress,
		CreatedAt:      consent.Timestamp,
	}

	// Store consent record in database using the storage interface
	if err := h.metadata.StoreCookieConsent(c.Request.Context(), record); err != nil {
		log.Printf("Failed to store cookie consent: %v", err)
		// Don't fail the request, but log the error
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

// GetLatestConsent retrieves the latest consent record for a user
func (h *CookieHandler) GetLatestConsent(c *gin.Context) {
	userID := c.Param("userID")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID required"})
		return
	}

	record, err := h.metadata.GetLatestConsent(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No consent record found"})
		return
	}

	consent := CookieConsent{
		Essential:  record.Essential,
		Analytics:  record.Analytics,
		Functional: record.Functional,
		Version:    record.ConsentVersion,
		UserAgent:  record.UserAgent,
		IPAddress:  record.IPAddress,
		Timestamp:  record.CreatedAt,
	}

	c.JSON(http.StatusOK, consent)
}

// GetUserConsentHistory returns all consent records for a user (for GDPR compliance)
func (h *CookieHandler) GetUserConsentHistory(c *gin.Context) {
	userID := c.Param("userID")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID required"})
		return
	}

	// Only allow users to access their own consent history or admins
	if userInterface, exists := c.Get("user"); exists {
		if user, ok := userInterface.(*storage.User); ok {
			if user.ID != userID {
				c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
				return
			}
		}
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	records, err := h.metadata.GetUserConsentHistory(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch consent history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"consents": records})
}

// DeleteUserConsentData deletes all consent data for a user (for GDPR right to erasure)
func (h *CookieHandler) DeleteUserConsentData(c *gin.Context) {
	userID := c.Param("userID")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID required"})
		return
	}

	// Only allow users to delete their own consent data or admins
	if userInterface, exists := c.Get("user"); exists {
		if user, ok := userInterface.(*storage.User); ok {
			if user.ID != userID {
				c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
				return
			}
		}
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	if err := h.metadata.DeleteUserConsentData(c.Request.Context(), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete consent data"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "Consent data deleted successfully"})
}

// ShowCookiePolicy displays the cookie policy page
func (h *CookieHandler) ShowCookiePolicy(c *gin.Context) {
	c.HTML(http.StatusOK, "cookie-policy.html", GetTemplateData(c, gin.H{
		"CurrentPage": "cookie-policy",
		"PageTitle":   "Cookie Policy",
	}))
}

// ShowPrivacyPolicy displays the privacy policy page
func (h *CookieHandler) ShowPrivacyPolicy(c *gin.Context) {
	c.HTML(http.StatusOK, "privacy-policy.html", GetTemplateData(c, gin.H{
		"CurrentPage": "privacy-policy",
		"PageTitle":   "Privacy Policy",
	}))
}

// generateConsentID generates a unique ID for consent records
func generateConsentID() string {
	return generateID() // Use your existing generateID function from other handlers
}
