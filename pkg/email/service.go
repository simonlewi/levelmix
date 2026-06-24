// pkg/email/service.go
package email

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

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
	SendTrialEnding(ctx context.Context, to, planName string, trialEndDate time.Time) error
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
		fromEmail = "onboarding@resend.dev"
	}

	fromName := os.Getenv("EMAIL_FROM_NAME")
	if fromName == "" {
		fromName = "LevelMix"
	}

	baseURL := os.Getenv("APP_URL")
	if baseURL == "" {
		baseURL = "https://levelmix.io"
	}

	client := resend.NewClient(apiKey)

	return &ResendService{
		client:    client,
		fromEmail: fromEmail,
		fromName:  fromName,
		baseURL:   baseURL,
	}, nil
}

const emailCSS = `
    body { font-family: 'Inter', Arial, sans-serif; line-height: 1.6; color: #EDEAE3; background-color: #0F0F0D; margin: 0; padding: 20px; }
    .container { max-width: 600px; margin: 0 auto; background: #1C1C19; border-radius: 8px; overflow: hidden; border: 1px solid #414750; }
    .header { background: #20201D; padding: 28px 30px; border-bottom: 1px solid #414750; }
    .wordmark { font-size: 11px; font-weight: 700; letter-spacing: 0.1em; color: #4A8AC7; text-transform: uppercase; margin: 0 0 8px 0; }
    .title { font-size: 22px; font-weight: 600; color: #EDEAE3; margin: 0; line-height: 1.3; }
    .content { padding: 30px; color: #B0ADA4; font-size: 15px; }
    .content p { margin: 0 0 16px 0; }
    .content strong, .content b { color: #EDEAE3; }
    .content ul, .content ol { padding-left: 20px; margin: 0 0 16px 0; }
    .content li { margin-bottom: 6px; }
    .block { background: #20201D; border: 1px solid #414750; padding: 16px 20px; border-radius: 6px; margin: 20px 0; color: #B0ADA4; }
    .block-success { border-color: #41c44e; }
    .block-error { border-color: #ef4444; }
    .block-warning { border-color: #D4A95E; }
    .feature { background: #20201D; border: 1px solid #414750; padding: 14px 18px; border-radius: 6px; margin: 8px 0; }
    .feature-label { font-weight: 600; color: #EDEAE3; font-size: 14px; }
    .feature-desc { color: #B0ADA4; font-size: 13px; margin-top: 2px; }
    .cta { text-align: center; margin: 24px 0; }
    .button { display: inline-block; padding: 13px 28px; background: #4A8AC7; color: #FFFFFF; text-decoration: none; border-radius: 6px; font-weight: 600; font-size: 14px; }
    .link { color: #4A8AC7; word-break: break-all; }
    .footer { padding: 20px 30px; border-top: 1px solid #414750; text-align: center; font-size: 12px; color: #7A7770; }
    .footer p { margin: 0; }`

