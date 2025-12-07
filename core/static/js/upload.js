let selectedFile = null;
let pollInterval = null;
let selectedProcessingMode = 'fast';
let selectedPreset = 'dj';

// Preset display names (keys match dropdown values, values are what shows in completion message)
const presetDisplayNames = {
    'dj': 'DJ Content',
    'streaming': 'Streaming Content',
    'podcast': 'Podcast Content',
    'broadcast': 'Broadcast Content',
    'custom': 'Custom Content'
};

// Preset descriptions (keys match dropdown values)
const presetDescriptions = {
    'dj': 'Loud and punchy, optimized for impact and energy.',
    'streaming': 'Normalized for streaming platforms like Spotify, Youtube and Apple Music.',
    'podcast': 'Clear and consistent levels for spoken word content.',
    'broadcast': 'EBU R128 standard for radio and television broadcast.',
    'custom': 'Set your own target loudness level.'
};

// Progress simulation system
let progressSimulation = {
    currentProgress: 0,
    targetProgress: 0,
    animationId: null,
    lastStatus: '',
    stageStartTime: null,
    estimatedDuration: null
};

// Get user tier from data attributes
const appContainer = document.getElementById('upload_content');
const userTier = parseInt(appContainer?.dataset.userTier || '0');
const isLoggedIn = appContainer?.dataset.isLoggedIn === 'true';
const isPremiumUser = isLoggedIn && (userTier === 2 || userTier === 3);

// File selection handler
function handleFileSelect(input) {
    const file = input.files[0];
    if (file) {
        selectedFile = file;
        showFileInfo(file);
        document.getElementById('upload-btn').disabled = false;
    }
}

// Show selected file info
function showFileInfo(file) {
    const fileSize = (file.size / (1024 * 1024)).toFixed(2);
    document.getElementById('file-name').textContent = file.name;
    document.getElementById('file-size').textContent = `${fileSize} MB`;

    document.getElementById('drop-content').classList.add('hidden');
    document.getElementById('file-info').classList.remove('hidden');
}

// Remove selected file
function removeFile() {
    selectedFile = null;
    document.getElementById('file-input').value = '';
    document.getElementById('upload-btn').disabled = true;

    document.getElementById('drop-content').classList.remove('hidden');
    document.getElementById('file-info').classList.add('hidden');
}

// Progress helpers
function getDetailedProgress(status, baseProgress, fileSize, elapsedTime, mode = 'precise') {
    if (baseProgress && baseProgress > 0) {
        return {
            progress: Math.min(baseProgress, 100),
            message: getMessageForBackendProgress(status, baseProgress, mode)
        };
    }

    const progressMaps = {
        precise: {
            'uploaded': { progress: 8, message: 'Upload complete, queuing for processing...' },
            'queued': { progress: 12, message: 'Queued for processing...' },
            'processing': {
                stages: [
                    { progress: 20, message: 'Initializing audio processor...', duration: 8 },
                    { progress: 35, message: 'Analyzing audio characteristics...', duration: 15 },
                    { progress: 50, message: 'Measuring loudness levels...', duration: 20 },
                    { progress: 75, message: 'Normalizing audio...', duration: 40 },
                    { progress: 90, message: 'Finalizing processed audio...', duration: 10 },
                    { progress: 95, message: 'Uploading normalized file...', duration: 7 }
                ]
            },
            'completed': { progress: 100, message: 'Processing complete!' },
            'failed': { progress: 0, message: 'Processing failed' }
        },
        fast: {
            'uploaded': { progress: 10, message: 'Upload complete, queuing...' },
            'queued': { progress: 20, message: 'Queued for fast processing...' },
            'processing': {
                stages: [
                    { progress: 30, message: 'Initializing fast processor...', duration: 5 },
                    { progress: 60, message: 'Fast normalizing audio...', duration: 20 },
                    { progress: 90, message: 'Finalizing processed audio...', duration: 5 },
                    { progress: 95, message: 'Uploading normalized file...', duration: 5 }
                ]
            },
            'completed': { progress: 100, message: 'Fast processing complete!' },
            'failed': { progress: 0, message: 'Fast processing failed' }
        }
    };

    const currentMap = progressMaps[mode] || progressMaps.precise;

    if (status === 'processing') {
        const stages = currentMap.processing.stages;
        let cumulativeDuration = 0;
        for (let i = 0; i < stages.length; i++) {
            cumulativeDuration += stages[i].duration;
            if (elapsedTime <= cumulativeDuration || i === stages.length - 1) {
                const stageStartProgress = i > 0 ? stages[i-1].progress : (mode === 'fast' ? 30 : 20);
                const stageEndProgress = stages[i].progress;
                const stageTimeProgress = i > 0 ?
                    (elapsedTime - (cumulativeDuration - stages[i].duration)) / stages[i].duration :
                    elapsedTime / stages[i].duration;

                return {
                    progress: Math.min(stageStartProgress + (stageEndProgress - stageStartProgress) * Math.min(stageTimeProgress, 1), 95),
                    message: stages[i].message
                };
            }
        }
        return { progress: 95, message: 'Uploading normalized file...' };
    }

    return currentMap[status] || { progress: 0, message: 'Processing...' };
}

