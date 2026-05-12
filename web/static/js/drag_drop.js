// --- Drag-and-Drop ---

function initDragDrop() {
    const padBtns = document.querySelectorAll('.pad-btn');
    padBtns.forEach(btn => {
        btn.addEventListener('dragover', handleDragOver);
        btn.addEventListener('dragleave', handleDragLeave);
        btn.addEventListener('drop', handleDrop);
    });

    // Also allow drop on slicer waveform container
    const waveformContainer = document.querySelector('.waveform-container');
    if (waveformContainer) {
        waveformContainer.addEventListener('dragover', handleDragOver);
        waveformContainer.addEventListener('dragleave', handleDragLeave);
        waveformContainer.addEventListener('drop', handleSlicerDrop);
    }
}

function handleDragOver(e) {
    e.preventDefault();
    e.stopPropagation();
    e.currentTarget.classList.add('drag-over');
    e.dataTransfer.dropEffect = 'copy';
}

function handleDragLeave(e) {
    e.preventDefault();
    // Only remove the class when the cursor actually leaves the element,
    // not when it moves to a child (pad-number, pad-name, etc.).
    if (!e.currentTarget.contains(e.relatedTarget)) {
        e.currentTarget.classList.remove('drag-over');
    }
}

function handleDrop(e) {
    e.preventDefault();
    e.stopPropagation();
    e.currentTarget.classList.remove('drag-over');

    var padIndex = parseInt(e.currentTarget.getAttribute('data-pad-index') || '0');

    // Check for internal browser-to-pad drag (WAV file from file browser).
    var wavPath = e.dataTransfer.getData('text/wav-path');
    if (wavPath) {
        var hasSample = e.currentTarget.classList.contains('has-sample');
        if (hasSample) {
            openAssignModal(wavPath, padIndex);
        } else {
            assignPathToPad(wavPath, padIndex, 'per-pad');
        }
        return;
    }

    // OS file drop handling.
    var files = e.dataTransfer.files;
    if (!files || files.length === 0) return;

    uploadFiles(files, padIndex, files.length > 1 ? 'per-pad' : 'per-pad');
}

function handleSlicerDrop(e) {
    e.preventDefault();
    e.stopPropagation();
    e.currentTarget.classList.remove('drag-over');

    const files = e.dataTransfer.files;
    if (!files || files.length === 0) return;

    // Load the first WAV file into the slicer
    // For local files, we need to upload then load
    const formData = new FormData();
    formData.append('files', files[0]);

    // Upload the file, then load it into the slicer
    fetch('/assign/upload?pad=0&mode=per-pad', {
        method: 'POST',
        body: formData
    }).then(() => {
        // For slicer, we'd need the file path on disk.
        // Since drag-drop gives us browser files, we upload and store them.
        // For now, show a message that the user should use the path input.
        const panel = document.getElementById('slicer-panel');
        if (panel) {
            const msg = panel.querySelector('.export-result');
            if (msg) {
                msg.textContent = 'File uploaded. Use the path input to load into slicer.';
            }
        }
    });
}

function showUploadBar(label) {
    var bar = document.getElementById('global-upload-bar');
    if (!bar) {
        bar = document.createElement('div');
        bar.id = 'global-upload-bar';
        bar.className = 'global-upload-bar';
        bar.innerHTML = '<div class="global-upload-fill" id="global-upload-fill"></div>' +
                        '<span class="global-upload-label" id="global-upload-label"></span>';
        document.body.appendChild(bar);
    }
    document.getElementById('global-upload-label').textContent = label || 'Uploading...';
    document.getElementById('global-upload-fill').style.width = '0%';
    bar.style.display = 'flex';
}

function updateUploadBar(pct, label) {
    var fill = document.getElementById('global-upload-fill');
    var lbl = document.getElementById('global-upload-label');
    if (fill) fill.style.width = Math.min(100, pct) + '%';
    if (lbl && label) lbl.textContent = label;
}

function hideUploadBar() {
    var bar = document.getElementById('global-upload-bar');
    if (bar) bar.style.display = 'none';
}

function uploadFiles(files, padIndex, mode) {
    var totalBytes = 0;
    for (var i = 0; i < files.length; i++) totalBytes += files[i].size;
    var label = files.length === 1 ? files[0].name : files.length + ' files';
    showUploadBar('Uploading ' + label + '...');

    var formData = new FormData();
    for (var i = 0; i < files.length; i++) formData.append('files', files[i]);
    formData.append('pad', String(padIndex));
    formData.append('mode', mode);

    var xhr = new XMLHttpRequest();
    xhr.open('POST', '/assign/upload');
    xhr.upload.addEventListener('progress', function(e) {
        if (!e.lengthComputable) return;
        var pct = (e.loaded / e.total) * 100;
        updateUploadBar(pct, 'Uploading ' + formatBytes(e.loaded) + ' / ' + formatBytes(e.total) + ' (' + Math.round(pct) + '%)');
    });
    xhr.addEventListener('load', function() {
        updateUploadBar(100, 'Done!');
        setTimeout(function() {
            hideUploadBar();
            AudioPlayer.clearCache();
            window.location.reload();
        }, 400);
    });
    xhr.addEventListener('error', function() {
        hideUploadBar();
        console.warn('Upload failed');
    });
    xhr.send(formData);
}

// Init drag-drop on load
document.addEventListener('DOMContentLoaded', initDragDrop);
