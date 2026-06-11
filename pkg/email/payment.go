// pkg/email/payment.go
package email

import (
	"context"
	"fmt"
	"time"

	"github.com/resendlabs/resend-go"
)

// SendPaymentSuccess sends a confirmation email when a payment succeeds
func (s *ResendService) SendPaymentSuccess(ctx context.Context, to, planName, amount string) error {
	dashboardLink := fmt.Sprintf("%s/dashboard", s.baseURL)

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>Payment confirmed</title>
  <style>%s</style>
</head>
<body>
  <div class="container">
    <div class="header">
      <p class="wordmark">LevelMix</p>
      <h1 class="title">Payment confirmed</h1>
    </div>
    <div class="content">
      <div class="block block-success">
        <strong>Your %s subscription is now active.</strong>
      </div>
      <div class="block">
        <strong>Amount charged:</strong> %s
      </div>
      <p>You have full access to all features included in your plan. We'll send a separate invoice with the full payment details.</p>
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
</html>`, emailCSS, planName, amount, dashboardLink)

	text := fmt.Sprintf(`Payment confirmed

Your LevelMix %s subscription is now active.

Amount charged: %s

You have full access to all features included in your plan. We'll send a separate invoice with the full payment details.

Go to dashboard: %s

Questions? Reply to this email anytime.

© 2026 LevelMix Audio. All rights reserved.`, planName, amount, dashboardLink)

	request := &resend.SendEmailRequest{
		From:    fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail),
		To:      []string{to},
		Subject: "Payment confirmed — your LevelMix subscription is active",
		Html:    html,
		Text:    text,
	}

	_, err := s.client.Emails.Send(request)
	return err
}

// SendPaymentFailed sends a notification when a payment fails
func (s *ResendService) SendPaymentFailed(ctx context.Context, to string) error {
	portalLink := fmt.Sprintf("%s/dashboard#billing", s.baseURL)

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>Payment failed</title>
  <style>%s</style>
</head>
<body>
  <div class="container">
    <div class="header">
      <p class="wordmark">LevelMix</p>
      <h1 class="title">Payment failed</h1>
    </div>
    <div class="content">
      <div class="block block-error">
        <strong>We were unable to process your payment.</strong> Your subscription is currently past due.
      </div>
      <p>Common reasons:</p>
      <ul>
        <li>Insufficient funds</li>
        <li>Expired card</li>
        <li>Card declined by your bank</li>
        <li>Incorrect billing information</li>
      </ul>
      <p>Update your payment method to keep your subscription active. We'll retry automatically over the next few days.</p>
      <div class="cta">
        <a href="%s" class="button">Update payment method</a>
      </div>
      <p>Need help? Reply to this email.</p>
    </div>
    <div class="footer">
      <p>© 2026 LevelMix Audio. All rights reserved.</p>
    </div>
  </div>
</body>
</html>`, emailCSS, portalLink)

	text := fmt.Sprintf(`Payment failed

We were unable to process your payment for your LevelMix subscription.

Common reasons:
- Insufficient funds
- Expired card
- Card declined by your bank
- Incorrect billing information

Update your payment method to keep your subscription active: %s

We'll retry automatically over the next few days.

Need help? Reply to this email.

© 2026 LevelMix Audio. All rights reserved.`, portalLink)

	request := &resend.SendEmailRequest{
		From:    fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail),
		To:      []string{to},
		Subject: "Action required: payment failed for your LevelMix subscription",
		Html:    html,
		Text:    text,
	}

	_, err := s.client.Emails.Send(request)
	return err
}