function animateProgress(targetProgress, message) {
    progressSimulation.targetProgress = Math.min(targetProgress, 100);

    if (progressSimulation.animationId) {
        cancelAnimationFrame(progressSimulation.animationId);
    }

    function updateProgress() {
        const diff = progressSimulation.targetProgress - progressSimulation.currentProgress;
        if (Math.abs(diff) > 0.1) {
            progressSimulation.currentProgress += diff * 0.02;

            const progressBar = document.getElementById('progress-bar');
            const progressText = document.getElementById('progress-text');
            const statusText = document.getElementById('status-text');

            if (progressBar && progressText) {
                const roundedProgress = Math.round(progressSimulation.currentProgress);
                progressBar.style.width = `${progressSimulation.currentProgress}%`;
                progressText.textContent = `${roundedProgress}%`;
            }

            if (statusText && message) {
                statusText.textContent = message;
            }

            progressSimulation.animationId = requestAnimationFrame(updateProgress);
        } else {
            progressSimulation.currentProgress = progressSimulation.targetProgress;
            const progressBar = document.getElementById('progress-bar');
            const progressText = document.getElementById('progress-text');

            if (progressBar && progressText) {
                progressBar.style.width = `${progressSimulation.currentProgress}%`;
                progressText.textContent = `${Math.round(progressSimulation.currentProgress)}%`;
            }
        }
    }

    updateProgress();
}

