let selectedFile = null;
let pollInterval = null;
let selectedProcessingMode = 'fast';
let selectedPreset = 'dj';
let selectedLufsTarget = -7;


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
    // Update current and target progress
    progressSimulation.targetProgress = Math.min(targetProgress, 100);
    progressSimulation.currentProgress = progressSimulation.targetProgress;

    // Let CSS transitions handle the smooth animation
    const progressBar = document.getElementById('progress-bar');
    const statusText = document.getElementById('status-text');

    if (progressBar) {
        progressBar.style.width = `${progressSimulation.currentProgress}%`;
    }

    if (statusText && message) {
        statusText.textContent = message;
    }
}

// Initialize when DOM is ready
document.addEventListener('DOMContentLoaded', function() {
    const dropArea = document.getElementById('drop-area');
    const uploadContent = document.getElementById('upload_content');
    const uploadForm = document.getElementById('upload-form');
    const processingModeInputs = document.querySelectorAll('input[name="processing_mode"]');
    const presetInputs = document.querySelectorAll('input[name="preset"]');

    // Handle processing mode selection
    processingModeInputs.forEach(input => {
        input.addEventListener('change', function() {
            selectedProcessingMode = this.value;
        });
    });

    // Handle LUFS slider (only exists for premium/professional users)
    const lufsSlider = document.getElementById('lufs-slider');
    const lufsValue = document.getElementById('lufs-value');
    const lufsContainer = document.getElementById('lufs-container');

    if (lufsSlider && lufsValue && lufsContainer) {
        // Initialize display from slider's current value
        selectedLufsTarget = parseInt(lufsSlider.value);
        lufsValue.textContent = selectedLufsTarget;

        // Activate custom LUFS and deselect presets
        function activateCustomLufs() {
            lufsContainer.classList.add('active');
            // Uncheck all preset radio buttons
            presetInputs.forEach(input => {
                input.checked = false;
            });
            selectedPreset = 'custom';
        }

        // Deactivate custom LUFS when preset is selected
        function deactivateCustomLufs() {
            lufsContainer.classList.remove('active');
        }

        // Activate on click
        lufsContainer.addEventListener('click', function() {
            activateCustomLufs();
        });

        // Update value on input and ensure active state
        lufsSlider.addEventListener('input', function() {
            selectedLufsTarget = parseInt(this.value);
            lufsValue.textContent = selectedLufsTarget;
            activateCustomLufs();
        });

        // Handle preset card selection - deactivate custom LUFS
        presetInputs.forEach(input => {
            input.addEventListener('change', function() {
                selectedPreset = this.value;
                deactivateCustomLufs();
                console.log('[Upload] Preset changed to:', selectedPreset);
            });
        });
    } else {
        // If no LUFS slider (non-premium users), just handle preset selection
        presetInputs.forEach(input => {
            input.addEventListener('change', function() {
                selectedPreset = this.value;
                console.log('[Upload] Preset changed to:', selectedPreset);
            });
        });
    }

    // Drag and drop handling - make entire page droppable
    function handleDragOver(e) {
        e.preventDefault();
        e.stopPropagation();
        if (dropArea) {
            dropArea.classList.add('drag-over');
        }
    }

    function handleDragLeave(e) {
        e.preventDefault();
        e.stopPropagation();
        // Only remove drag-over if we're leaving the upload content entirely
        if (e.target === uploadContent && dropArea) {
            dropArea.classList.remove('drag-over');
        }
    }

    function handleDrop(e) {
        e.preventDefault();
        e.stopPropagation();
        if (dropArea) {
            dropArea.classList.remove('drag-over');
        }

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
    }

    // Attach drag-and-drop to entire upload content area
    if (uploadContent) {
        uploadContent.addEventListener('dragover', handleDragOver);
        uploadContent.addEventListener('dragleave', handleDragLeave);
        uploadContent.addEventListener('drop', handleDrop);
    }

    // Also keep the drop area events for visual feedback
    if (dropArea) {
        dropArea.addEventListener('dragover', (e) => {
            e.preventDefault();
            e.stopPropagation();
        });
    }

    // Handle form submission
    if (uploadForm) {
        uploadForm.addEventListener('submit', async function(e) {
            e.preventDefault();

            if (!selectedFile) {
                alert('Please select a file to upload');
                return;
            }

            try {
                showUploadingState();
                await uploadFileWithPresignedURL(selectedFile, selectedPreset, selectedProcessingMode, selectedLufsTarget);
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

                // Update status badge based on current status
                updateStatusBadge(data.status, finalProgress);

                // Update file metadata from backend (duration and extension)
                updateFileMetadataFromBackend(data);

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

    // Get preset display name from the mapping
    const presetText = presetDisplayNames[selectedPreset] || selectedPreset;
    const fullFileName = selectedFile ? selectedFile.name : 'Your audio file';
    const fileName = fullFileName.substring(0, fullFileName.lastIndexOf('.')) || fullFileName;

    // Format duration
    let durationText = '--';
    if (data.durationSeconds !== undefined && data.durationSeconds !== null) {
        const minutes = Math.floor(data.durationSeconds / 60);
        const seconds = data.durationSeconds % 60;
        durationText = `${minutes}:${seconds.toString().padStart(2, '0')}`;
    }

    // Get extension
    const extensionText = data.format ? data.format.toUpperCase() : '--';

    // Get queue type
    const queueText = isPremiumUser ? 'Fast Queue' : 'Standard Queue';
    const queueClass = isPremiumUser ? 'queue-badge queue-fast' : 'queue-badge queue-precise';

    // Check for silence trimmed
    const showSilenceTrimNotice = data.silenceTrimmed || false;

    setTimeout(() => {
        const mainContainer = document.getElementById('main-container');
        mainContainer.className = 'state-transition mx-auto'; // Keep same layout as processing
        mainContainer.style.maxWidth = '960px'; // Match processing state width
        mainContainer.innerHTML = `
            <div id="completed-state" class="state-transition">
                <div class="processing-card complete border-2 rounded-lg p-8">
                    <!-- File Info Header -->
                    <div class="file-info-header flex items-center gap-6 mb-6">
                        <div class="file-icon-box w-16 h-16 bg-gradient-to-br from-legendary to-legendary-teal rounded-lg flex items-center justify-center flex-shrink-0">
                            <svg class="w-8 h-8 text-white" fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"/>
                            </svg>
                        </div>
                        <div class="file-details flex-1 min-w-0">
                            <div class="text-white text-lg font-semibold truncate mb-2 text-left">${fileName}</div>
                            <div class="file-meta-display flex gap-3 text-sm text-slate-400 items-center text-left">
                                <span>${durationText}</span>
                                <span>•</span>
                                <span class="uppercase">${extensionText}</span>
                                <span>•</span>
                                <span class="${queueClass}">${queueText}</span>
                            </div>
                            ${showSilenceTrimNotice ? '<div class="text-xs text-teal-400 mt-3 text-left">• Silence trimmed from start/end</div>' : ''}
                        </div>
                        <span class="status-badge status-complete flex-shrink-0">
                            <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"/>
                            </svg>
                            Complete
                        </span>
                    </div>

                    <!-- Progress Bar -->
                    <div class="bg-slate-700 rounded-full mb-4" style="height: 8px;">
                        <div class="progress-bar complete rounded-full" style="width: 100%; height: 8px;"></div>
                    </div>

                    <!-- Success Message and Download Button -->
                    <p class="text-sm text-success font-medium mb-6">Processing complete! Ready to download.</p>

                    <div class="space-y-3">
                        <button onclick="downloadFile('${fileId}')"
                                class="w-full bg-success text-white px-6 py-3 rounded-lg font-semibold hover:bg-success/90 transition-all duration-300 flex items-center justify-center gap-2">
                            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"/>
                            </svg>
                            Download Processed Audio
                        </button>

                        <button onclick="uploadAnother()"
                                class="w-full bg-slate-700 text-white px-6 py-3 rounded-lg font-semibold hover:bg-slate-600 transition-colors">
                            Process Another File
                        </button>
                    </div>
                </div>
            </div>
        `;
    }, 1000);
}

// Error state
function showErrorState(error) {
    const mainContainer = document.getElementById('main-container');
    mainContainer.className = 'state-transition mx-auto';
    mainContainer.style.maxWidth = '960px';
    mainContainer.innerHTML = `
        <div id="error-state" class="state-transition">
            <div class="text-center">
                <div class="w-16 h-16 bg-red-500 rounded-full flex items-center justify-center mx-auto mb-6">
                    <svg class="w-8 h-8 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
                    </svg>
                </div>

                <h2 class="text-3xl font-bold mb-4 text-red-400">Processing Failed</h2>
                <p class="text-slate-300 mb-8">${error}</p>

                <button onclick="uploadAnother()"
                        class="w-full bg-legendary-teal text-white px-6 py-3 rounded-lg font-bold hover:bg-legendary-teal/90 transition-all duration-300 hover:-translate-y-0.5">
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
    const mainContainer = document.getElementById('main-container');
    mainContainer.className = 'state-transition mx-auto';
    mainContainer.style.maxWidth = '960px';
    mainContainer.innerHTML = `
        <div id="cancelled-state" class="state-transition">
            <div class="text-center">
                <div class="w-16 h-16 bg-red-500 rounded-full flex items-center justify-center mx-auto mb-6">
                    <svg class="w-8 h-8 text-white" fill="currentColor" viewBox="0 0 24 24">
                        <path d="M12,2C17.53,2 22,6.47 22,12C22,17.53 17.53,22 12,22C6.47,22 2,17.53 2,12C2,6.47 6.47,2 12,2M15.59,7L12,10.59L8.41,7L7,8.41L10.59,12L7,15.59L8.41,17L12,13.41L15.59,17L17,15.59L13.41,12L17,8.41L15.59,7Z" />
                    </svg>
                </div>

                <h2 class="text-3xl font-bold mb-4 text-red-400">Processing Cancelled</h2>
                <p class="text-slate-300 mb-8">Your processing job has been cancelled.</p>

                <button onclick="uploadAnother()"
                        class="w-full bg-legendary-teal text-white px-6 py-3 rounded-lg font-bold hover:bg-legendary-teal/90 transition-all duration-300 hover:-translate-y-0.5">
                    Upload Another File
                </button>
            </div>
        </div>
    `;
}

// Presigned URL upload flow
async function uploadFileWithPresignedURL(file, preset, processingMode, lufsTarget) {
    const presignedData = await getPresignedUploadURL(file);
    await uploadToS3(file, presignedData.upload_url, presignedData.content_type);
    await confirmUploadAndProcess(presignedData.file_id, file.name, preset, processingMode, lufsTarget);
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
        let uploadStarted = false;
        let bytesUploaded = 0;

        xhr.upload.addEventListener('progress', (event) => {
            uploadStarted = true;
            bytesUploaded = event.loaded;
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
                // Provide more specific error messages based on the failure
                let errorMessage = `Upload failed with status ${xhr.status}`;

                // Check if this might be a cloud storage issue
                if (!uploadStarted || bytesUploaded === 0) {
                    errorMessage = 'Unable to read the file. If you selected this file from cloud storage (Google Drive, Dropbox, OneDrive, etc.), please download it to your device first and try again.';
                } else if (xhr.status === 403) {
                    errorMessage = 'Upload permission denied. Please try again.';
                } else if (xhr.status >= 500) {
                    errorMessage = 'Server error during upload. Please try again in a moment.';
                }

                reject(new Error(errorMessage));
            }
        });

        xhr.addEventListener('error', () => {
            // Network errors often occur when trying to upload from cloud storage
            let errorMessage = 'Unable to upload the file. ';
            if (!uploadStarted || bytesUploaded === 0) {
                errorMessage += 'If you selected this file from cloud storage (Google Drive, Dropbox, OneDrive, etc.), please download it to your device first and try again.';
            } else {
                errorMessage += 'Please check your internet connection and try again.';
            }
            reject(new Error(errorMessage));
        });

        xhr.addEventListener('abort', () => reject(new Error('Upload cancelled')));

        xhr.open('PUT', uploadURL);
        xhr.setRequestHeader('Content-Type', contentType);

        try {
            xhr.send(file);
        } catch (e) {
            // This can happen when the file isn't actually available locally
            reject(new Error('Unable to read the file. If you selected this file from cloud storage (Google Drive, Dropbox, OneDrive, etc.), please download it to your device first and try again.'));
        }
    });
}

async function confirmUploadAndProcess(fileId, filename, preset, processingMode, lufsTarget) {
    const formData = new FormData();
    formData.append('file_id', fileId);
    formData.append('filename', filename);
    formData.append('preset', preset);
    formData.append('processing_mode', processingMode);

    // Only send target_lufs if it's defined (premium/professional users)
    if (lufsTarget !== undefined && lufsTarget !== null) {
        formData.append('target_lufs', lufsTarget.toString());
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
    const mainContainer = document.getElementById('main-container');
    mainContainer.innerHTML = '';
    mainContainer.className = 'state-transition mx-auto'; // Remove max-w-md for processing state but keep centered
    mainContainer.appendChild(clone);

    // Populate file information
    populateFileInfo(filename);

    const parser = new DOMParser();
    const doc = parser.parseFromString(html, 'text/html');
    const extractedFileId = doc.querySelector('[data-file-id]')?.getAttribute('data-file-id') || fileId;

    currentFileId = extractedFileId;
    progressSimulation.currentProgress = 0;
    progressSimulation.targetProgress = 0;
    animateProgress(1, 'Upload complete, queuing for processing...');
    startStatusPolling(extractedFileId);
}

function populateFileInfo(filename) {
    const fileNameDisplay = document.getElementById('file-name-display');
    const queueTypeBadge = document.getElementById('queue-type-badge');
    const extensionElement = document.getElementById('file-extension');

    if (fileNameDisplay) {
        // Remove extension from display name
        const nameWithoutExt = filename.substring(0, filename.lastIndexOf('.')) || filename;
        fileNameDisplay.textContent = nameWithoutExt;
    }

    // Populate extension from filename
    if (extensionElement) {
        const ext = filename.substring(filename.lastIndexOf('.') + 1).toUpperCase();
        if (ext) {
            extensionElement.textContent = ext;
        }
    }

    if (queueTypeBadge) {
        // Queue type based on user tier
        if (isPremiumUser) {
            queueTypeBadge.textContent = 'Fast Queue';
            queueTypeBadge.className = 'queue-badge queue-fast';
        } else {
            queueTypeBadge.textContent = 'Standard Queue';
            queueTypeBadge.className = 'queue-badge queue-precise';
        }
    }
}

function updateFileMetadataFromBackend(data) {
    // Update duration - only update if we have valid duration data
    const durationElement = document.getElementById('file-duration');
    if (durationElement && data.durationSeconds !== undefined && data.durationSeconds !== null) {
        const minutes = Math.floor(data.durationSeconds / 60);
        const seconds = data.durationSeconds % 60;

        // Format as "MM:SS" for better precision
        const formattedDuration = `${minutes}:${seconds.toString().padStart(2, '0')}`;
        durationElement.textContent = formattedDuration;
    }

    // Update extension from backend if available and not already set
    const extensionElement = document.getElementById('file-extension');
    if (data.format && extensionElement && extensionElement.textContent === '--') {
        extensionElement.textContent = data.format.toUpperCase();
    }

    // Show silence trim notice if silence was removed
    const silenceTrimNotice = document.getElementById('silence-trim-notice');
    if (data.silenceTrimmed && silenceTrimNotice) {
        silenceTrimNotice.classList.remove('hidden');
    }
}

function updateStatusBadge(status, progress) {
    const statusBadge = document.getElementById('status-badge');
    const processingCard = document.querySelector('.processing-card');

    if (!statusBadge) return;

    const roundedProgress = Math.round(progress);

    // Check if status is a processing state
    const processingStates = ['processing', 'downloading', 'fast_analyzing', 'precise_analyzing', 'normalizing', 'uploading'];
    const isProcessing = processingStates.includes(status);

    if (status === 'queued' || status === 'uploaded') {
        statusBadge.className = 'status-badge status-queued';
        statusBadge.innerHTML = `
            <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm1-12a1 1 0 10-2 0v4a1 1 0 00.293.707l2.828 2.829a1 1 0 101.415-1.415L11 9.586V6z"/>
            </svg>
            Queued
        `;
        if (processingCard) processingCard.classList.remove('processing', 'complete');
    } else if (isProcessing) {
        statusBadge.className = 'status-badge status-processing';
        statusBadge.innerHTML = `${roundedProgress}%`;
        if (processingCard) {
            processingCard.classList.add('processing');
            processingCard.classList.remove('complete');
        }
    } else if (status === 'completed') {
        statusBadge.className = 'status-badge status-complete';
        statusBadge.innerHTML = `
            <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"/>
            </svg>
            Complete
        `;
        if (processingCard) {
            processingCard.classList.add('complete');
            processingCard.classList.remove('processing');
        }

        const progressBar = document.getElementById('progress-bar');
        if (progressBar) progressBar.classList.add('complete');
    }
}

function showUploadingState() {
    const mainContainer = document.getElementById('main-container');
    mainContainer.className = 'state-transition mx-auto';
    mainContainer.style.maxWidth = '960px';
    mainContainer.innerHTML = `
        <div id="uploading-state" class="state-transition">
            <div class="text-center">
                <div class="spinner-legendary mx-auto mb-6 w-16 h-16"></div>
                <h2 class="text-3xl font-bold mb-4 bg-gradient-to-r from-legendary to-legendary-teal bg-clip-text text-transparent">Uploading Your File</h2>
                <p id="upload-status-text" class="text-slate-300 mb-8">Uploading to cloud storage...</p>

                <div class="bg-slate-800 border-2 border-legendary/30 rounded-lg p-6 mb-6">
                    <div class="flex justify-between items-center mb-2">
                        <span class="text-sm text-slate-400">Upload Progress</span>
                        <span id="upload-progress-text" class="text-sm text-legendary-teal font-semibold">0%</span>
                    </div>
                    <div class="w-full bg-slate-700 rounded-full h-2">
                        <div id="upload-progress-bar" class="bg-gradient-to-r from-legendary to-legendary-teal h-2 rounded-full transition-all duration-300" style="width: 0%"></div>
                    </div>
                </div>

                <p class="text-slate-400 text-sm">Large files may take a few moments to upload</p>
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