// SendPasswordReset sends a password reset email
func (s *ResendService) SendPasswordReset(ctx context.Context, to, token string) error {
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", s.baseURL, token)

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>Reset your password</title>
  <style>%s</style>
</head>
<body>
  <div class="container">
    <div class="header">
      <p class="wordmark">LevelMix</p>
      <h1 class="title">Reset your password</h1>
    </div>
    <div class="content">
      <p>Hi there,</p>
      <p>We received a request to reset your LevelMix password. Click the button below to choose a new one.</p>
      <div class="cta">
        <a href="%s" class="button">Reset password</a>
      </div>
      <p>Or copy this link into your browser:</p>
      <p><a href="%s" class="link">%s</a></p>
      <p><strong>This link expires in 1 hour.</strong></p>
      <p>If you didn't request this, you can safely ignore this email.</p>
    </div>
    <div class="footer">
      <p>© 2026 LevelMix Audio. All rights reserved.</p>
    </div>
  </div>
</body>
</html>`, emailCSS, resetLink, resetLink, resetLink)

	text := fmt.Sprintf(`Reset your password

Hi there,

We received a request to reset your LevelMix password.

Reset your password: %s

This link expires in 1 hour. If you didn't request this, you can safely ignore this email.

© 2026 LevelMix Audio. All rights reserved.`, resetLink)

	request := &resend.SendEmailRequest{
		From:    fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail),
		To:      []string{to},
		Subject: "Reset your LevelMix password",
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

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>Welcome to LevelMix</title>
  <style>%s</style>
</head>
<body>
  <div class="container">
    <div class="header">
      <p class="wordmark">LevelMix</p>
      <h1 class="title">Welcome aboard.</h1>
    </div>
    <div class="content">
      <p>Hi there,</p>
      <p>Your LevelMix account is ready. Here's what you can do:</p>
      <div class="feature">
        <div class="feature-label">Normalize audio files</div>
        <div class="feature-desc">Achieve consistent loudness across all your tracks.</div>
      </div>
      <div class="feature">
        <div class="feature-label">Choose your LUFS target</div>
        <div class="feature-desc">Presets for streaming, podcast, radio, and club.</div>
      </div>
      <div class="feature">
        <div class="feature-label">Fast processing</div>
        <div class="feature-desc">Processed and ready to download in minutes.</div>
      </div>
      <div class="cta">
        <a href="%s" class="button">Go to dashboard</a>
      </div>
      <p>Questions? Reply to this email anytime.</p>
    </div>
    <div class="footer">
      <p>© 2026 LevelMix Audio. All rights reserved.</p>
    </div>
  </div>
</body>
</html>`, emailCSS, dashboardLink)

	text := fmt.Sprintf(`Welcome to LevelMix.

Hi there,

Your LevelMix account is ready.

Normalize audio files — Achieve consistent loudness across all your tracks.
Choose your LUFS target — Presets for streaming, podcast, radio, and club.
Fast processing — Processed and ready to download in minutes.

Go to dashboard: %s

Questions? Reply to this email anytime.

© 2026 LevelMix Audio. All rights reserved.`, dashboardLink)

	request := &resend.SendEmailRequest{
		From:    fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail),
		To:      []string{to},
		Subject: "Welcome to LevelMix",
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
	html := `<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>Account deleted</title>
  <style>
    body { font-family: 'Inter', Arial, sans-serif; line-height: 1.6; color: #EDEAE3; background-color: #0F0F0D; margin: 0; padding: 20px; }
    .container { max-width: 600px; margin: 0 auto; background: #1C1C19; border-radius: 8px; overflow: hidden; border: 1px solid #414750; }
    .header { background: #20201D; padding: 28px 30px; border-bottom: 1px solid #414750; }
    .wordmark { font-size: 11px; font-weight: 700; letter-spacing: 0.1em; color: #4A8AC7; text-transform: uppercase; margin: 0 0 8px 0; }
    .title { font-size: 22px; font-weight: 600; color: #EDEAE3; margin: 0; }
    .content { padding: 30px; color: #B0ADA4; font-size: 15px; }
    .content p { margin: 0 0 16px 0; }
    .content strong { color: #EDEAE3; }
    .content ul { padding-left: 20px; margin: 0 0 16px 0; }
    .content li { margin-bottom: 6px; }
    .footer { padding: 20px 30px; border-top: 1px solid #414750; text-align: center; font-size: 12px; color: #7A7770; }
    .footer p { margin: 0; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <p class="wordmark">LevelMix</p>
      <h1 class="title">Account deleted</h1>
    </div>
    <div class="content">
      <p>Hi there,</p>
      <p>Your LevelMix account has been deleted. Here's what was removed:</p>
      <ul>
        <li>Your account and profile</li>
        <li>All uploaded and processed audio files</li>
        <li>Processing history and statistics</li>
        <li>Any active subscriptions</li>
      </ul>
      <p>If you change your mind, you're welcome to create a new account at any time.</p>
      <p>If you didn't request this deletion, contact our support team immediately.</p>
    </div>
    <div class="footer">
      <p>© 2026 LevelMix Audio. All rights reserved.</p>
    </div>
  </div>
</body>
</html>`

	text := `Account deleted

Hi there,

Your LevelMix account has been deleted. Here's what was removed:

- Your account and profile
- All uploaded and processed audio files
- Processing history and statistics
- Any active subscriptions

If you change your mind, you're welcome to create a new account at any time.

If you didn't request this deletion, contact our support team immediately.

© 2026 LevelMix Audio. All rights reserved.`

	request := &resend.SendEmailRequest{
		From:    fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail),
		To:      []string{to},
		Subject: "Your LevelMix account has been deleted",
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
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>Email address changed</title>
  <style>%s</style>
</head>
<body>
  <div class="container">
    <div class="header">
      <p class="wordmark">LevelMix</p>
      <h1 class="title">Email address changed</h1>
    </div>
    <div class="content">
      <p>Hi there,</p>
      <div class="block block-warning">
        Your account email was changed from <strong>%s</strong> to <strong>%s</strong>.
      </div>
      <p>If you made this change, no further action is needed. Use your new email to sign in going forward.</p>
      <p><strong>If you didn't make this change:</strong></p>
      <ol>
        <li>Your account may be compromised.</li>
        <li>Contact our support team immediately.</li>
        <li>Try resetting your password using the new email address.</li>
      </ol>
    </div>
    <div class="footer">
      <p>© 2026 LevelMix Audio. All rights reserved.</p>
    </div>
  </div>
</body>
</html>`, emailCSS, oldEmail, newEmail)

	text := fmt.Sprintf(`Email address changed

Hi there,

Your LevelMix account email was changed from %s to %s.