// Initialize when DOM is ready
document.addEventListener('DOMContentLoaded', function() {
    const dropArea = document.getElementById('drop-area');
    const presetSelect = document.getElementById('preset-select');
    const customInput = document.getElementById('custom-lufs-input');
    const presetDescription = document.getElementById('preset-description');
    const uploadForm = document.getElementById('upload-form');
    const processingModeInputs = document.querySelectorAll('input[name="processing_mode"]');

    // Handle processing mode selection
    processingModeInputs.forEach(input => {
        input.addEventListener('change', function() {
            selectedProcessingMode = this.value;
        });
    });

    // Handle preset selection
    if (presetSelect) {
        // Initialize description on page load
        if (presetDescription) {
            presetDescription.textContent = presetDescriptions[presetSelect.value] || '';
        }

        presetSelect.addEventListener('change', function() {
            selectedPreset = this.value;
            console.log('[Upload] Preset changed to:', selectedPreset);

            // Update description
            if (presetDescription) {
                presetDescription.textContent = presetDescriptions[this.value] || '';
            }

            // Toggle custom input
            if (customInput) {
                if (this.value === 'custom') {
                    customInput.classList.remove('hidden');
                } else {
                    customInput.classList.add('hidden');
                }
            }
        });
    }

    // Drag and drop handling
    if (dropArea) {
        dropArea.addEventListener('dragover', (e) => {
            e.preventDefault();
            dropArea.classList.add('drag-over');
        });

        dropArea.addEventListener('dragleave', (e) => {
            e.preventDefault();
            dropArea.classList.remove('drag-over');
        });

        dropArea.addEventListener('drop', (e) => {
            e.preventDefault();
            dropArea.classList.remove('drag-over');

            const files = e.dataTransfer.files;
            if (files.length > 0) {
                const file = files[0];
                const fileType = file.type;
                const fileName = file.name.toLowerCase();

                let isValidFile = false;
                if (isPremiumUser) {
                    isValidFile = fileType.startsWith('audio/') || fileName.endsWith('.mp3') || fileName.endsWith('.wav');
                } else {
                    isValidFile = fileType.startsWith('audio/mpeg') || fileName.endsWith('.mp3');
                }

                if (isValidFile) {
                    const fileInput = document.getElementById('file-input');
                    fileInput.files = files;
                    handleFileSelect(fileInput);
                } else {
                    if (isPremiumUser) {
                        alert('Please upload an audio file (MP3 or WAV supported)');
                    } else {
                        alert('Please upload an audio file (MP3 only - WAV support available with Premium)');
                    }
                }
            }
        });
    }

    // Handle form submission
    if (uploadForm) {
        uploadForm.addEventListener('submit', async function(e) {
            e.preventDefault();

            const customValue = document.getElementById('custom_lufs_value');

            // Validate custom LUFS if selected
            if (selectedPreset === 'custom' && customValue) {
                const value = parseFloat(customValue.value);
                if (isNaN(value) || value < -30 || value > -2) {
                    alert('Please enter a valid LUFS value between -30 and -2');
                    return;
                }
            }

            if (!selectedFile) {
                alert('Please select a file to upload');
                return;
            }

            try {
                showUploadingState();
                await uploadFileWithPresignedURL(selectedFile, selectedPreset, selectedProcessingMode);
            } catch (error) {
                console.error('Upload error:', error);
                showErrorState(error.message || 'Upload failed. Please try again.');
            }
        });
    }
});

// Status polling
function startStatusPolling(fileId) {
    if (pollInterval) clearInterval(pollInterval);

    const startTime = Date.now();
    progressSimulation.stageStartTime = startTime;
    let lastStatus = '';
    let fileSize = selectedFile ? selectedFile.size : null;

    pollInterval = setInterval(() => {
        const elapsedTime = (Date.now() - startTime) / 1000;

        fetch(`/status/${fileId}`)
            .then(response => response.json())
            .then(data => {
                if (data.status !== lastStatus) {
                    progressSimulation.stageStartTime = Date.now();
                    lastStatus = data.status;
                }

                let finalProgress, finalMessage;

                if (data.progress && data.progress > 0) {
                    finalProgress = Math.min(data.progress, 100);
                    finalMessage = getMessageForBackendProgress(data.status, data.progress, selectedProcessingMode);
                } else {
                    const stageElapsedTime = (Date.now() - progressSimulation.stageStartTime) / 1000;
                    const progressInfo = getDetailedProgress(data.status, data.progress || 0, fileSize, stageElapsedTime, selectedProcessingMode);
                    finalProgress = progressInfo.progress;
                    finalMessage = progressInfo.message;
                }

                animateProgress(finalProgress, finalMessage);

                const cancelBtn = document.getElementById('cancel-btn');
                if (cancelBtn && finalProgress >= 90) {
                    cancelBtn.disabled = true;
                }

                if (data.status === 'completed') {
                    clearInterval(pollInterval);
                    setTimeout(() => showCompletedState(fileId, data), 1000);
                } else if (data.status === 'failed') {
                    clearInterval(pollInterval);
                    showErrorState(data.error || 'Processing failed');
                } else if (data.status === 'cancelled') {
                    clearInterval(pollInterval);
                    showCancelledState();
                }
            })
            .catch(error => {
                console.error('Status polling error:', error);
            });
    }, 1500);
}

