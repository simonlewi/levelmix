// pkg/email/service.go
package email

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/resendlabs/resend-go"
)

// EmailService defines the interface for sending emails
type EmailService interface {
	SendPasswordReset(ctx context.Context, to, token string) error
	SendWelcome(ctx context.Context, to string) error
	SendAccountDeleted(ctx context.Context, to string) error
	SendEmailChanged(ctx context.Context, oldEmail, newEmail string) error
	SendEmailChangeConfirmation(ctx context.Context, to string) error
	SendPasswordChanged(ctx context.Context, to string) error
	SendPaymentSuccess(ctx context.Context, to, planName, amount string) error
	SendPaymentFailed(ctx context.Context, to string) error
	SendSubscriptionCanceled(ctx context.Context, to, planName string) error
	SendSubscriptionReactivated(ctx context.Context, to, planName string) error
}

// ResendService implements EmailService using Resend
type ResendService struct {
	client    *resend.Client
	fromEmail string
	fromName  string
	baseURL   string
}

// NewResendService creates a new Resend email service
func NewResendService() (EmailService, error) {
	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("RESEND_API_KEY environment variable not set")
	}

	// DEBUG: Check if key looks correct
	log.Printf("API Key loaded - starts with: %s, length: %d",
		apiKey[:4], len(apiKey))

	if !strings.HasPrefix(apiKey, "re_") {
		return nil, fmt.Errorf("API key should start with 're_'")
	}

	fromEmail := os.Getenv("EMAIL_FROM")
	if fromEmail == "" {
		fromEmail = "onboarding@resend.dev" // Default from email
	}

	fromName := os.Getenv("EMAIL_FROM_NAME")
	if fromName == "" {
		fromName = "LevelMix"
	}

	baseURL := os.Getenv("APP_URL")
	if baseURL == "" {
		baseURL = "https://levelmix.io" // Default base URL
	}

	client := resend.NewClient(apiKey)

	return &ResendService{
		client:    client,
		fromEmail: fromEmail,
		fromName:  fromName,
		baseURL:   baseURL,
	}, nil
}

