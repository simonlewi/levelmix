console.log('Dashboard scripts loaded');

// Track active polling intervals
const pollingIntervals = new Map();

// Start polling for all processing/queued jobs when page loads
document.addEventListener('DOMContentLoaded', () => {
    const processingJobs = document.querySelectorAll('[data-job-status="processing"], [data-job-status="queued"]');
    processingJobs.forEach(jobElement => {
        const fileId = jobElement.dataset.fileId;
        if (fileId) {
            startJobPolling(fileId);
        }
    });
});

// Poll job status
function startJobPolling(fileId) {
    // Don't start if already polling
    if (pollingIntervals.has(fileId)) {
        return;
    }

    const pollInterval = setInterval(async () => {
        try {
            const response = await fetch(`/status/${fileId}`);
            if (!response.ok) {
                throw new Error('Failed to fetch status');
            }

            const data = await response.json();

            // Update UI based on status
            if (data.status === 'completed') {
                clearInterval(pollInterval);
                pollingIntervals.delete(fileId);
                updateJobUI(fileId, 'completed');
            } else if (data.status === 'failed') {
                clearInterval(pollInterval);
                pollingIntervals.delete(fileId);
                updateJobUI(fileId, 'failed', data.error);
            } else if (data.status === 'processing') {
                updateJobUI(fileId, 'processing');
            }
        } catch (error) {
            console.error(`Error polling status for ${fileId}:`, error);
        }
    }, 2000); // Poll every 2 seconds

    pollingIntervals.set(fileId, pollInterval);
}

// Update job UI based on status
function updateJobUI(fileId, status, errorMessage = null) {
    const jobElement = document.querySelector(`[data-file-id="${fileId}"]`);
    if (!jobElement) {
        // Job not visible, reload page to show updated list
        console.log('Job element not found for', fileId, '- reloading page');
        window.location.reload();
        return;
    }

    console.log('Updating job UI for', fileId, 'to status:', status);

    // More specific selectors
    const iconContainer = jobElement.querySelector('.flex-shrink-0.mr-4');
    const actionContainers = jobElement.querySelectorAll('.flex.items-center.gap-2');
    // The action container is the second one (first is the file info container)
    const actionContainer = actionContainers[actionContainers.length - 1];

    if (!iconContainer || !actionContainer) {
        console.error('Could not find containers for job', fileId);
        console.log('iconContainer:', iconContainer);
        console.log('actionContainers found:', actionContainers.length);
        return;
    }

    if (status === 'completed') {
        // Update icon to success
        iconContainer.innerHTML = `
            <div class="w-12 h-12 bg-success/20 rounded-lg flex items-center justify-center">
                <svg class="w-6 h-6 text-success" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"/>
                </svg>
            </div>
        `;

        // Update action to download button
        actionContainer.innerHTML = `
            <a href="/download/${fileId}" class="inline-flex items-center px-4 py-2 text-sm font-semibold text-white bg-success rounded-lg hover:bg-success/90 transition-all duration-300 hover:-translate-y-0.5">
                <svg class="w-4 h-4 mr-1.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"/>
                </svg>
                Download
            </a>
        `;

        // Update status attribute
        jobElement.dataset.jobStatus = 'completed';
    } else if (status === 'failed') {
        // Update icon to failed
        iconContainer.innerHTML = `
            <div class="w-12 h-12 bg-red-500/20 rounded-lg flex items-center justify-center">
                <svg class="w-6 h-6 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
                </svg>
            </div>
        `;

        // Update action to retry button
        actionContainer.innerHTML = `
            <button onclick="retryJob('${fileId}')" class="inline-flex items-center px-4 py-2 text-sm font-semibold text-legendary-teal border-2 border-legendary-teal rounded-lg hover:bg-legendary-teal/10 transition-all duration-300">
                <svg class="w-4 h-4 mr-1.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
                </svg>
                Retry
            </button>
        `;

        // Update status attribute
        jobElement.dataset.jobStatus = 'failed';
    } else if (status === 'processing' || status === 'queued') {
        // Update icon to processing spinner
        iconContainer.innerHTML = `
            <div class="w-12 h-12 bg-legendary/20 rounded-lg flex items-center justify-center">
                <div class="spinner-legendary w-6 h-6"></div>
            </div>
        `;

        // Replace with processing/queued badge
        actionContainer.innerHTML = `
            <span class="inline-flex items-center px-3 py-1.5 text-xs font-semibold bg-legendary/20 text-legendary border border-legendary/30 rounded-lg">
                <span class="inline-block w-2 h-2 mr-2 bg-legendary rounded-full animate-pulse"></span>
                ${status === 'queued' ? 'Queued' : 'Processing'}
            </span>
        `;

        // Update status attribute
        jobElement.dataset.jobStatus = status;
    }
}