function getMessageForBackendProgress(status, progress, mode = 'precise') {
    if (status === 'completed') return 'Processing complete!';
    if (status === 'failed') return 'Processing failed';

    if (status === 'processing') {
        if (progress <= 15) return mode === 'fast' ? 'Initializing fast processor...' : 'Initializing audio processor...';
        if (progress <= 35) return mode === 'fast' ? 'Fast analyzing audio...' : 'Analyzing audio characteristics...';
        if (progress <= 55) return 'Normalizing audio...';
        if (progress <= 85) return 'Finalizing processed audio...';
        if (progress <= 99) return 'Uploading normalized file...';
        return 'Processing complete!';
    }

    const statusMessages = {
        'uploaded': 'Upload complete, queuing for processing...',
        'queued': mode === 'fast' ? 'Queued for fast processing...' : 'Queued for processing...'
    };

    return statusMessages[status] || 'Processing...';
}

// Completed state
function showCompletedState(fileId, data) {
    animateProgress(100, 'Processing complete!');

    const modeIconSrc = selectedProcessingMode === 'fast' ? '/static/images/fast-icon.png' : '/static/images/precise-icon.png';
    const modeText = selectedProcessingMode === 'fast' ? 'Fast Processing' : 'Precise Processing';

    // Get preset display name from the mapping
    const presetText = presetDisplayNames[selectedPreset] || selectedPreset;

    setTimeout(() => {
        document.getElementById('main-container').innerHTML = `
            <div id="completed-state" class="state-transition">
                <div class="text-center">
                    <div class="w-16 h-16 bg-green-500 rounded-full flex items-center justify-center mx-auto mb-6 checkmark">
                        <svg class="w-8 h-8 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7"></path>
                        </svg>
                    </div>

                    <h2 class="text-3xl font-bold mb-4 flex items-center justify-center">
                        <img src="${modeIconSrc}" alt="${modeText}" class="w-6 h-6 mr-2">
                        ${modeText} Complete!
                    </h2>
                    <p class="text-gray-300 mb-8">Your audio has been optimized for <strong>${presetText}</strong></p>

                    <div class="space-y-4">
                        <button onclick="downloadFile('${fileId}')"
                                class="w-full bg-cyan-400 text-gray-900 px-6 py-3 rounded-lg font-bold hover:bg-cyan-300 transition-colors flex items-center justify-center">
                            <svg class="w-5 h-5 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
                            </svg>
                            Download Processed Audio
                        </button>

                        <button onclick="uploadAnother()"
                                class="w-full bg-gray-600 text-white px-6 py-3 rounded-lg hover:bg-gray-500 transition-colors">
                            Process Another File
                        </button>
                    </div>

                    <div class="mt-8 p-4 bg-gray-800 rounded-lg text-sm text-gray-400">
                        <p>✓ High-quality encoding preserved</p>
                        <p>✓ Headroom for streaming services preserved</p>
                        <p>✓ Silence trimmed from the start and end of the file</p>
                        <p>✓ Ready for upload</p>
                    </div>
                </div>
            </div>
        `;
    }, 1000);
}

// Error state
function showErrorState(error) {
    document.getElementById('main-container').innerHTML = `
        <div id="error-state" class="state-transition">
            <div class="text-center">
                <div class="w-16 h-16 bg-red-500 rounded-full flex items-center justify-center mx-auto mb-6">
                    <svg class="w-8 h-8 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
                    </svg>
                </div>

                <h2 class="text-3xl font-bold mb-4 text-red-400">Processing Failed</h2>
                <p class="text-gray-300 mb-8">${error}</p>

                <button onclick="uploadAnother()"
                        class="w-full bg-cyan-400 text-gray-900 px-6 py-3 rounded-lg font-bold hover:bg-cyan-300 transition-colors">
                    Try Again
                </button>
            </div>
        </div>
    `;
}