// SendPasswordReset sends a password reset email
func (s *ResendService) SendPasswordReset(ctx context.Context, to, token string) error {
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", s.baseURL, token)

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Reset Your Password</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #06b6d4 0%%, #3b82f6 100%%); color: white; padding: 30px; text-align: center; border-radius: 8px 8px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border-radius: 0 0 8px 8px; }
        .button { display: inline-block; padding: 14px 30px; background: #06b6d4; color: white; text-decoration: none; border-radius: 5px; margin: 20px 0; }
        .footer { text-align: center; margin-top: 30px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Password Reset Request</h1>
        </div>
        <div class="content">
            <p>Hi there,</p>
            <p>We received a request to reset your password for your LevelMix account. Click the button below to create a new password:</p>
            <div style="text-align: center;">
                <a href="%s" class="button">Reset Password</a>
            </div>
            <p>Or copy and paste this link into your browser:</p>
            <p style="word-break: break-all; color: #06b6d4;">%s</p>
            <p><strong>This link will expire in 1 hour for security reasons.</strong></p>
            <p>If you didn't request a password reset, you can safely ignore this email. Your password won't be changed.</p>
        </div>
        <div class="footer">
            <p>© 2025 LevelMix. All rights reserved.</p>
        </div>
    </div>
</body>
</html>`, resetLink, resetLink)

	text := fmt.Sprintf(`Password Reset Request

Hi there,

We received a request to reset your password for your LevelMix account.

To reset your password, visit this link:
%s

This link will expire in 1 hour for security reasons.

If you didn't request a password reset, you can safely ignore this email. Your password won't be changed.

© 2025 LevelMix. All rights reserved.`, resetLink)

	request := &resend.SendEmailRequest{
		From:    fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail),
		To:      []string{to},
		Subject: "Reset Your LevelMix Password",
		Html:    html,
		Text:    text,
	}

	_, err := s.client.Emails.Send(request)
	if err != nil {
		log.Printf("Failed to send password reset email to %s: %v", to, err)
		return fmt.Errorf("failed to send email: %w", err)
	}

	log.Printf("Password reset email sent successfully to %s", to)
	return nil
}

// SendWelcome sends a welcome email to new users
func (s *ResendService) SendWelcome(ctx context.Context, to string) error {
	dashboardLink := fmt.Sprintf("%s/dashboard", s.baseURL)

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Welcome to LevelMix!</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #06b6d4 0%%, #3b82f6 100%%); color: white; padding: 30px; text-align: center; border-radius: 8px 8px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border-radius: 0 0 8px 8px; }
        .button { display: inline-block; padding: 14px 30px; background: #06b6d4; color: white; text-decoration: none; border-radius: 5px; margin: 20px 0; }
        .feature { padding: 15px; background: white; border-radius: 5px; margin: 10px 0; }
        .footer { text-align: center; margin-top: 30px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Welcome to LevelMix! 🎵</h1>
        </div>
        <div class="content">
            <p>Hi there!</p>
            <p>Welcome to LevelMix - your professional audio normalization solution. We're excited to have you on board!</p>
            
            <h3>What you can do with LevelMix:</h3>
            <div class="feature">
                <strong>🎚️ Normalize Audio Files</strong><br>
                Achieve consistent loudness levels for all your audio content
            </div>
            <div class="feature">
                <strong>🎯 Multiple LUFS Targets</strong><br>
                Choose from streaming, podcast, radio, or club-ready presets
            </div>
            <div class="feature">
                <strong>⚡ Fast Processing</strong><br>
                Get your normalized audio in minutes
            </div>
            
            <div style="text-align: center;">
                <a href="%s" class="button">Go to Dashboard</a>
            </div>
            
            <p>If you have any questions, feel free to reach out to our support team.</p>
            <p>Happy mixing!</p>
            <p>- The LevelMix Team</p>
        </div>
        <div class="footer">
            <p>© 2025 LevelMix. All rights reserved.</p>
        </div>
    </div>
</body>
</html>`, dashboardLink)

	text := fmt.Sprintf(`Welcome to LevelMix! 🎵

Hi there!

Welcome to LevelMix - your professional audio normalization solution. We're excited to have you on board!

What you can do with LevelMix:

🎚️ Normalize Audio Files
Achieve consistent loudness levels for all your audio content

🎯 Multiple LUFS Targets
Choose from streaming, podcast, radio, or club-ready presets

⚡ Fast Processing
Get your normalized audio in minutes

Go to Dashboard: %s

If you have any questions, feel free to reach out to our support team.

Happy mixing!
- The LevelMix Team

© 2025 LevelMix. All rights reserved.`, dashboardLink)

	request := &resend.SendEmailRequest{
		From:    fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail),
		To:      []string{to},
		Subject: "Welcome to LevelMix! 🎵",
		Html:    html,
		Text:    text,
	}

	_, err := s.client.Emails.Send(request)
	if err != nil {
		log.Printf("Failed to send welcome email to %s: %v", to, err)
		return fmt.Errorf("failed to send email: %w", err)
	}

	log.Printf("Welcome email sent successfully to %s", to)
	return nil
}

