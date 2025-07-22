// handlers/account.go
package handlers

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/simonlewi/levelmix/pkg/email"
	"github.com/simonlewi/levelmix/pkg/storage"
	"golang.org/x/crypto/bcrypt"
)

type AccountHandler struct {
	metadata     storage.MetadataStorage
	audioStorage storage.AudioStorage
	emailService email.EmailService
}

func NewAccountHandler(metadata storage.MetadataStorage, audioStorage storage.AudioStorage) *AccountHandler {
	return &AccountHandler{
		metadata:     metadata,
		audioStorage: audioStorage,
		emailService: email.NewMockEmailService(), // Use mock for simplicity, can be replaced with real service
	}
}

// ShowDeleteConfirmation shows the delete account confirmation page
func (h *AccountHandler) ShowDeleteConfirmation(c *gin.Context) {
	userInterface, exists := c.Get("user")
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	user := userInterface.(*storage.User)

	c.HTML(http.StatusOK, "delete-account.html", GetTemplateData(c, gin.H{
		"CurrentPage": "delete-account",
		"PageTitle":   "Delete Account",
		"user":        user,
	}))
}

// HandleDeleteAccount processes the account deletion
func (h *AccountHandler) HandleDeleteAccount(c *gin.Context) {
	userInterface, exists := c.Get("user")
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	user := userInterface.(*storage.User)

	// Verify password for security
	password := c.PostForm("password")
	if password == "" {
		c.Redirect(http.StatusSeeOther, "/account/delete?error=password_required")
		return
	}

	// Get user from storage to verify password
	fullUser, err := h.metadata.GetUser(c.Request.Context(), user.ID)
	if err != nil {
		log.Printf("Failed to get user for deletion: %v", err)
		c.Redirect(http.StatusSeeOther, "/account/delete?error=server_error")
		return
	}

	// For OAuth users, we can't verify password
	if fullUser.AuthProvider != "email" {
		// For OAuth users, we might want to implement a different verification method
		// For now, we'll proceed with deletion after confirmation
	} else {
		// Verify password for email users
		if fullUser.PasswordHash == nil {
			c.Redirect(http.StatusSeeOther, "/account/delete?error=invalid_password")
			return
		}

		err = bcrypt.CompareHashAndPassword([]byte(*fullUser.PasswordHash), []byte(password))
		if err != nil {
			c.Redirect(http.StatusSeeOther, "/account/delete?error=invalid_password")
			return
		}
	}

	// Delete user's audio files from S3
	jobs, err := h.metadata.GetUserJobs(c.Request.Context(), user.ID, 1000, 0) // Get all jobs
	if err == nil {
		for _, job := range jobs {
			// Delete original and processed files from S3
			h.audioStorage.Delete(c.Request.Context(), "uploads/"+job.AudioFileID)
			h.audioStorage.Delete(c.Request.Context(), "processed/"+job.AudioFileID)
		}
	}

	// Delete user data from database
	// This should cascade delete all related data due to foreign key constraints
	err = h.deleteUserData(c.Request.Context(), user.ID)
	if err != nil {
		log.Printf("Failed to delete user data: %v", err)
		c.Redirect(http.StatusSeeOther, "/account/delete?error=deletion_failed")
		return
	}

	// Clear session
	h.clearSession(c)

	if h.emailService != nil {
		go func() {
			if err := h.emailService.SendAccountDeleted(context.Background(), fullUser.Email); err != nil {
				log.Printf("Failed to send account deletion email: %v", err)
			}
		}()
	}

	// Redirect to home with success message
	c.Redirect(http.StatusSeeOther, "/?account_deleted=true")
}

func (h *AccountHandler) deleteUserData(ctx context.Context, userID string) error {
	// Since we have CASCADE DELETE set up in the database,
	// deleting the user should automatically delete all related data
	// But we'll add this method to handle any additional cleanup if needed

	// The actual deletion would be implemented in the metadata storage
	// For now, we'll just log
	log.Printf("Deleting all data for user: %s", userID)

	return h.metadata.DeleteUser(ctx, userID)
}

func (h *AccountHandler) clearSession(c *gin.Context) {
	c.SetCookie("session_token", "", -1, "/", "", false, true)
	c.SetCookie("user_id", "", -1, "/", "", false, true)
}