If you made this change, no further action is needed. Use your new email to sign in going forward.

If you didn't make this change:
1. Your account may be compromised.
2. Contact our support team immediately.
3. Try resetting your password using the new email address.

© 2026 LevelMix Audio. All rights reserved.`, oldEmail, newEmail)

	request := &resend.SendEmailRequest{
		From:    fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail),
		To:      []string{oldEmail},
		Subject: "Your LevelMix email address has been changed",
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

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>Email updated</title>
  <style>%s</style>
</head>
<body>
  <div class="container">
    <div class="header">
      <p class="wordmark">LevelMix</p>
      <h1 class="title">Email updated</h1>
    </div>
    <div class="content">
      <p>Hi there,</p>
      <div class="block block-success">
        <strong>Your email address has been updated.</strong> This address is now active on your account.
      </div>
      <p>You can now use this email to:</p>
      <ul>
        <li>Sign in to your account</li>
        <li>Receive notifications</li>
        <li>Reset your password if needed</li>
      </ul>
      <div class="cta">
        <a href="%s" class="button">Go to dashboard</a>
      </div>
      <p>If you didn't make this change, contact our support team immediately.</p>
    </div>
    <div class="footer">
      <p>© 2026 LevelMix Audio. All rights reserved.</p>
    </div>
  </div>
</body>
</html>`, emailCSS, dashboardLink)

	text := fmt.Sprintf(`Email updated

Hi there,

Your LevelMix email address has been updated. This address is now active on your account.

You can now use this email to:
- Sign in to your account
- Receive notifications
- Reset your password if needed

Go to dashboard: %s

If you didn't make this change, contact our support team immediately.

© 2026 LevelMix Audio. All rights reserved.`, dashboardLink)

	request := &resend.SendEmailRequest{
		From:    fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail),
		To:      []string{to},
		Subject: "Your LevelMix email has been updated",
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
	html := `<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>Password changed</title>
  <style>
    body { font-family: 'Inter', Arial, sans-serif; line-height: 1.6; color: #EDEAE3; background-color: #0F0F0D; margin: 0; padding: 20px; }
    .container { max-width: 600px; margin: 0 auto; background: #1C1C19; border-radius: 8px; overflow: hidden; border: 1px solid #414750; }
    .header { background: #20201D; padding: 28px 30px; border-bottom: 1px solid #414750; }
    .wordmark { font-size: 11px; font-weight: 700; letter-spacing: 0.1em; color: #4A8AC7; text-transform: uppercase; margin: 0 0 8px 0; }
    .title { font-size: 22px; font-weight: 600; color: #EDEAE3; margin: 0; }
    .content { padding: 30px; color: #B0ADA4; font-size: 15px; }
    .content p { margin: 0 0 16px 0; }
    .content strong { color: #EDEAE3; }
    .content ol { padding-left: 20px; margin: 0 0 16px 0; }
    .content li { margin-bottom: 6px; }
    .block-success { background: #20201D; border: 1px solid #41c44e; padding: 16px 20px; border-radius: 6px; margin: 20px 0; }
    .footer { padding: 20px 30px; border-top: 1px solid #414750; text-align: center; font-size: 12px; color: #7A7770; }
    .footer p { margin: 0; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <p class="wordmark">LevelMix</p>
      <h1 class="title">Password changed</h1>
    </div>
    <div class="content">
      <p>Hi there,</p>
      <div class="block-success">
        <strong>Your password has been changed successfully.</strong> You can sign in with your new password.
      </div>
      <p>If you didn't make this change:</p>
      <ol>
        <li>Reset your password immediately.</li>
        <li>Check your account for unauthorized activity.</li>
        <li>Contact our support team.</li>
      </ol>
    </div>
    <div class="footer">
      <p>© 2026 LevelMix Audio. All rights reserved.</p>
    </div>
  </div>
</body>
</html>`

	text := `Password changed

Hi there,

Your LevelMix password has been changed successfully. You can sign in with your new password.

If you didn't make this change:
1. Reset your password immediately.
2. Check your account for unauthorized activity.
3. Contact our support team.

© 2026 LevelMix Audio. All rights reserved.`

	request := &resend.SendEmailRequest{
		From:    fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail),
		To:      []string{to},
		Subject: "Your LevelMix password has been changed",
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
	log.Printf("MOCK EMAIL - Password reset for %s (token: %s)", to, token)
	return nil
}

// SendWelcome implements EmailService.
func (m *MockEmailService) SendWelcome(ctx context.Context, to string) error {
	log.Printf("MOCK EMAIL - Welcome email for %s", to)
	return nil
}

// NewMockEmailService creates a mock email service that logs instead of sending
func NewMockEmailService() EmailService {
	log.Println("Initialized Mock Email Service")
	return &MockEmailService{}
}

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