// SendAccountDeleted sends a confirmation email when account is deleted
func (s *ResendService) SendAccountDeleted(ctx context.Context, to string) error {
	html := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Account Deleted</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #ef4444; color: white; padding: 30px; text-align: center; border-radius: 8px 8px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border-radius: 0 0 8px 8px; }
        .footer { text-align: center; margin-top: 30px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Account Deleted</h1>
        </div>
        <div class="content">
            <p>Hi there,</p>
            <p>This email confirms that your LevelMix account has been successfully deleted.</p>
            <p><strong>What's been removed:</strong></p>
            <ul>
                <li>Your account and profile information</li>
                <li>All uploaded and processed audio files</li>
                <li>Processing history and statistics</li>
                <li>Any active subscriptions</li>
            </ul>
            <p>We're sorry to see you go. If you ever want to come back, you're always welcome to create a new account.</p>
            <p>If you didn't request this deletion, please contact our support team immediately.</p>
            <p>Thank you for using LevelMix.</p>
            <p>- The LevelMix Team</p>
        </div>
        <div class="footer">
            <p>© 2025 LevelMix. All rights reserved.</p>
        </div>
    </div>
</body>
</html>`

	text := `Account Deleted

Hi there,

This email confirms that your LevelMix account has been successfully deleted.

What's been removed:
- Your account and profile information
- All uploaded and processed audio files
- Processing history and statistics
- Any active subscriptions

We're sorry to see you go. If you ever want to come back, you're always welcome to create a new account.

If you didn't request this deletion, please contact our support team immediately.

Thank you for using LevelMix.
- The LevelMix Team

© 2025 LevelMix. All rights reserved.`

	request := &resend.SendEmailRequest{
		From:    fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail),
		To:      []string{to},
		Subject: "Your LevelMix Account Has Been Deleted",
		Html:    html,
		Text:    text,
	}

	_, err := s.client.Emails.Send(request)
	if err != nil {
		log.Printf("Failed to send account deletion email to %s: %v", to, err)
		// Don't return error as account is already deleted
		return nil
	}

	log.Printf("Account deletion email sent successfully to %s", to)
	return nil
}

// SendEmailChanged notifies the old email address that the email has been changed
func (s *ResendService) SendEmailChanged(ctx context.Context, oldEmail, newEmail string) error {
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Email Address Changed</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #ef4444; color: white; padding: 30px; text-align: center; border-radius: 8px 8px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border-radius: 0 0 8px 8px; }
        .warning { background: #fef3c7; border: 1px solid #f59e0b; padding: 15px; border-radius: 5px; margin: 20px 0; }
        .button { display: inline-block; padding: 14px 30px; background: #ef4444; color: white; text-decoration: none; border-radius: 5px; margin: 20px 0; }
        .footer { text-align: center; margin-top: 30px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Email Address Changed</h1>
        </div>
        <div class="content">
            <p>Hi there,</p>
            <p>This email confirms that your LevelMix account email address has been changed.</p>
            
            <div class="warning">
                <strong>⚠️ Important:</strong> Your email has been changed from <strong>%s</strong> to <strong>%s</strong>
            </div>
            
            <p>If you made this change, no further action is needed. You'll need to use your new email address to sign in going forward.</p>
            
            <p><strong>If you didn't make this change:</strong></p>
            <ol>
                <li>Your account may have been compromised</li>
                <li>Contact our support team immediately</li>
                <li>Try to reset your password using the new email address</li>
            </ol>
            
            <p>For security reasons, this notification has been sent to your previous email address.</p>
        </div>
        <div class="footer">
            <p>© 2025 LevelMix. All rights reserved.</p>
        </div>
    </div>
</body>
</html>`, oldEmail, newEmail)

	text := fmt.Sprintf(`Email Address Changed

Hi there,

This email confirms that your LevelMix account email address has been changed.

⚠️ Important: Your email has been changed from %s to %s

If you made this change, no further action is needed. You'll need to use your new email address to sign in going forward.

If you didn't make this change:
1. Your account may have been compromised
2. Contact our support team immediately
3. Try to reset your password using the new email address

For security reasons, this notification has been sent to your previous email address.

© 2025 LevelMix. All rights reserved.`, oldEmail, newEmail)

	request := &resend.SendEmailRequest{
		From:    fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail),
		To:      []string{oldEmail},
		Subject: "Your LevelMix Email Address Has Been Changed",
		Html:    html,
		Text:    text,
	}

	_, err := s.client.Emails.Send(request)
	if err != nil {
		log.Printf("Failed to send email change notification to %s: %v", oldEmail, err)
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// SendEmailChangeConfirmation sends a confirmation to the new email address
func (s *ResendService) SendEmailChangeConfirmation(ctx context.Context, to string) error {
	dashboardLink := fmt.Sprintf("%s/dashboard", s.baseURL)

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Email Change Confirmed</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #06b6d4 0%%, #3b82f6 100%%); color: white; padding: 30px; text-align: center; border-radius: 8px 8px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border-radius: 0 0 8px 8px; }
        .button { display: inline-block; padding: 14px 30px; background: #06b6d4; color: white; text-decoration: none; border-radius: 5px; margin: 20px 0; }
        .footer { text-align: center; margin-top: 30px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Email Successfully Updated!</h1>
        </div>
        <div class="content">
            <p>Hi there,</p>
            <p>This email confirms that your LevelMix account email address has been successfully updated to this address.</p>
            
            <p>You can now use this email address to:</p>
            <ul>
                <li>Sign in to your account</li>
                <li>Receive notifications and updates</li>
                <li>Reset your password if needed</li>
            </ul>
            
            <div style="text-align: center;">
                <a href="%s" class="button">Go to Dashboard</a>
            </div>
            
            <p>If you didn't make this change, please contact our support team immediately.</p>
        </div>
        <div class="footer">
            <p>© 2025 LevelMix. All rights reserved.</p>
        </div>
    </div>
</body>
</html>`, dashboardLink)

	text := fmt.Sprintf(`Email Successfully Updated!

Hi there,

This email confirms that your LevelMix account email address has been successfully updated to this address.

You can now use this email address to:
- Sign in to your account
- Receive notifications and updates
- Reset your password if needed

Go to Dashboard: %s

If you didn't make this change, please contact our support team immediately.

© 2025 LevelMix. All rights reserved.`, dashboardLink)

	request := &resend.SendEmailRequest{
		From:    fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail),
		To:      []string{to},
		Subject: "Welcome to Your Updated LevelMix Email",
		Html:    html,
		Text:    text,
	}

	_, err := s.client.Emails.Send(request)
	if err != nil {
		log.Printf("Failed to send email change confirmation to %s: %v", to, err)
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// SendPasswordChanged sends a confirmation when password is changed
func (s *ResendService) SendPasswordChanged(ctx context.Context, to string) error {
	html := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Password Changed</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #06b6d4 0%, #3b82f6 100%); color: white; padding: 30px; text-align: center; border-radius: 8px 8px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border-radius: 0 0 8px 8px; }
        .success { background: #d1fae5; border: 1px solid #10b981; padding: 15px; border-radius: 5px; margin: 20px 0; }
        .footer { text-align: center; margin-top: 30px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Password Successfully Changed</h1>
        </div>
        <div class="content">
            <p>Hi there,</p>
            
            <div class="success">
                <strong>✓ Success:</strong> Your password has been changed successfully.
            </div>
            
            <p>This email confirms that your LevelMix account password was changed. You can now sign in with your new password.</p>
            
            <p><strong>Security Tips:</strong></p>
            <ul>
                <li>Never share your password with anyone</li>
                <li>Use a unique password for each online account</li>
                <li>Consider using a password manager</li>
                <li>Enable two-factor authentication when available</li>
            </ul>
            
            <p>If you didn't make this change, please:</p>
            <ol>
                <li>Reset your password immediately</li>
                <li>Check your account for any unauthorized activity</li>
                <li>Contact our support team</li>
            </ol>
        </div>
        <div class="footer">
            <p>© 2025 LevelMix. All rights reserved.</p>
        </div>
    </div>
</body>
</html>`

	text := `Password Successfully Changed

Hi there,

✓ Success: Your password has been changed successfully.

This email confirms that your LevelMix account password was changed. You can now sign in with your new password.

Security Tips:
- Never share your password with anyone
- Use a unique password for each online account
- Consider using a password manager
- Enable two-factor authentication when available

If you didn't make this change, please:
1. Reset your password immediately
2. Check your account for any unauthorized activity
3. Contact our support team

© 2025 LevelMix. All rights reserved.`

	request := &resend.SendEmailRequest{
		From:    fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail),
		To:      []string{to},
		Subject: "Your LevelMix Password Has Been Changed",
		Html:    html,
		Text:    text,
	}

	_, err := s.client.Emails.Send(request)
	if err != nil {
		log.Printf("Failed to send password change confirmation to %s: %v", to, err)
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

type MockEmailService struct{}

// SendPasswordReset implements EmailService.
func (m *MockEmailService) SendPasswordReset(ctx context.Context, to string, token string) error {
	panic("unimplemented")
}

// SendWelcome implements EmailService.
func (m *MockEmailService) SendWelcome(ctx context.Context, to string) error {
	panic("unimplemented")
}

// NewMockEmailService creates a mock email service that logs instead of sending
func NewMockEmailService() EmailService {
	log.Println("Initialized Mock Email Service")
	return &MockEmailService{}
}

// Add these to MockEmailService as well:

func (m *MockEmailService) SendEmailChanged(ctx context.Context, oldEmail, newEmail string) error {
	log.Printf("MOCK EMAIL - Email change notification for %s -> %s", oldEmail, newEmail)
	return nil
}

func (m *MockEmailService) SendEmailChangeConfirmation(ctx context.Context, to string) error {
	log.Printf("MOCK EMAIL - Email change confirmation for %s", to)
	return nil
}

func (m *MockEmailService) SendPasswordChanged(ctx context.Context, to string) error {
	log.Printf("MOCK EMAIL - Password change confirmation for %s", to)
	return nil
}

// SendAccountDeleted mocks sending an account deletion email
func (m *MockEmailService) SendAccountDeleted(ctx context.Context, to string) error {
	log.Printf("MOCK EMAIL - Account deletion confirmation for %s", to)
	return nil
}
