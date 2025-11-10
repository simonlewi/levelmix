console.log('Dashboard scripts loaded');

// Open Stripe Billing Portal for subscription management
async function openBillingPortal() {
    try {
        const button = event.target;
        const originalText = button.textContent;
        button.disabled = true;
        button.textContent = 'Loading...';

        const response = await fetch('/api/v1/payment/portal', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            }
        });

        if (!response.ok) {
            throw new Error('Failed to open billing portal');
        }

        const data = await response.json();

        // Redirect to Stripe billing portal
        if (data.url) {
            window.location.href = data.url;
        } else {
            throw new Error('No portal URL returned');
        }
    } catch (error) {
        console.error('Error opening billing portal:', error);
        alert('Failed to open billing portal. Please try again or contact support.');

        // Restore button state
        const button = event.target;
        button.disabled = false;
        button.textContent = 'Manage Subscription';
    }
}
