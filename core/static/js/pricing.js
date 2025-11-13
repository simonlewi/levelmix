console.log('Pricing page scripts loaded - v2');
// HTMX event handlers for navigation
document.body.addEventListener('htmx:beforeRequest', function(evt) {
    console.log('Navigation request started');
});

document.body.addEventListener('htmx:afterRequest', function(evt) {
    console.log('Navigation completed');
});

// Start checkout process
async function startCheckout(event, planId, billingInterval) {
    // Check if user is logged in by checking for user data in page
    const isLoggedIn = document.querySelector('[data-user-logged-in]')?.dataset.userLoggedIn === 'true';

    if (!isLoggedIn) {
        // Redirect to register with plan parameter
        window.location.href = '/register?plan=' + planId + '&interval=' + billingInterval;
        return;
    }

    try {
        // Get the button that was clicked
        const button = event.target;

        // Show loading state
        button.disabled = true;
        const originalText = button.textContent;
        button.textContent = 'Loading...';

        // Call checkout API
        const response = await fetch('/api/v1/payment/checkout', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                plan_id: planId,
                billing_interval: billingInterval
            })
        });

        if (!response.ok) {
            let errorMessage = 'Failed to create checkout session';
            try {
                const error = await response.json();
                errorMessage = error.message || errorMessage;
            } catch (e) {
                // If response is not JSON, use default error message
            }
            throw new Error(errorMessage);
        }

        const data = await response.json();

        // Redirect to Stripe Checkout
        if (data.checkout_url) {
            window.location.href = data.checkout_url;
        } else {
            throw new Error('No checkout URL returned');
        }
    } catch (error) {
        console.error('Checkout error:', error);
        console.error('Plan ID:', planId);
        console.error('Billing interval:', billingInterval);
        alert('Failed to start checkout for ' + planId + ': ' + error.message);
        // Restore button state
        const button = event.target;
        button.disabled = false;
        button.textContent = 'Start ' + planId.charAt(0).toUpperCase() + planId.slice(1);
    }
}