// Retry a failed job
async function retryJob(fileID) {
    try {
        const button = event.target.closest('button');
        const originalHTML = button.innerHTML;
        button.disabled = true;
        button.innerHTML = '<svg class="w-4 h-4 animate-spin" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/></svg> Retrying...';

        const response = await fetch(`/retry/${fileID}`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            }
        });

        if (!response.ok) {
            const data = await response.json();
            throw new Error(data.error || 'Failed to retry job');
        }

        const data = await response.json();
        console.log('Job retry successful:', data);

        // Update UI to show queued status
        updateJobUI(fileID, 'queued'); // Show "Queued" badge
        startJobPolling(fileID); // Start polling for updates
    } catch (error) {
        console.error('Error retrying job:', error);
        alert(`Failed to retry job: ${error.message}`);

        // Restore button state
        if (button) {
            button.disabled = false;
            button.innerHTML = originalHTML;
        }
    }
}

// Open Stripe Billing Portal for subscription management
async function openBillingPortal(event) {
    try {
        const button = event.target.closest('button');
        const originalHTML = button.innerHTML;
        button.disabled = true;
        button.innerHTML = `
            <svg class="animate-spin inline w-4 h-4 mr-1" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            Loading...
        `;

        const response = await fetch('/api/v1/payment/portal', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            }
        });

        if (!response.ok) {
            const errorData = await response.json().catch(() => ({}));
            throw new Error(errorData.error || 'Failed to open billing portal');
        }

        const data = await response.json();

        // Redirect to Stripe billing portal
        if (data.url) {
            button.innerHTML = `
                <svg class="inline w-4 h-4 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path>
                </svg>
                Redirecting...
            `;
            // Small delay so user sees the success state
            setTimeout(() => {
                window.location.href = data.url;
            }, 300);
        } else {
            throw new Error('No portal URL returned');
        }
    } catch (error) {
        console.error('Error opening billing portal:', error);

        // Show user-friendly error message
        const button = event.target.closest('button');
        button.innerHTML = `
            <svg class="inline w-4 h-4 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
            </svg>
            Failed - Try Again
        `;

        // Restore button after 2 seconds
        setTimeout(() => {
            button.disabled = false;
            button.innerHTML = 'Manage Subscription';
        }, 2000);
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

function showEmailModal() {
    const modal = document.getElementById('emailModal');
    if (modal) {
        modal.classList.remove('hidden');
        // Focus on the email input
        const emailInput = document.getElementById('new_email');
        if (emailInput) {
            setTimeout(() => emailInput.focus(), 100);
        }
    }
}

function hideEmailModal() {
    const modal = document.getElementById('emailModal');
    if (modal) {
        modal.classList.add('hidden');
    }
}

function showPasswordModal() {
    const modal = document.getElementById('passwordModal');
    if (modal) {
        modal.classList.remove('hidden');
        // Focus on the current password input
        const passwordInput = document.getElementById('pass_current_password');
        if (passwordInput) {
            setTimeout(() => passwordInput.focus(), 100);
        }
    }
}

function hidePasswordModal() {
    const modal = document.getElementById('passwordModal');
    if (modal) {
        modal.classList.add('hidden');
    }
}

// Close modals when clicking outside
document.addEventListener('DOMContentLoaded', function() {
    const nameModal = document.getElementById('nameModal');
    const emailModal = document.getElementById('emailModal');
    const passwordModal = document.getElementById('passwordModal');

    // Handle clicking outside modals
    [nameModal, emailModal, passwordModal].forEach(modal => {
        if (modal) {
            modal.addEventListener('click', function(e) {
                if (e.target === modal) {
                    if (modal === nameModal) hideNameModal();
                    if (modal === emailModal) hideEmailModal();
                    if (modal === passwordModal) hidePasswordModal();
                }
            });
        }
    });

    // Auto-show modals if there's an error or success in the URL
    const urlParams = new URLSearchParams(window.location.search);
    const nameError = urlParams.get('nameError');
    const nameSuccess = urlParams.get('nameSuccess');
    const emailError = urlParams.get('emailError');
    const emailSuccess = urlParams.get('emailSuccess');
    const passwordError = urlParams.get('passwordError');
    const passwordSuccess = urlParams.get('passwordSuccess');

    if (nameError || nameSuccess) {
        showNameModal();
        if (nameSuccess) {
            setTimeout(function() {
                hideNameModal();
                window.history.replaceState({}, document.title, '/dashboard');
            }, 2000);
        }
    }

    if (emailError || emailSuccess) {
        showEmailModal();
        if (emailSuccess) {
            setTimeout(function() {
                hideEmailModal();
                window.history.replaceState({}, document.title, '/dashboard');
            }, 2000);
        }
    }

    if (passwordError || passwordSuccess) {
        showPasswordModal();
        if (passwordSuccess) {
            setTimeout(function() {
                hidePasswordModal();
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

// Handle Escape key to close all modals
document.addEventListener('keydown', function(e) {
    if (e.key === 'Escape') {
        hideNameModal();
        hideEmailModal();
        hidePasswordModal();
    }
});
