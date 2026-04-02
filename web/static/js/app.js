// MPC Editor - Client-side JavaScript

// Update range slider value displays
document.addEventListener('input', function(e) {
    if (e.target.classList.contains('slider-input')) {
        const display = e.target.parentElement.querySelector('.value-display');
        if (display) {
            display.textContent = e.target.value;
        }
    }
});

// Tab switching (client-side only, no server round-trip)
document.addEventListener('click', function(e) {
    if (e.target.classList.contains('param-tab')) {
        const tabs = e.target.parentElement.querySelectorAll('.param-tab');
        tabs.forEach(t => t.classList.remove('active'));
        e.target.classList.add('active');

        // Map tab index to section
        const sections = e.target.closest('.pad-params-panel').querySelectorAll('.param-section');
        const idx = Array.from(tabs).indexOf(e.target);
        sections.forEach((s, i) => {
            s.style.display = i === idx ? 'block' : 'none';
        });
    }
});

// Initialize: show only first tab's section
document.addEventListener('htmx:afterSettle', function() {
    initTabs();
});

function initTabs() {
    const panel = document.querySelector('.pad-params-panel');
    if (!panel) return;
    const sections = panel.querySelectorAll('.param-section');
    sections.forEach((s, i) => {
        s.style.display = i === 0 ? 'block' : 'none';
    });
}

// Run on initial load
document.addEventListener('DOMContentLoaded', initTabs);

// Keyboard shortcuts
document.addEventListener('keydown', function(e) {
    // Don't intercept when typing in an input
    if (e.target.tagName === 'INPUT' || e.target.tagName === 'SELECT' || e.target.tagName === 'TEXTAREA') {
        return;
    }

    // Space: play selected pad
    if (e.code === 'Space') {
        e.preventDefault();
        const selected = document.querySelector('.pad-btn.selected');
        if (selected) {
            selected.click();
        }
    }

    // Escape: stop playback
    if (e.code === 'Escape') {
        AudioPlayer.stop();
    }
});

// Clear audio cache when program changes
document.addEventListener('htmx:afterSettle', function(e) {
    if (e.detail.pathInfo && e.detail.pathInfo.requestPath === '/program/open') {
        AudioPlayer.clearCache();
    }
    // Re-init drag-and-drop after HTMX updates
    initDragDrop();
});

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
    e.currentTarget.classList.remove('drag-over');
}

function handleDrop(e) {
    e.preventDefault();
    e.stopPropagation();
    e.currentTarget.classList.remove('drag-over');

    const files = e.dataTransfer.files;
    if (!files || files.length === 0) return;

    // Get pad index from the button's hx-get attribute
    const hxGet = e.currentTarget.getAttribute('hx-get');
    const match = hxGet && hxGet.match(/\/pad\/(\d+)/);
    const padIndex = match ? parseInt(match[1]) : 0;

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

function uploadFiles(files, padIndex, mode) {
    const formData = new FormData();
    for (const file of files) {
        formData.append('files', file);
    }
    formData.append('pad', String(padIndex));
    formData.append('mode', mode);

    fetch('/assign/upload', {
        method: 'POST',
        body: formData
    })
    .then(r => r.json())
    .then(data => {
        // Refresh the page to show updated pad names
        AudioPlayer.clearCache();
        window.location.reload();
    })
    .catch(err => {
        console.warn('Upload failed:', err);
    });
}

// Init drag-drop on load
document.addEventListener('DOMContentLoaded', initDragDrop);

// --- File Browser ---

function openBrowser(context, targetInputId) {
    window._browserTargetId = targetInputId;
    var overlay = document.createElement('div');
    overlay.id = 'browser-overlay';
    overlay.className = 'file-browser-overlay';
    overlay.innerHTML = '<div id="file-browser" class="file-browser"></div>';
    overlay.addEventListener('click', function(e) {
        if (e.target === overlay) closeBrowser();
    });
    document.body.appendChild(overlay);
    htmx.ajax('GET', '/browse?context=' + encodeURIComponent(context), '#file-browser');
}

function closeBrowser() {
    var overlay = document.getElementById('browser-overlay');
    if (overlay) overlay.remove();
}

function selectFile(path, context) {
    var target = document.getElementById(window._browserTargetId);
    if (target) target.value = path;
    closeBrowser();
}

function selectDir(path, context) {
    var target = document.getElementById(window._browserTargetId);
    if (target) target.value = path;
    closeBrowser();
}

// --- Save Confirmation Modal ---

function openSaveConfirm() {
    var pathInput = document.getElementById('save-pgm-path');
    var path = pathInput ? pathInput.value : '';

    var overlay = document.createElement('div');
    overlay.id = 'save-confirm-overlay';
    overlay.className = 'file-browser-overlay';
    overlay.addEventListener('click', function(e) {
        if (e.target === overlay) closeSaveConfirm();
    });

    var modal = document.createElement('div');
    modal.className = 'save-confirm-modal';
    modal.innerHTML =
        '<div class="save-confirm-header">Save Program</div>' +
        '<div class="save-confirm-body">' +
            '<label class="save-confirm-label">Save to:</label>' +
            '<input type="text" id="save-confirm-path" class="path-input" value="' +
                path.replace(/"/g, '&quot;') + '" style="width:100%">' +
        '</div>' +
        '<div class="save-confirm-actions">' +
            '<button class="btn-primary" onclick="confirmSave()">Confirm Save</button>' +
            '<button class="btn-sm" onclick="closeSaveConfirm()">Cancel</button>' +
        '</div>';

    overlay.appendChild(modal);
    document.body.appendChild(overlay);

    document.getElementById('save-confirm-path').select();
}

function closeSaveConfirm() {
    var overlay = document.getElementById('save-confirm-overlay');
    if (overlay) overlay.remove();
}

function confirmSave() {
    var pathInput = document.getElementById('save-confirm-path');
    var path = pathInput ? pathInput.value : '';

    var original = document.getElementById('save-pgm-path');
    if (original) original.value = path;

    closeSaveConfirm();

    htmx.ajax('POST', '/program/save', {
        target: 'body',
        values: { path: path }
    });
}