// Download function
function downloadFile(fileId) {
    const button = document.querySelector(`[onclick="downloadFile('${fileId}')"]`);

    if (button) {
        const originalText = button.innerHTML;
        button.innerHTML = `
            <svg class="animate-spin w-5 h-5 mr-2 inline" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="m4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            Preparing Download...
        `;
        button.disabled = true;

        setTimeout(() => {
            button.innerHTML = originalText;
            button.disabled = false;
        }, 3000);
    }

    const downloadLink = document.createElement('a');
    downloadLink.href = `/download/${fileId}`;
    downloadLink.style.display = 'none';
    downloadLink.download = '';

    document.body.appendChild(downloadLink);

    try {
        downloadLink.click();
    } catch (error) {
        window.location.href = `/download/${fileId}`;
    } finally {
        setTimeout(() => {
            document.body.removeChild(downloadLink);
        }, 1000);
    }
}

function uploadAnother() {
    window.location.reload();
}

// Cancel processing
let currentFileId = null;

function cancelProcessing() {
    if (!currentFileId) return;

    const cancelBtn = document.getElementById('cancel-btn');
    if (cancelBtn) {
        cancelBtn.disabled = true;
        cancelBtn.textContent = 'Cancelling...';
    }

    fetch(`/cancel/${currentFileId}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' }
    })
    .then(response => response.json())
    .then(data => {
        if (data.status === 'cancelled') {
            if (pollInterval) clearInterval(pollInterval);
            showCancelledState();
        } else {
            alert('Failed to cancel: ' + (data.error || 'Unknown error'));
            if (cancelBtn) {
                cancelBtn.disabled = false;
                cancelBtn.textContent = 'Cancel Processing';
            }
        }
    })
    .catch(error => {
        alert('Failed to cancel processing');
        if (cancelBtn) {
            cancelBtn.disabled = false;
            cancelBtn.textContent = 'Cancel Processing';
        }
    });
}

function showCancelledState() {
    document.getElementById('main-container').innerHTML = `
        <div id="cancelled-state" class="state-transition">
            <div class="text-center">
                <div class="w-16 h-16 bg-red-500 rounded-full flex items-center justify-center mx-auto mb-6">
                    <svg class="w-8 h-8 text-white" fill="currentColor" viewBox="0 0 24 24">
                        <path d="M12,2C17.53,2 22,6.47 22,12C22,17.53 17.53,22 12,22C6.47,22 2,17.53 2,12C2,6.47 6.47,2 12,2M15.59,7L12,10.59L8.41,7L7,8.41L10.59,12L7,15.59L8.41,17L12,13.41L15.59,17L17,15.59L13.41,12L17,8.41L15.59,7Z" />
                    </svg>
                </div>

                <h2 class="text-3xl font-bold mb-4 text-red-400">Processing Cancelled</h2>
                <p class="text-gray-300 mb-8">Your processing job has been cancelled.</p>

                <button onclick="uploadAnother()"
                        class="w-full bg-cyan-400 text-gray-900 px-6 py-3 rounded-lg font-bold hover:bg-cyan-300 transition-colors">
                    Upload Another File
                </button>
            </div>
        </div>
    `;
}

// Presigned URL upload flow
async function uploadFileWithPresignedURL(file, preset, processingMode) {
    const presignedData = await getPresignedUploadURL(file);
    await uploadToS3(file, presignedData.upload_url, presignedData.content_type);
    await confirmUploadAndProcess(presignedData.file_id, file.name, preset, processingMode);
}

async function getPresignedUploadURL(file) {
    const params = new URLSearchParams({
        filename: file.name,
        filesize: file.size.toString()
    });

    const response = await fetch(`/api/presigned-upload?${params.toString()}`, {
        method: 'GET',
        credentials: 'include',
        headers: { 'Accept': 'application/json' }
    });

    if (!response.ok) {
        const error = await response.json();
        throw new Error(error.error || 'Failed to get upload URL');
    }

    return await response.json();
}

async function uploadToS3(file, uploadURL, contentType) {
    return new Promise((resolve, reject) => {
        const xhr = new XMLHttpRequest();

        xhr.upload.addEventListener('progress', (event) => {
            if (event.lengthComputable) {
                const percentComplete = Math.round((event.loaded / event.total) * 100);
                updateUploadProgress(percentComplete);
            }
        });

        xhr.addEventListener('load', () => {
            if (xhr.status >= 200 && xhr.status < 300) {
                updateUploadProgress(100);
                resolve();
            } else {
                reject(new Error(`S3 upload failed with status ${xhr.status}`));
            }
        });

        xhr.addEventListener('error', () => reject(new Error('Network error during S3 upload')));
        xhr.addEventListener('abort', () => reject(new Error('Upload cancelled')));

        xhr.open('PUT', uploadURL);
        xhr.setRequestHeader('Content-Type', contentType);
        xhr.send(file);
    });
}

async function confirmUploadAndProcess(fileId, filename, preset, processingMode) {
    const formData = new FormData();
    formData.append('file_id', fileId);
    formData.append('filename', filename);
    formData.append('preset', preset);
    formData.append('processing_mode', processingMode);

    // If custom preset, also send the custom value
    if (preset === 'custom') {
        const customValue = document.getElementById('custom_lufs_value');
        if (customValue) {
            formData.append('custom_lufs_value', customValue.value);
        }
    }

    const response = await fetch('/api/confirm-upload', {
        method: 'POST',
        credentials: 'include',
        body: formData
    });

    if (!response.ok) {
        throw new Error('Failed to confirm upload and start processing');
    }

    const html = await response.text();

    const template = document.getElementById('processing-template');
    const clone = template.content.cloneNode(true);
    document.getElementById('main-container').innerHTML = '';
    document.getElementById('main-container').appendChild(clone);

    const parser = new DOMParser();
    const doc = parser.parseFromString(html, 'text/html');
    const extractedFileId = doc.querySelector('[data-file-id]')?.getAttribute('data-file-id') || fileId;

    currentFileId = extractedFileId;
    progressSimulation.currentProgress = 0;
    progressSimulation.targetProgress = 0;
    animateProgress(5, 'Upload complete, queuing for processing...');
    startStatusPolling(extractedFileId);
}

function showUploadingState() {
    document.getElementById('main-container').innerHTML = `
        <div id="uploading-state" class="state-transition">
            <div class="text-center">
                <div class="spinner mx-auto mb-6"></div>
                <h2 class="text-3xl font-bold mb-4">Uploading Your File</h2>
                <p id="upload-status-text" class="text-gray-300 mb-8">Uploading to cloud storage...</p>

                <div class="bg-gray-800 rounded-lg p-6 mb-6">
                    <div class="flex justify-between items-center mb-2">
                        <span class="text-sm text-gray-400">Upload Progress</span>
                        <span id="upload-progress-text" class="text-sm text-cyan-400">0%</span>
                    </div>
                    <div class="w-full bg-gray-700 rounded-full h-2">
                        <div id="upload-progress-bar" class="bg-cyan-400 h-2 rounded-full transition-all duration-300" style="width: 0%"></div>
                    </div>
                </div>

                <p class="text-gray-400 text-sm">Large files may take a few moments to upload</p>
            </div>
        </div>
    `;
}

function updateUploadProgress(percent) {
    const progressBar = document.getElementById('upload-progress-bar');
    const progressText = document.getElementById('upload-progress-text');
    const statusText = document.getElementById('upload-status-text');

    if (progressBar) progressBar.style.width = `${percent}%`;
    if (progressText) progressText.textContent = `${percent}%`;
    if (statusText) {
        statusText.textContent = percent < 100 ? 'Uploading to cloud storage...' : 'Upload complete, preparing for processing...';
    }
}
