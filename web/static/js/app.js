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

// Re-initialize UI components when detail panel or other HTMX content changes
document.addEventListener('htmx:afterSettle', function(e) {
    if (e.detail.pathInfo && e.detail.pathInfo.requestPath === '/program/open') {
        AudioPlayer.clearCache();
    }
    // When the detail panel loads a PGM, clear audio cache
    if (e.detail.pathInfo && e.detail.pathInfo.requestPath === '/detail') {
        AudioPlayer.clearCache();
    }
    // Re-init drag-and-drop and tabs after HTMX updates
    initDragDrop();
    initTabs();

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

    // Auto-open when selecting a file in the open-pgm browser
    if (context === 'open-pgm') {
        htmx.ajax('POST', '/program/open', {
            target: 'body',
            values: { path: path }
        });
    }
    // Auto-open save confirm when selecting in save-pgm browser
    if (context === 'save-pgm') {
        openSaveConfirm();
    }
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

// --- New Modal ---

var _importFiles = [];

function openNewModal() {
    var dirInput = document.getElementById('browser-current-dir');
    var destDir = dirInput ? dirInput.value : '';

    var overlay = document.createElement('div');
    overlay.id = 'new-modal-overlay';
    overlay.className = 'file-browser-overlay';
    overlay.addEventListener('click', function(e) {
        if (e.target === overlay) closeNewModal();
    });

    var modal = document.createElement('div');
    modal.className = 'new-modal';
    modal.innerHTML =
        '<div class="new-modal-header">' +
            '<span class="new-modal-title">New</span>' +
            '<button class="new-modal-close" onclick="closeNewModal()">&times;</button>' +
        '</div>' +
        '<div class="new-modal-tabs">' +
            '<button class="new-modal-tab active" data-tab="new-program">New Program</button>' +
            '<button class="new-modal-tab" data-tab="import-files">Import Files</button>' +
        '</div>' +
        '<div class="new-modal-body">' +
            '<div id="new-program-tab" class="new-modal-tab-content">' +
                '<p style="color:#aaa;margin-bottom:16px">Create a blank program. Unsaved changes will be lost.</p>' +
                '<div class="import-actions">' +
                    '<button class="btn-primary" onclick="confirmNewProgram()">Create</button>' +
                '</div>' +
            '</div>' +
            '<div id="import-files-tab" class="new-modal-tab-content" style="display:none">' +
                '<div class="import-dest">' +
                    'Import to: <input type="hidden" id="import-dest-path" value="' + destDir.replace(/"/g, '&quot;') + '">' +
                    '<span class="import-dest-path" onclick="changeImportDest()">' + (destDir || 'workspace root') + '</span>' +
                '</div>' +
                '<div class="import-drop-zone" id="import-drop-zone">' +
                    'Drag and drop files here<br>' +
                    '<span class="import-drop-zone-hint">.wav .mp3 .flac .ogg .aif .m4a .pgm .seq .mid .sng .all</span>' +
                '</div>' +
                '<input type="file" id="import-file-input" multiple accept=".wav,.mp3,.flac,.ogg,.aif,.aiff,.m4a,.wma,.opus,.pgm,.seq,.mid,.sng,.all" style="display:none" onchange="handleImportFileSelect(this)">' +
                '<div style="text-align:center;margin-top:8px">' +
                    '<button class="btn-sm" onclick="document.getElementById(\'import-file-input\').click()">Browse Files</button>' +
                '</div>' +
                '<div class="import-file-list" id="import-file-list"></div>' +
                '<div class="import-actions">' +
                    '<button class="btn-primary" id="import-btn" onclick="doWorkspaceImport()" disabled>Import</button>' +
                '</div>' +
            '</div>' +
        '</div>';

    overlay.appendChild(modal);
    document.body.appendChild(overlay);

    // Tab switching
    var tabs = modal.querySelectorAll('.new-modal-tab');
    tabs.forEach(function(tab) {
        tab.addEventListener('click', function() {
            tabs.forEach(function(t) { t.classList.remove('active'); });
            tab.classList.add('active');
            var target = tab.getAttribute('data-tab');
            document.getElementById('new-program-tab').style.display = target === 'new-program' ? 'block' : 'none';
            document.getElementById('import-files-tab').style.display = target === 'import-files' ? 'block' : 'none';
        });
    });

    // Drop zone handlers
    var dropZone = document.getElementById('import-drop-zone');
    dropZone.addEventListener('dragover', function(e) {
        e.preventDefault();
        e.stopPropagation();
        dropZone.classList.add('drag-over');
        e.dataTransfer.dropEffect = 'copy';
    });
    dropZone.addEventListener('dragleave', function(e) {
        e.preventDefault();
        dropZone.classList.remove('drag-over');
    });
    dropZone.addEventListener('drop', function(e) {
        e.preventDefault();
        e.stopPropagation();
        dropZone.classList.remove('drag-over');
        if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
            addImportFiles(e.dataTransfer.files);
        }
    });

    _importFiles = [];
}

function closeNewModal() {
    var overlay = document.getElementById('new-modal-overlay');
    if (overlay) overlay.remove();
    _importFiles = [];
}

function confirmNewProgram() {
    closeNewModal();
    htmx.ajax('POST', '/program/new', { target: 'body' });
}

function changeImportDest() {
    openBrowser('select-dir', 'import-dest-path');
    // When selectDir is called, it updates the hidden input.
    // We also need to update the displayed path text.
    var checkInterval = setInterval(function() {
        var overlay = document.getElementById('browser-overlay');
        if (!overlay) {
            clearInterval(checkInterval);
            var input = document.getElementById('import-dest-path');
            var display = document.querySelector('.import-dest-path');
            if (input && display) {
                display.textContent = input.value || 'workspace root';
            }
        }
    }, 200);
}

function handleImportFileSelect(input) {
    if (input.files && input.files.length > 0) {
        addImportFiles(input.files);
    }
    input.value = '';
}

function addImportFiles(fileList) {
    for (var i = 0; i < fileList.length; i++) {
        _importFiles.push(fileList[i]);
    }
    renderImportFileList();
}

function removeImportFile(index) {
    _importFiles.splice(index, 1);
    renderImportFileList();
}

function renderImportFileList() {
    var list = document.getElementById('import-file-list');
    var btn = document.getElementById('import-btn');
    if (!list) return;

    if (_importFiles.length === 0) {
        list.innerHTML = '';
        if (btn) btn.disabled = true;
        return;
    }

    var html = '';
    for (var i = 0; i < _importFiles.length; i++) {
        var f = _importFiles[i];
        var ext = f.name.lastIndexOf('.') >= 0 ? f.name.substring(f.name.lastIndexOf('.')) : '';
        var size = f.size < 1024 ? f.size + ' B' :
                   f.size < 1048576 ? Math.round(f.size / 1024) + ' KB' :
                   (f.size / 1048576).toFixed(1) + ' MB';
        html += '<div class="import-file-item">' +
            '<span>' + ext.toUpperCase().replace('.', '[') + '] ' + f.name + ' <span style="color:#888">(' + size + ')</span></span>' +
            '<button class="import-file-remove" onclick="removeImportFile(' + i + ')">&times;</button>' +
            '</div>';
    }
    list.innerHTML = html;
    if (btn) btn.disabled = false;
}

function doWorkspaceImport() {
    if (_importFiles.length === 0) return;

    var formData = new FormData();
    for (var i = 0; i < _importFiles.length; i++) {
        formData.append('files', _importFiles[i]);
    }
    var destInput = document.getElementById('import-dest-path');
    var destDir = destInput ? destInput.value : '';
    formData.append('dest', destDir);

    var btn = document.getElementById('import-btn');
    var hasNonWav = _importFiles.some(function(f) {
        var ext = f.name.substring(f.name.lastIndexOf('.')).toLowerCase();
        return ext !== '.wav' && ext !== '.pgm' && ext !== '.seq' && ext !== '.mid' && ext !== '.sng' && ext !== '.all';
    });
    if (btn) {
        btn.disabled = true;
        btn.textContent = hasNonWav ? 'Importing (transcoding...)' : 'Importing...';
    }

    fetch('/workspace/import', { method: 'POST', body: formData })
        .then(function(r) { return r.json(); })
        .then(function(data) {
            closeNewModal();
            // Refresh the browser nav panel at the destination directory
            htmx.ajax('GET', '/browse/nav?dir=' + encodeURIComponent(destDir), '#file-nav');
        })
        .catch(function(err) {
            console.warn('Import failed:', err);
            if (btn) {
                btn.disabled = false;
                btn.textContent = 'Import';
            }
        });
}

// --- Browser Nav Highlighting ---

// After browser nav re-renders (directory navigation, mkdir), re-apply tab highlighting
document.addEventListener('htmx:afterSettle', function(e) {
    if (e.detail.target && e.detail.target.id === 'file-nav') {
        TabManager.refreshBrowserHighlight();
    }
});

// --- WAV-to-Pad Assignment ---

// Assign a WAV file to the currently selected pad in the active PGM editor.
// Called from the "Assign to Pad" button in the WAV detail view.
function assignWavToPad(wavPath) {
    var selectedBtn = document.querySelector('.pad-btn.selected');
    var padIndex = 0;
    var padLabel = 'A1';
    if (selectedBtn) {
        var padGet = selectedBtn.getAttribute('hx-get');
        var padMatch = padGet && padGet.match(/\/pad\/(\d+)/);
        if (padMatch) padIndex = parseInt(padMatch[1]);
        var bank = String.fromCharCode(65 + Math.floor(padIndex / 16));
        padLabel = bank + ((padIndex % 16) + 1);
    }

    fetch('/assign/path', {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: 'path=' + encodeURIComponent(wavPath) + '&pad=' + padIndex
    })
    .then(function() {
        AudioPlayer.clearCache();
        AudioPlayer.invalidatePad(padIndex);
        // Re-open the PGM to refresh the pad grid
        var lastPgm = document.querySelector('#detail-panel .detail-pgm');
        if (lastPgm) {
            window.location.reload();
        }
    })
    .catch(function(err) {
        console.warn('Assignment failed:', err);
    });
}