// SendSubscriptionCanceled sends a notification when a subscription is canceled
func (s *ResendService) SendSubscriptionCanceled(ctx context.Context, to, planName string) error {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>Subscription canceled</title>
  <style>%s</style>
</head>
<body>
  <div class="container">
    <div class="header">
      <p class="wordmark">LevelMix</p>
      <h1 class="title">Subscription canceled</h1>
    </div>
    <div class="content">
      <p>Hi there,</p>
      <p>Your LevelMix <strong>%s</strong> subscription has been canceled.</p>
      <p>What happens next:</p>
      <ul>
        <li>No further charges will be made.</li>
        <li>Your current benefits remain active until the end of the billing period.</li>
        <li>After that, your account moves to the free plan.</li>
        <li>All your data is preserved.</li>
      </ul>
      <p>You can reactivate anytime from your dashboard.</p>
    </div>
    <div class="footer">
      <p>© 2026 LevelMix Audio. All rights reserved.</p>
    </div>
  </div>
</body>
</html>`, emailCSS, planName)

	text := fmt.Sprintf(`Subscription canceled

Hi there,

Your LevelMix %s subscription has been canceled.

What happens next:
- No further charges will be made.
- Your current benefits remain active until the end of the billing period.
- After that, your account moves to the free plan.
- All your data is preserved.

You can reactivate anytime from your dashboard.

© 2026 LevelMix Audio. All rights reserved.`, planName)

	request := &resend.SendEmailRequest{
		From:    fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail),
		To:      []string{to},
		Subject: "Your LevelMix subscription has been canceled",
		Html:    html,
		Text:    text,
	}

	_, err := s.client.Emails.Send(request)
	return err
}

// SendSubscriptionReactivated sends a notification when a subscription is reactivated
func (s *ResendService) SendSubscriptionReactivated(ctx context.Context, to, planName string) error {
	dashboardLink := fmt.Sprintf("%s/dashboard", s.baseURL)

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>Subscription reactivated</title>
  <style>%s</style>
</head>
<body>
  <div class="container">
    <div class="header">
      <p class="wordmark">LevelMix</p>
      <h1 class="title">You're back.</h1>
    </div>
    <div class="content">
      <div class="block block-success">
        <strong>Your %s subscription is active again.</strong>
      </div>
      <p>You have full access to all features in your plan. Your subscription will continue automatically.</p>
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
</html>`, emailCSS, planName, dashboardLink)

	text := fmt.Sprintf(`You're back.

Your LevelMix %s subscription is active again.

You have full access to all features in your plan. Your subscription will continue automatically.

Go to dashboard: %s

Questions? Reply to this email anytime.

© 2026 LevelMix Audio. All rights reserved.`, planName, dashboardLink)

	request := &resend.SendEmailRequest{
		From:    fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail),
		To:      []string{to},
		Subject: "Your LevelMix subscription is active again",
		Html:    html,
		Text:    text,
	}

	_, err := s.client.Emails.Send(request)
	return err
}

