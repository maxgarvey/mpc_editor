// --- Drag-and-Drop ---

// Pad-button drag-and-drop uses document-level delegation so listeners are
// attached exactly once regardless of how many HTMX swaps replace the pad grid.
(function() {
    document.addEventListener('dragstart', function(e) {
        var wavPath = e.dataTransfer.getData('text/wav-path');
        // Note: during dragstart, getData returns '' in some browsers; log the types instead.
        console.log('[drag-drop] dragstart — target:', e.target, 'types:', Array.from(e.dataTransfer.types));
    });

    document.addEventListener('dragover', function(e) {
        var btn = e.target.closest('.pad-btn');
        if (!btn) return;
        e.preventDefault();
        btn.classList.add('drag-over');
        e.dataTransfer.dropEffect = 'copy';
    });

    document.addEventListener('dragleave', function(e) {
        var btn = e.target.closest('.pad-btn');
        if (!btn) return;
        // Only remove the class when the cursor actually leaves the button,
        // not when it moves to a child element (pad-number, pad-name, etc.).
        if (!btn.contains(e.relatedTarget)) {
            btn.classList.remove('drag-over');
        }
    });

    document.addEventListener('drop', function(e) {
        var btn = e.target.closest('.pad-btn');
        console.log('[drag-drop] drop event — target:', e.target, 'btn:', btn);
        if (!btn) return;
        e.preventDefault();
        btn.classList.remove('drag-over');

        var padIndex = parseInt(btn.getAttribute('data-pad-index') || '0');

        // Internal browser-to-pad drag (WAV file from file browser).
        var wavPath = e.dataTransfer.getData('text/wav-path');
        console.log('[drag-drop] padIndex:', padIndex, 'wavPath:', JSON.stringify(wavPath), 'has-sample:', btn.classList.contains('has-sample'));
        if (wavPath) {
            if (btn.classList.contains('has-sample')) {
                console.log('[drag-drop] occupied pad — opening assign modal');
                openAssignModal(wavPath, padIndex);
            } else {
                console.log('[drag-drop] empty pad — calling assignPathToPad');
                assignPathToPad(wavPath, padIndex, 'per-pad');
            }
            return;
        }

        // OS file drop.
        var files = e.dataTransfer.files;
        console.log('[drag-drop] no wav-path, files:', files ? files.length : 0);
        if (!files || files.length === 0) return;
        uploadFiles(files, padIndex, 'per-pad');
    });
})();

// initDragDrop handles the slicer waveform container drop target, which lives
// on a separate page and is initialized once on DOMContentLoaded.
function initDragDrop() {
    var waveformContainer = document.querySelector('.waveform-container');
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
    if (!e.currentTarget.contains(e.relatedTarget)) {
        e.currentTarget.classList.remove('drag-over');
    }
}

function handleSlicerDrop(e) {
    e.preventDefault();
    e.stopPropagation();
    e.currentTarget.classList.remove('drag-over');

    const files = e.dataTransfer.files;
    if (!files || files.length === 0) return;

    const formData = new FormData();
    formData.append('files', files[0]);

    fetch('/assign/upload?pad=0&mode=per-pad', {
        method: 'POST',
        body: formData
    }).then(() => {
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

document.addEventListener('DOMContentLoaded', initDragDrop);
