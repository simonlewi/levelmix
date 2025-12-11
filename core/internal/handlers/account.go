// handlers/account.go
package handlers

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

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

// ShowChangeEmail displays the change email form
func (h *AccountHandler) ShowChangeEmail(c *gin.Context) {
	userInterface, exists := c.Get("user")
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	user := userInterface.(*storage.User)

	// OAuth users can't change email
	if user.AuthProvider != "email" {
		c.Redirect(http.StatusSeeOther, "/dashboard?error=oauth_email")
		return
	}

	c.HTML(http.StatusOK, "change-email.html", GetTemplateData(c, gin.H{
		"CurrentPage": "change-email",
		"PageTitle":   "Change Email",
		"user":        user,
		"error":       c.Query("error"),
		"success":     c.Query("success"),
	}))
}

// HandleChangeEmail processes the email change request
func (h *AccountHandler) HandleChangeEmail(c *gin.Context) {
	userInterface, exists := c.Get("user")
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	user := userInterface.(*storage.User)

	// OAuth users can't change email
	if user.AuthProvider != "email" {
		c.Redirect(http.StatusSeeOther, "/dashboard?error=oauth_email")
		return
	}

	// Get form data
	currentPassword := c.PostForm("current_password")
	newEmail := strings.ToLower(c.PostForm("new_email"))
	confirmEmail := c.PostForm("confirm_email")

	// Validate inputs
	if currentPassword == "" || newEmail == "" || confirmEmail == "" {
		c.Redirect(http.StatusSeeOther, "/account/change-email?error=missing_fields")
		return
	}

	// Check if emails match
	if newEmail != confirmEmail {
		c.Redirect(http.StatusSeeOther, "/account/change-email?error=email_mismatch")
		return
	}

	// Check if new email is same as current
	if newEmail == user.Email {
		c.Redirect(http.StatusSeeOther, "/account/change-email?error=same_email")
		return
	}

	// Validate email format
	if !isValidEmail(newEmail) {
		c.Redirect(http.StatusSeeOther, "/account/change-email?error=invalid_email")
		return
	}

	// Verify current password
	if user.PasswordHash == nil {
		c.Redirect(http.StatusSeeOther, "/account/change-email?error=invalid_password")
		return
	}

	err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(currentPassword))
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/account/change-email?error=invalid_password")
		return
	}

	// Check if new email already exists
	existingUser, _ := h.metadata.GetUserByEmail(c.Request.Context(), newEmail)
	if existingUser != nil {
		c.Redirect(http.StatusSeeOther, "/account/change-email?error=email_exists")
		return
	}

	// Update email
	oldEmail := user.Email
	user.Email = newEmail
	user.UpdatedAt = time.Now()

	if err := h.metadata.UpdateUser(c.Request.Context(), user); err != nil {
		log.Printf("Failed to update email for user %s: %v", user.ID, err)
		c.Redirect(http.StatusSeeOther, "/account/change-email?error=update_failed")
		return
	}

	// Send confirmation emails
	if h.emailService != nil {
		// Send to old email
		go func() {
			if err := h.emailService.SendEmailChanged(context.Background(), oldEmail, newEmail); err != nil {
				log.Printf("Failed to send email change notification to old email: %v", err)
			}
		}()

		// Send to new email
		go func() {
			if err := h.emailService.SendEmailChangeConfirmation(context.Background(), newEmail); err != nil {
				log.Printf("Failed to send email change confirmation to new email: %v", err)
			}
		}()
	}

	c.Redirect(http.StatusSeeOther, "/account/change-email?success=true")
}

// ShowChangePassword displays the change password form
func (h *AccountHandler) ShowChangePassword(c *gin.Context) {
	userInterface, exists := c.Get("user")
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	user := userInterface.(*storage.User)

	// OAuth users can't change password
	if user.AuthProvider != "email" {
		c.Redirect(http.StatusSeeOther, "/dashboard?error=oauth_password")
		return
	}

	c.HTML(http.StatusOK, "change-password.html", GetTemplateData(c, gin.H{
		"CurrentPage": "change-password",
		"PageTitle":   "Change Password",
		"user":        user,
		"error":       c.Query("error"),
		"success":     c.Query("success"),
	}))
}

// HandleChangePassword processes the password change request
func (h *AccountHandler) HandleChangePassword(c *gin.Context) {
	userInterface, exists := c.Get("user")
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	user := userInterface.(*storage.User)

	// OAuth users can't change password
	if user.AuthProvider != "email" {
		c.Redirect(http.StatusSeeOther, "/dashboard?error=oauth_password")
		return
	}

	// Get form data
	currentPassword := c.PostForm("current_password")
	newPassword := c.PostForm("new_password")
	confirmPassword := c.PostForm("confirm_password")

	// Validate inputs
	if currentPassword == "" || newPassword == "" || confirmPassword == "" {
		c.Redirect(http.StatusSeeOther, "/account/change-password?error=missing_fields")
		return
	}

	// Check if passwords match
	if newPassword != confirmPassword {
		c.Redirect(http.StatusSeeOther, "/account/change-password?error=password_mismatch")
		return
	}

	// Check password length
	if len(newPassword) < 8 {
		c.Redirect(http.StatusSeeOther, "/account/change-password?error=password_short")
		return
	}

	// Verify current password
	if user.PasswordHash == nil {
		c.Redirect(http.StatusSeeOther, "/account/change-password?error=invalid_password")
		return
	}

	err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(currentPassword))
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/account/change-password?error=invalid_password")
		return
	}

	// Check if new password is same as current
	err = bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(newPassword))
	if err == nil {
		c.Redirect(http.StatusSeeOther, "/account/change-password?error=same_password")
		return
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Failed to hash password for user %s: %v", user.ID, err)
		c.Redirect(http.StatusSeeOther, "/account/change-password?error=server_error")
		return
	}

	// Update password
	hashedStr := string(hashedPassword)
	user.PasswordHash = &hashedStr
	user.UpdatedAt = time.Now()

	if err := h.metadata.UpdateUser(c.Request.Context(), user); err != nil {
		log.Printf("Failed to update password for user %s: %v", user.ID, err)
		c.Redirect(http.StatusSeeOther, "/account/change-password?error=update_failed")
		return
	}

	// Send confirmation email
	if h.emailService != nil {
		go func() {
			if err := h.emailService.SendPasswordChanged(context.Background(), user.Email); err != nil {
				log.Printf("Failed to send password change confirmation: %v", err)
			}
		}()
	}

	c.Redirect(http.StatusSeeOther, "/account/change-password?success=true")
}

// Helper function to validate email format
func isValidEmail(email string) bool {
	// Basic email validation
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}

	if len(parts[0]) == 0 || len(parts[1]) == 0 {
		return false
	}

	// Check if domain has at least one dot
	if !strings.Contains(parts[1], ".") {
		return false
	}

	return true
}
