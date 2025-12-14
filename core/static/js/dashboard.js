console.log('Dashboard scripts loaded');

// Open Stripe Billing Portal for subscription management
async function openBillingPortal(event) {
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

function showNameModal() {
    const modal = document.getElementById('nameModal');
    if (modal) {
        modal.classList.remove('hidden');
        // Focus on the name input
        const nameInput = document.getElementById('name');
        if (nameInput) {
            setTimeout(() => nameInput.focus(), 100);
        }
    }
}

function hideNameModal() {
    const modal = document.getElementById('nameModal');
    if (modal) {
        modal.classList.add('hidden');
    }
}

// Close modal when clicking outside
document.addEventListener('DOMContentLoaded', function() {
    const modal = document.getElementById('nameModal');
    if (modal) {
        modal.addEventListener('click', function(e) {
            if (e.target === modal) {
                hideNameModal();
            }
        });
    }

    // Auto-show modal if there's a name error or success in the URL
    const urlParams = new URLSearchParams(window.location.search);
    const nameError = urlParams.get('nameError');
    const nameSuccess = urlParams.get('nameSuccess');

    if (nameError || nameSuccess) {
        showNameModal();

        // Auto-hide success message after 2 seconds
        if (nameSuccess) {
            setTimeout(function() {
                hideNameModal();
                // Clean up URL
                window.history.replaceState({}, document.title, '/dashboard');
            }, 2000);
        }
    }

    // Handle "Add your name" link click (smooth scroll to account settings)
    const addNameLink = document.querySelector('a[href="#account-settings"]');
    if (addNameLink) {
        addNameLink.addEventListener('click', function(e) {
            e.preventDefault();
            const accountSettings = document.getElementById('account-settings');
            if (accountSettings) {
                accountSettings.scrollIntoView({ behavior: 'smooth' });
                // Show modal after scroll
                setTimeout(showNameModal, 500);
            }
        });
    }
});

// Handle Escape key to close modal
document.addEventListener('keydown', function(e) {
    if (e.key === 'Escape') {
        hideNameModal();
    }
});
