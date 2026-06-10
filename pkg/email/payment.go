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

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Payment Successful</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #06b6d4 0%%, #3b82f6 100%%); color: white; padding: 30px; text-align: center; border-radius: 8px 8px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border-radius: 0 0 8px 8px; }
        .success { background: #d1fae5; border: 1px solid #10b981; padding: 15px; border-radius: 5px; margin: 20px 0; text-align: center; }
        .button { display: inline-block; padding: 14px 30px; background: #06b6d4; color: white; text-decoration: none; border-radius: 5px; margin: 20px 0; }
        .detail { background: white; padding: 15px; border-radius: 5px; margin: 10px 0; }
        .footer { text-align: center; margin-top: 30px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Payment Successful! 🎉</h1>
        </div>
        <div class="content">
            <div class="success">
                <strong>✓ Payment Received</strong><br>
                Thank you for your subscription to LevelMix %s
            </div>
            
            <div class="detail">
                <strong>Plan:</strong> %s<br>
                <strong>Amount:</strong> %s
            </div>
            
            <p>Your subscription is now active and you have full access to all features included in your plan.</p>
            
            <div style="text-align: center;">
                <a href="%s" class="button">Go to Dashboard</a>
            </div>
            
            <p>You'll receive a separate invoice email with the full payment details.</p>
            
            <p>Questions? Contact our support team anytime.</p>
        </div>
        <div class="footer">
            <p>© 2025 LevelMix. All rights reserved.</p>
        </div>
    </div>
</body>
</html>`, planName, planName, amount, dashboardLink)

	text := fmt.Sprintf(`Payment Successful! 🎉

✓ Payment Received
Thank you for your subscription to LevelMix %s

Plan: %s
Amount: %s

Your subscription is now active and you have full access to all features included in your plan.

Go to Dashboard: %s

You'll receive a separate invoice email with the full payment details.

Questions? Contact our support team anytime.

© 2025 LevelMix. All rights reserved.`, planName, planName, amount, dashboardLink)

	request := &resend.SendEmailRequest{
		From:    fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail),
		To:      []string{to},
		Subject: "Payment Successful - Your LevelMix Subscription is Active",
		Html:    html,
		Text:    text,
	}

	_, err := s.client.Emails.Send(request)
	return err
}

// SendPaymentFailed sends a notification when a payment fails
func (s *ResendService) SendPaymentFailed(ctx context.Context, to string) error {
	portalLink := fmt.Sprintf("%s/dashboard#billing", s.baseURL)

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Payment Failed</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #ef4444; color: white; padding: 30px; text-align: center; border-radius: 8px 8px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border-radius: 0 0 8px 8px; }
        .alert { background: #fee2e2; border: 1px solid #ef4444; padding: 15px; border-radius: 5px; margin: 20px 0; }
        .button { display: inline-block; padding: 14px 30px; background: #ef4444; color: white; text-decoration: none; border-radius: 5px; margin: 20px 0; }
        .footer { text-align: center; margin-top: 30px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Payment Failed</h1>
        </div>
        <div class="content">
            <div class="alert">
                <strong>⚠️ Action Required</strong><br>
                We were unable to process your payment for your LevelMix subscription.
            </div>
            
            <p>Your subscription is currently past due. To continue using LevelMix, please update your payment method.</p>
            
            <p><strong>Common reasons for payment failures:</strong></p>
            <ul>
                <li>Insufficient funds</li>
                <li>Expired card</li>
                <li>Card declined by bank</li>
                <li>Incorrect billing information</li>
            </ul>
            
            <div style="text-align: center;">
                <a href="%s" class="button">Update Payment Method</a>
            </div>
            
            <p>We'll automatically retry the payment within the next few days. If the payment continues to fail, your subscription may be suspended.</p>
            
            <p>Need help? Contact our support team.</p>
        </div>
        <div class="footer">
            <p>© 2025 LevelMix. All rights reserved.</p>
        </div>
    </div>
</body>
</html>`, portalLink)

	text := fmt.Sprintf(`Payment Failed

⚠️ Action Required
We were unable to process your payment for your LevelMix subscription.

Your subscription is currently past due. To continue using LevelMix, please update your payment method.

Common reasons for payment failures:
- Insufficient funds
- Expired card
- Card declined by bank
- Incorrect billing information

Update Payment Method: %s

We'll automatically retry the payment within the next few days. If the payment continues to fail, your subscription may be suspended.

Need help? Contact our support team.

© 2025 LevelMix. All rights reserved.`, portalLink)

	request := &resend.SendEmailRequest{
		From:    fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail),
		To:      []string{to},
		Subject: "Action Required: Payment Failed for Your LevelMix Subscription",
		Html:    html,
		Text:    text,
	}

	_, err := s.client.Emails.Send(request)
	return err
}