// SendTrialEnding sends a reminder email 48 hours before a trial ends.
func (s *ResendService) SendTrialEnding(ctx context.Context, to, planName string, trialEndDate time.Time) error {
	pricingLink := fmt.Sprintf("%s/pricing", s.baseURL)

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>Your trial ends soon</title>
  <style>%s</style>
</head>
<body>
  <div class="container">
    <div class="header">
      <p class="wordmark">LevelMix</p>
      <h1 class="title">Your trial ends in 2 days</h1>
    </div>
    <div class="content">
      <div class="block block-warning">
        Your LevelMix <strong>%s</strong> trial ends on <strong>%s</strong>.
      </div>
      <p>Subscribe before your trial expires to keep full access.</p>
      <div class="cta">
        <a href="%s" class="button">Choose a plan</a>
      </div>
      <p>Questions? Reply to this email anytime.</p>
    </div>
    <div class="footer">
      <p>© 2026 LevelMix Audio. All rights reserved.</p>
    </div>
  </div>
</body>
</html>`, emailCSS, planName, trialEndDate.Format("January 2, 2006"), pricingLink)

	text := fmt.Sprintf(`Your trial ends in 2 days.

Your LevelMix %s trial ends on %s.

Subscribe before your trial expires to keep full access: %s

Questions? Reply to this email anytime.

© 2026 LevelMix Audio. All rights reserved.`, planName, trialEndDate.Format("January 2, 2006"), pricingLink)

	request := &resend.SendEmailRequest{
		From:    fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail),
		To:      []string{to},
		Subject: "Your LevelMix trial ends in 2 days",
		Html:    html,
		Text:    text,
	}

	_, err := s.client.Emails.Send(request)
	return err
}

// SendTrialEnding sends a reminder email 2 days before the trial expires.
func (s *ResendService) SendTrialEnding(ctx context.Context, to, planName string, trialEndDate time.Time) error {
	dashboardLink := fmt.Sprintf("%s/dashboard", s.baseURL)
	billingLink := fmt.Sprintf("%s/dashboard#billing", s.baseURL)
	formattedDate := trialEndDate.Format("January 2, 2006")

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Your Trial Ends Soon</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; background: #f4f4f4; margin: 0; padding: 0; }
        .container { max-width: 600px; margin: 40px auto; background: white; border-radius: 8px; overflow: hidden; box-shadow: 0 2px 8px rgba(0,0,0,0.08); }
        .header { background: linear-gradient(135deg, #4A8AC7 0%%, #D4A95E 100%%); color: white; padding: 32px 30px; text-align: center; }
        .header h1 { margin: 0; font-size: 22px; font-weight: 600; }
        .header p { margin: 8px 0 0; opacity: 0.9; font-size: 14px; }
        .content { padding: 30px; }
        .notice { background: #FFF8EC; border-left: 4px solid #D4A95E; padding: 16px 20px; border-radius: 0 6px 6px 0; margin: 20px 0; }
        .notice strong { color: #9A6E00; display: block; margin-bottom: 4px; }
        .btn-primary { display: inline-block; padding: 13px 30px; background: #D4A95E; color: white !important; text-decoration: none; border-radius: 6px; font-weight: 600; font-size: 15px; }
        .btn-secondary { display: block; text-align: center; color: #4A8AC7 !important; text-decoration: none; margin-top: 14px; font-size: 13px; }
        .footer { text-align: center; padding: 20px 30px; font-size: 12px; color: #999; background: #f9f9f9; border-top: 1px solid #eee; }
        .footer a { color: #999; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Your free trial ends in 2 days</h1>
            <p>LevelMix %s</p>
        </div>
        <div class="content">
            <p>Hi there,</p>
            <p>Just a heads-up — your LevelMix <strong>%s</strong> free trial ends on <strong>%s</strong>.</p>

            <div class="notice">
                <strong>What happens on %s</strong>
                Your saved card will be charged automatically and your subscription continues uninterrupted. No action needed if you want to keep going.
            </div>

            <p>If you'd like to cancel before then, you can do so any time from your dashboard — no questions asked.</p>

            <div style="text-align: center; margin: 28px 0;">
                <a href="%s" class="btn-primary">Go to Dashboard</a>
                <a href="%s" class="btn-secondary">Manage or cancel subscription</a>
            </div>

            <p style="font-size: 13px; color: #888;">Questions? Just reply to this email.</p>
        </div>
        <div class="footer">
            <p>© 2025 LevelMix · Tricode Digital AB</p>
        </div>
    </div>
</body>
</html>`, planName, planName, formattedDate, formattedDate, dashboardLink, billingLink)

	text := fmt.Sprintf(`Your LevelMix free trial ends in 2 days

Hi there,

Your LevelMix %s free trial ends on %s.

What happens on %s:
Your saved card will be charged automatically and your subscription continues. No action needed if you want to keep going.

If you'd like to cancel before then, you can do so any time from your dashboard — no questions asked.

Dashboard: %s
Manage or cancel: %s

Questions? Reply to this email.

© 2025 LevelMix · Tricode Digital AB`,
		planName, formattedDate, formattedDate, dashboardLink, billingLink)

	request := &resend.SendEmailRequest{
		From:    fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail),
		To:      []string{to},
		Subject: fmt.Sprintf("Your LevelMix trial ends on %s", formattedDate),
		Html:    html,
		Text:    text,
	}

	_, err := s.client.Emails.Send(request)
	return err
}

// Mock implementations for testing
func (m *MockEmailService) SendPaymentSuccess(ctx context.Context, to, planName, amount string) error {
	return nil
}

func (m *MockEmailService) SendPaymentFailed(ctx context.Context, to string) error {
	return nil
}

func (m *MockEmailService) SendSubscriptionCanceled(ctx context.Context, to, planName string) error {
	return nil
}

func (m *MockEmailService) SendSubscriptionReactivated(ctx context.Context, to, planName string) error {
	return nil
}

func (m *MockEmailService) SendTrialEnding(ctx context.Context, to, planName string, trialEndDate time.Time) error {
	return nil
}

func (m *MockEmailService) SendTrialEnding(ctx context.Context, to, planName string, trialEndDate time.Time) error {
	return nil
}
