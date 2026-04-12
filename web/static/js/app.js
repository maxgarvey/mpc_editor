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

// Track active param tab so it survives HTMX re-renders
var _activeParamTab = 0;

// Param tab switching (client-side only, no server round-trip)
document.addEventListener('click', function(e) {
    if (e.target.classList.contains('param-tab')) {
        const tabs = e.target.parentElement.querySelectorAll('.param-tab');
        tabs.forEach(t => t.classList.remove('active'));
        e.target.classList.add('active');

        // Map tab index to section
        const sections = e.target.closest('.pad-params-panel').querySelectorAll('.param-section');
        const idx = Array.from(tabs).indexOf(e.target);
        _activeParamTab = idx;
        sections.forEach((s, i) => {
            s.style.display = i === idx ? 'block' : 'none';
        });
    }
});

// Bank tab highlighting (bank tabs live outside the HTMX swap target)
document.addEventListener('click', function(e) {
    if (e.target.classList.contains('bank-tab')) {
        const tabs = e.target.parentElement.querySelectorAll('.bank-tab');
        tabs.forEach(t => t.classList.remove('active'));
        e.target.classList.add('active');
    }
});

// Pad button highlighting (pad grid isn't re-rendered on pad select)
document.addEventListener('click', function(e) {
    var btn = e.target.closest('.pad-btn');
    if (btn) {
        document.querySelectorAll('.pad-btn.selected').forEach(function(b) {
            b.classList.remove('selected');
        });
        btn.classList.add('selected');
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
    const tabs = panel.querySelectorAll('.param-tab');
    const idx = _activeParamTab;
    sections.forEach((s, i) => {
        s.style.display = i === idx ? 'block' : 'none';
    });
    tabs.forEach((t, i) => {
        t.classList.toggle('active', i === idx);
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

    // Check for internal browser-to-pad drag (WAV file from file browser).
    var wavPath = e.dataTransfer.getData('text/wav-path');
    if (wavPath) {
        var hxGet = e.currentTarget.getAttribute('hx-get');
        var match = hxGet && hxGet.match(/\/pad\/(\d+)/);
        var padIndex = match ? parseInt(match[1]) : 0;
        var hasSample = e.currentTarget.classList.contains('has-sample');

        if (hasSample) {
            openAssignModal(wavPath, padIndex);
        } else {
            assignPathToPad(wavPath, padIndex, 'per-pad');
        }
        return;
    }

    // OS file drop handling.
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

// --- Settings Modal ---

function openSettingsModal() {
    var overlay = document.createElement('div');
    overlay.id = 'settings-overlay';
    overlay.className = 'file-browser-overlay';
    overlay.addEventListener('click', function(e) {
        if (e.target === overlay) closeSettingsModal();
    });

    var modal = document.createElement('div');
    modal.className = 'settings-modal';
    modal.id = 'settings-modal';
    modal.innerHTML = '<div class="settings-header">Settings</div>' +
        '<div id="settings-content" class="settings-body">Loading...</div>' +
        '<div class="settings-actions">' +
            '<button class="btn-primary" onclick="saveSettings()">Save</button>' +
            '<button class="btn-primary" onclick="closeSettingsModal()">Cancel</button>' +
        '</div>';

    overlay.appendChild(modal);
    document.body.appendChild(overlay);

    // Fetch settings content from server
    fetch('/settings')
        .then(function(r) { return r.text(); })
        .then(function(html) {
            document.getElementById('settings-content').innerHTML = html;
        });
}

function closeSettingsModal() {
    var overlay = document.getElementById('settings-overlay');
    if (overlay) overlay.remove();
}

function saveSettings() {
    var workspace = document.getElementById('settings-workspace');
    var profile = document.getElementById('settings-profile');

    var params = new URLSearchParams();
    if (workspace) params.set('workspace', workspace.value);
    if (profile) params.set('profile', profile.value);

    fetch('/settings/save', {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: params.toString()
    }).then(function(r) {
        if (r.ok) {
            // Check for HX-Redirect header
            var redirect = r.headers.get('HX-Redirect');
            if (redirect) {
                window.location.href = redirect;
            } else {
                window.location.reload();
            }
        }
    });
}

// --- New Folder Modal ---

function openMkdirModal(parent, context, htmxTarget) {
    var overlay = document.createElement('div');
    overlay.id = 'mkdir-overlay';
    overlay.className = 'file-browser-overlay';
    overlay.addEventListener('click', function(e) {
        if (e.target === overlay) closeMkdirModal();
    });

    var modal = document.createElement('div');
    modal.className = 'save-confirm-modal';
    modal.innerHTML =
        '<div class="save-confirm-header">New Folder</div>' +
        '<div class="save-confirm-body">' +
            '<label class="save-confirm-label">Folder name:</label>' +
            '<input type="text" id="mkdir-name" class="path-input" placeholder="My Folder" style="width:100%" maxlength="255">' +
        '</div>' +
        '<div class="save-confirm-actions">' +
            '<button class="btn-primary" id="mkdir-confirm-btn" onclick="confirmMkdir()">Create</button>' +
            '<button class="btn-primary" onclick="closeMkdirModal()">Cancel</button>' +
        '</div>';

    overlay.appendChild(modal);
    document.body.appendChild(overlay);

    // Store context for the confirm action.
    window._mkdirParent = parent;
    window._mkdirContext = context;
    window._mkdirTarget = htmxTarget;

    var nameInput = document.getElementById('mkdir-name');
    nameInput.focus();
    nameInput.addEventListener('keydown', function(e) {
        if (e.key === 'Enter') confirmMkdir();
        if (e.key === 'Escape') closeMkdirModal();
    });
}

function closeMkdirModal() {
    var overlay = document.getElementById('mkdir-overlay');
    if (overlay) overlay.remove();
}

function confirmMkdir() {
    var nameInput = document.getElementById('mkdir-name');
    var name = nameInput ? nameInput.value.trim() : '';
    if (!name) {
        nameInput.focus();
        return;
    }

    closeMkdirModal();

    htmx.ajax('POST', '/workspace/mkdir', {
        target: window._mkdirTarget,
        values: {
            parent: window._mkdirParent,
            context: window._mkdirContext,
            name: name
        }
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
            '<button class="new-modal-tab active" data-tab="new-project">New Project</button>' +
            '<button class="new-modal-tab" data-tab="new-program">New Program</button>' +
            '<button class="new-modal-tab" data-tab="import-files">Import Files</button>' +
        '</div>' +
        '<div class="new-modal-body">' +
            '<div id="new-project-tab" class="new-modal-tab-content">' +
                '<p style="color:#aaa;margin-bottom:12px">Create a self-contained project folder with a blank program inside. ' +
                    'Samples assigned to this program will be saved in the same folder ' +
                    'so it works directly on MPC 1000 CF cards.</p>' +
                '<div style="margin-bottom:12px">' +
                    '<label style="color:#aaa;display:block;margin-bottom:4px">Project name <span style="color:#666">(max 16 chars)</span></label>' +
                    '<input type="text" id="new-project-name" class="path-input" maxlength="16" ' +
                        'placeholder="e.g. beat001" style="width:100%" ' +
                        'oninput="validateProjectName(this)">' +
                    '<div id="project-name-hint" style="font-size:11px;margin-top:4px;color:#666"></div>' +
                '</div>' +
                '<div class="import-actions">' +
                    '<button class="btn-primary" id="new-project-btn" onclick="confirmNewProject()" disabled>Create Project</button>' +
                '</div>' +
            '</div>' +
            '<div id="new-program-tab" class="new-modal-tab-content" style="display:none">' +
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
                '<div class="import-attribution">' +
                    '<label class="settings-label">Source / Attribution (optional)</label>' +
                    '<input type="text" id="import-source" class="path-input" style="width:100%" placeholder="e.g. Splice, freesound.org, recorded live">' +
                    '<p class="settings-hint">Applied to all imported WAV files as their source.</p>' +
                '</div>' +
                '<div class="import-actions">' +
                    '<button class="btn-primary" id="import-btn" onclick="confirmImportDest()" disabled>Import</button>' +
                '</div>' +
            '</div>' +
        '</div>';

    overlay.appendChild(modal);
    document.body.appendChild(overlay);

    // Tab switching
    var tabs = modal.querySelectorAll('.new-modal-tab');
    var tabIds = ['new-project-tab', 'new-program-tab', 'import-files-tab'];
    tabs.forEach(function(tab) {
        tab.addEventListener('click', function() {
            tabs.forEach(function(t) { t.classList.remove('active'); });
            tab.classList.add('active');
            var target = tab.getAttribute('data-tab');
            tabIds.forEach(function(id) {
                var el = document.getElementById(id);
                if (el) el.style.display = (id === target + '-tab') ? 'block' : 'none';
            });
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

function validateProjectName(input) {
    var name = input.value.trim();
    var btn = document.getElementById('new-project-btn');
    var hint = document.getElementById('project-name-hint');
    if (!btn || !hint) return;

    if (name.length === 0) {
        btn.disabled = true;
        hint.textContent = '';
        hint.style.color = '#666';
        return;
    }

    // Check for invalid characters
    if (/[\/\\]/.test(name) || name === '.' || name === '..') {
        btn.disabled = true;
        hint.textContent = 'Invalid characters in name';
        hint.style.color = '#ff6b4a';
        return;
    }

    // Warn about spaces (MPC compatibility)
    if (/\s/.test(name)) {
        hint.textContent = 'Spaces may cause issues on some MPC firmware';
        hint.style.color = '#c8a040';
    } else {
        hint.textContent = 'Creates: ' + name + '/' + name + '.pgm';
        hint.style.color = '#666';
    }

    btn.disabled = false;
}

function confirmNewProject() {
    var input = document.getElementById('new-project-name');
    var name = input ? input.value.trim() : '';
    if (!name) return;

    var dirInput = document.getElementById('browser-current-dir');
    var parentDir = dirInput ? dirInput.value : '';

    closeNewModal();
    htmx.ajax('POST', '/project/new', {
        target: 'body',
        values: { name: name, parent: parentDir }
    });
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

function confirmImportDest() {
    if (_importFiles.length === 0) return;

    var destInput = document.getElementById('import-dest-path');
    var destDir = destInput ? destInput.value : '';
    var displayDest = destDir || 'workspace root';
    var count = _importFiles.length;
    var sourceInput = document.getElementById('import-source');
    var source = sourceInput ? sourceInput.value.trim() : '';

    var msg = 'Import ' + count + ' file' + (count > 1 ? 's' : '') + ' to:\n' + displayDest;
    if (source) {
        msg += '\n\nSource attribution: ' + source;
    }

    if (confirm(msg)) {
        doWorkspaceImport();
    }
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

    var sourceInput = document.getElementById('import-source');
    var source = sourceInput ? sourceInput.value.trim() : '';
    if (source) {
        formData.append('source', source);
    }

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

// --- Browser Refresh (triggered by server via HX-Trigger) ---

document.addEventListener('refreshBrowser', function() {
    htmx.ajax('GET', '/browse/nav', { target: '#file-nav' });
});

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

// --- Drag-to-Pad Assignment Modal ---

function openAssignModal(wavPath, padIndex) {
    var bank = String.fromCharCode(65 + Math.floor(padIndex / 16));
    var padLabel = bank + ((padIndex % 16) + 1);
    var sampleName = wavPath.split('/').pop();

    var overlay = document.createElement('div');
    overlay.id = 'assign-overlay';
    overlay.className = 'file-browser-overlay';
    overlay.addEventListener('click', function(e) {
        if (e.target === overlay) closeAssignModal();
    });

    var modal = document.createElement('div');
    modal.className = 'save-confirm-modal';
    modal.innerHTML =
        '<div class="save-confirm-header">Assign Sample</div>' +
        '<div class="save-confirm-body">' +
            '<p>Pad ' + padLabel + ' already has a sample.</p>' +
            '<p>Assign <strong>' + sampleName + '</strong>?</p>' +
        '</div>' +
        '<div class="save-confirm-actions">' +
            '<button class="btn-primary" onclick="assignReplace()">Replace</button>' +
            '<button class="btn-primary" onclick="assignAddLayer()">Layer</button>' +
            '<button class="btn-primary" onclick="closeAssignModal()">Cancel</button>' +
        '</div>';

    overlay.appendChild(modal);
    document.body.appendChild(overlay);

    window._pendingAssign = { wavPath: wavPath, padIndex: padIndex };
}

function closeAssignModal() {
    var overlay = document.getElementById('assign-overlay');
    if (overlay) overlay.remove();
    window._pendingAssign = null;
}

function assignReplace() {
    var a = window._pendingAssign;
    closeAssignModal();
    if (a) assignPathToPad(a.wavPath, a.padIndex, 'replace');
}

function assignAddLayer() {
    var a = window._pendingAssign;
    closeAssignModal();
    if (a) assignPathToPad(a.wavPath, a.padIndex, 'per-layer');
}

function assignPathToPad(wavPath, padIndex, mode) {
    fetch('/assign/path', {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: 'path=' + encodeURIComponent(wavPath) + '&pad=' + padIndex + '&mode=' + mode
    }).then(function(resp) {
        AudioPlayer.clearCache();
        AudioPlayer.invalidatePad(padIndex);
        window.location.reload();
    }).catch(function(err) {
        console.warn('Assign to pad failed:', err);
    });
}

// --- Sample Picker ---

var _sampleCache = null;

function openSamplePicker(layerIndex) {
    var overlay = document.createElement('div');
    overlay.id = 'sample-picker-overlay';
    overlay.className = 'file-browser-overlay';
    overlay.addEventListener('click', function(e) {
        if (e.target === overlay) closeSamplePicker();
    });

    var modal = document.createElement('div');
    modal.className = 'sample-picker-modal';
    modal.innerHTML =
        '<div class="save-confirm-header">Select Sample</div>' +
        '<div class="sample-picker-body">' +
            '<input type="text" id="sample-picker-filter" class="sample-input" placeholder="Type to filter..." autofocus>' +
            '<div id="sample-picker-list" class="sample-picker-list"></div>' +
        '</div>' +
        '<div class="save-confirm-actions">' +
            '<button class="btn-primary" onclick="clearSampleLayer()">Clear</button>' +
            '<button class="btn-primary" onclick="closeSamplePicker()">Cancel</button>' +
        '</div>';

    overlay.appendChild(modal);
    document.body.appendChild(overlay);

    window._pickerLayerIndex = layerIndex;

    var filterInput = document.getElementById('sample-picker-filter');
    filterInput.addEventListener('input', function() {
        renderSampleList(this.value);
    });
    filterInput.addEventListener('keydown', function(e) {
        if (e.key === 'Escape') closeSamplePicker();
    });
    filterInput.focus();

    // Load samples (cached after first fetch)
    if (_sampleCache) {
        renderSampleList('');
    } else {
        fetch('/api/samples')
            .then(function(r) { return r.json(); })
            .then(function(data) {
                _sampleCache = data;
                renderSampleList('');
            });
    }
}

function renderSampleList(filter) {
    var list = document.getElementById('sample-picker-list');
    if (!list || !_sampleCache) return;

    var lower = filter.toLowerCase();
    var filtered = _sampleCache.filter(function(s) {
        return !lower || s.name.toLowerCase().indexOf(lower) !== -1;
    });

    // Cap display to avoid DOM thrashing
    var max = 200;
    var html = '';
    for (var i = 0; i < filtered.length && i < max; i++) {
        var s = filtered[i];
        var displayName = s.name;
        html += '<div class="sample-picker-item" onclick="selectSample(\'' +
            escapeAttr(s.path) + '\')" title="' + escapeAttr(s.path) + '">' +
            escapeHtml(displayName) + '</div>';
    }
    if (filtered.length > max) {
        html += '<div class="sample-picker-overflow">' + (filtered.length - max) + ' more...</div>';
    }
    if (filtered.length === 0) {
        html = '<div class="sample-picker-empty">No matching samples</div>';
    }
    list.innerHTML = html;
}

function selectSample(relPath) {
    var layerIndex = window._pickerLayerIndex;
    closeSamplePicker();

    // Extract just the filename without extension for the sample name
    var parts = relPath.split('/');
    var filename = parts[parts.length - 1];
    var sampleName = filename.replace(/\.[^.]+$/, '');

    // Set the sample name via the layer update endpoint
    fetch('/pad/layer/' + layerIndex, {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: 'sample_name=' + encodeURIComponent(sampleName)
    }).then(function(r) { return r.text(); })
    .then(function(html) {
        var target = document.getElementById('pad-params');
        if (target) {
            target.innerHTML = html;
            if (typeof htmx !== 'undefined') htmx.process(target);
            if (typeof initTabs === 'function') initTabs();
        }
    });
}

function clearSampleLayer() {
    var layerIndex = window._pickerLayerIndex;
    closeSamplePicker();

    fetch('/pad/layer/' + layerIndex, {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: 'sample_name='
    }).then(function(r) { return r.text(); })
    .then(function(html) {
        var target = document.getElementById('pad-params');
        if (target) {
            target.innerHTML = html;
            if (typeof htmx !== 'undefined') htmx.process(target);
            if (typeof initTabs === 'function') initTabs();
        }
    });
}

function closeSamplePicker() {
    var overlay = document.getElementById('sample-picker-overlay');
    if (overlay) overlay.remove();
    window._pickerLayerIndex = null;
}

function escapeHtml(str) {
    var div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

function escapeAttr(str) {
    return str.replace(/'/g, "\\'").replace(/"/g, '&quot;');
}

// --- Pad Picker (Assign to Pad from WAV detail) ---

var _padPickerPrograms = null;
var _padPickerState = null; // { wavPath, pgmPath, bank }

function openPadPicker(wavPath) {
    var sampleName = wavPath.split('/').pop();

    var overlay = document.createElement('div');
    overlay.id = 'pad-picker-overlay';
    overlay.className = 'file-browser-overlay';
    overlay.addEventListener('click', function(e) {
        if (e.target === overlay) closePadPicker();
    });

    var modal = document.createElement('div');
    modal.className = 'pad-picker-modal';
    modal.innerHTML =
        '<div class="save-confirm-header">Assign ' + escapeHtml(sampleName) + ' to Pad</div>' +
        '<div class="pad-picker-body">' +
            '<div class="pad-picker-program-row">' +
                '<label>Program:</label>' +
                '<div class="pad-picker-program-select" id="pad-picker-pgm-select">' +
                    '<span class="pad-picker-pgm-name" id="pad-picker-pgm-name" onclick="toggleProgramDropdown()">Loading...</span>' +
                    '<div class="pad-picker-pgm-dropdown" id="pad-picker-pgm-dropdown" style="display:none">' +
                        '<input type="text" id="pad-picker-pgm-filter" class="sample-input" placeholder="Filter programs..." oninput="filterProgramList(this.value)">' +
                        '<div id="pad-picker-pgm-list" class="pad-picker-pgm-list"></div>' +
                    '</div>' +
                '</div>' +
            '</div>' +
            '<div class="pad-picker-banks" id="pad-picker-banks">' +
                '<span class="pad-picker-bank active" onclick="switchPickerBank(0)">A</span>' +
                '<span class="pad-picker-bank" onclick="switchPickerBank(1)">B</span>' +
                '<span class="pad-picker-bank" onclick="switchPickerBank(2)">C</span>' +
                '<span class="pad-picker-bank" onclick="switchPickerBank(3)">D</span>' +
            '</div>' +
            '<div class="pad-picker-grid" id="pad-picker-grid"></div>' +
        '</div>' +
        '<div class="save-confirm-actions">' +
            '<button class="btn-primary" onclick="closePadPicker()">Cancel</button>' +
        '</div>';

    overlay.appendChild(modal);
    document.body.appendChild(overlay);

    _padPickerState = { wavPath: wavPath, pgmPath: null, bank: 0 };

    // Load programs list, then load pads for the default program.
    fetch('/api/programs')
        .then(function(r) { return r.json(); })
        .then(function(programs) {
            _padPickerPrograms = programs;
            // Default to current session program, or first in list.
            var defaultPgm = programs.find(function(p) { return p.current; }) || programs[0];
            if (defaultPgm) {
                _padPickerState.pgmPath = defaultPgm.path;
                document.getElementById('pad-picker-pgm-name').textContent = defaultPgm.name;
                renderProgramDropdownList('');
                loadPickerPads();
            } else {
                document.getElementById('pad-picker-pgm-name').textContent = '(no programs)';
            }
        });
}

function closePadPicker() {
    var overlay = document.getElementById('pad-picker-overlay');
    if (overlay) overlay.remove();
    _padPickerState = null;
}

function toggleProgramDropdown() {
    var dd = document.getElementById('pad-picker-pgm-dropdown');
    if (!dd) return;
    var visible = dd.style.display !== 'none';
    dd.style.display = visible ? 'none' : 'block';
    if (!visible) {
        var filterInput = document.getElementById('pad-picker-pgm-filter');
        if (filterInput) {
            filterInput.value = '';
            filterInput.focus();
        }
        renderProgramDropdownList('');
    }
}

function filterProgramList(filter) {
    renderProgramDropdownList(filter);
}

function renderProgramDropdownList(filter) {
    var list = document.getElementById('pad-picker-pgm-list');
    if (!list || !_padPickerPrograms) return;

    var lower = filter.toLowerCase();
    var filtered = _padPickerPrograms.filter(function(p) {
        return !lower || p.name.toLowerCase().indexOf(lower) !== -1 || p.path.toLowerCase().indexOf(lower) !== -1;
    });

    var html = '';
    for (var i = 0; i < filtered.length; i++) {
        var p = filtered[i];
        var cls = (_padPickerState && p.path === _padPickerState.pgmPath) ? ' active' : '';
        html += '<div class="pad-picker-pgm-item' + cls + '" onclick="selectPickerProgram(\'' +
            escapeAttr(p.path) + '\', \'' + escapeAttr(p.name) + '\')">' +
            escapeHtml(p.path) + '</div>';
    }
    if (filtered.length === 0) {
        html = '<div class="pad-picker-pgm-item" style="color:#888;font-style:italic">No matching programs</div>';
    }
    list.innerHTML = html;
}

function selectPickerProgram(path, name) {
    _padPickerState.pgmPath = path;
    _padPickerState.bank = 0;
    document.getElementById('pad-picker-pgm-name').textContent = name;
    document.getElementById('pad-picker-pgm-dropdown').style.display = 'none';
    // Reset bank tabs.
    var tabs = document.querySelectorAll('.pad-picker-bank');
    tabs.forEach(function(t, i) { t.classList.toggle('active', i === 0); });
    loadPickerPads();
}

function switchPickerBank(bank) {
    _padPickerState.bank = bank;
    var tabs = document.querySelectorAll('.pad-picker-bank');
    tabs.forEach(function(t, i) { t.classList.toggle('active', i === bank); });
    loadPickerPads();
}

function loadPickerPads() {
    var st = _padPickerState;
    if (!st || !st.pgmPath) return;

    var grid = document.getElementById('pad-picker-grid');
    if (!grid) return;
    grid.innerHTML = '<div style="color:#888;padding:8px">Loading...</div>';

    fetch('/api/program-pads?path=' + encodeURIComponent(st.pgmPath) + '&bank=' + st.bank)
        .then(function(r) { return r.json(); })
        .then(function(pads) {
            var html = '';
            for (var i = 0; i < pads.length; i++) {
                var p = pads[i];
                var cls = 'pad-picker-btn';
                if (p.layers > 0) cls += ' has-sample';
                var title = p.name || '(empty)';
                if (p.layers > 1) title += ' (' + p.layers + ' layers)';
                html += '<button class="' + cls + '" onclick="pickPad(' + p.index + ')" title="' +
                    escapeAttr(title) + '">' +
                    '<span class="pad-number">' + p.display + '</span>';
                if (p.name) {
                    html += '<span class="pad-name">' + escapeHtml(p.name) + '</span>';
                }
                if (p.layers > 1) {
                    html += '<span class="pad-layers">' + p.layers + 'L</span>';
                }
                html += '</button>';
            }
            grid.innerHTML = html;
        })
        .catch(function(err) {
            grid.innerHTML = '<div style="color:#f88;padding:8px">Error loading pads</div>';
        });
}

function pickPad(padIndex) {
    var st = _padPickerState;
    if (!st) return;

    fetch('/api/assign-to-program', {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: 'pgm_path=' + encodeURIComponent(st.pgmPath) +
              '&wav_path=' + encodeURIComponent(st.wavPath) +
              '&pad=' + padIndex
    }).then(function(r) { return r.json(); })
    .then(function(data) {
        closePadPicker();
        AudioPlayer.clearCache();
        // If assigning to the session's current program, reload to reflect changes.
        var pgmDetail = document.querySelector('.detail-pgm');
        if (pgmDetail) {
            window.location.reload();
        }
    })
    .catch(function(err) {
        console.warn('Assign to program failed:', err);
    });
}