// SendSubscriptionCanceled sends a notification when a subscription is canceled
func (s *ResendService) SendSubscriptionCanceled(ctx context.Context, to, planName string) error {
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Subscription Canceled</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #64748b; color: white; padding: 30px; text-align: center; border-radius: 8px 8px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border-radius: 0 0 8px 8px; }
        .info { background: #e0e7ff; border: 1px solid #6366f1; padding: 15px; border-radius: 5px; margin: 20px 0; }
        .footer { text-align: center; margin-top: 30px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Subscription Canceled</h1>
        </div>
        <div class="content">
            <p>Hi there,</p>
            
            <div class="info">
                Your LevelMix %s subscription has been canceled as requested.
            </div>
            
            <p>You'll continue to have access to your subscription features until the end of your current billing period. After that, your account will be downgraded to the free plan.</p>
            
            <p><strong>What happens next:</strong></p>
            <ul>
                <li>No further charges will be made</li>
                <li>Your current benefits remain active until the end of the billing period</li>
                <li>You'll be automatically moved to the Free plan</li>
                <li>All your data will be preserved</li>
            </ul>
            
            <p>We're sorry to see you go! If you have any feedback about your experience with LevelMix, we'd love to hear it.</p>
            
            <p>You can reactivate your subscription anytime from your dashboard.</p>
        </div>
        <div class="footer">
            <p>© 2025 LevelMix. All rights reserved.</p>
        </div>
    </div>
</body>
</html>`, planName)

	text := fmt.Sprintf(`Subscription Canceled

Hi there,

Your LevelMix %s subscription has been canceled as requested.

You'll continue to have access to your subscription features until the end of your current billing period. After that, your account will be downgraded to the free plan.

What happens next:
- No further charges will be made
- Your current benefits remain active until the end of the billing period
- You'll be automatically moved to the Free plan
- All your data will be preserved

We're sorry to see you go! If you have any feedback about your experience with LevelMix, we'd love to hear it.

You can reactivate your subscription anytime from your dashboard.

© 2025 LevelMix. All rights reserved.`, planName)

	request := &resend.SendEmailRequest{
		From:    fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail),
		To:      []string{to},
		Subject: "Your LevelMix Subscription Has Been Canceled",
		Html:    html,
		Text:    text,
	}

	_, err := s.client.Emails.Send(request)
	return err
}

// SendSubscriptionReactivated sends a notification when a subscription is reactivated
func (s *ResendService) SendSubscriptionReactivated(ctx context.Context, to, planName string) error {
	dashboardLink := fmt.Sprintf("%s/dashboard", s.baseURL)

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Subscription Reactivated</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #06b6d4 0%%, #3b82f6 100%%); color: white; padding: 30px; text-align: center; border-radius: 8px 8px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border-radius: 0 0 8px 8px; }
        .success { background: #d1fae5; border: 1px solid #10b981; padding: 15px; border-radius: 5px; margin: 20px 0; text-align: center; }
        .button { display: inline-block; padding: 14px 30px; background: #06b6d4; color: white; text-decoration: none; border-radius: 5px; margin: 20px 0; }
        .footer { text-align: center; margin-top: 30px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Welcome Back! 🎉</h1>
        </div>
        <div class="content">
            <div class="success">
                <strong>✓ Subscription Reactivated</strong><br>
                Your LevelMix %s subscription is now active again
            </div>
            
            <p>Great news! Your subscription has been reactivated and will continue automatically.</p>
            
            <p>You now have full access to all %s features:</p>
            <ul>
                <li>Extended processing time</li>
                <li>Priority processing</li>
                <li>Multiple format support</li>
                <li>Advanced features</li>
            </ul>
            
            <div style="text-align: center;">
                <a href="%s" class="button">Go to Dashboard</a>
            </div>
            
            <p>Thank you for choosing LevelMix!</p>
        </div>
        <div class="footer">
            <p>© 2025 LevelMix. All rights reserved.</p>
        </div>
    </div>
</body>
</html>`, planName, planName, dashboardLink)

	text := fmt.Sprintf(`Welcome Back! 🎉

✓ Subscription Reactivated
Your LevelMix %s subscription is now active again

Great news! Your subscription has been reactivated and will continue automatically.

You now have full access to all %s features:
- Extended processing time
- Priority processing
- Multiple format support
- Advanced features

Go to Dashboard: %s

Thank you for choosing LevelMix!

© 2025 LevelMix. All rights reserved.`, planName, planName, dashboardLink)

	request := &resend.SendEmailRequest{
		From:    fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail),
		To:      []string{to},
		Subject: "Your LevelMix Subscription is Active Again!",
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
	// log.Printf("MOCK EMAIL - Payment success for %s: %s plan, %s", to, planName, amount)
	return nil
}

func (m *MockEmailService) SendPaymentFailed(ctx context.Context, to string) error {
	// log.Printf("MOCK EMAIL - Payment failed notification for %s", to)
	return nil
}

func (m *MockEmailService) SendSubscriptionCanceled(ctx context.Context, to, planName string) error {
	// log.Printf("MOCK EMAIL - Subscription canceled for %s: %s", to, planName)
	return nil
}

func (m *MockEmailService) SendSubscriptionReactivated(ctx context.Context, to, planName string) error {
	// log.Printf("MOCK EMAIL - Subscription reactivated for %s: %s", to, planName)
	return nil
}

func (m *MockEmailService) SendTrialEnding(ctx context.Context, to, planName string, trialEndDate time.Time) error {
	return nil
}
