// MPC Editor - Core JavaScript

// HX-Trigger event names — must match the Go constants in internal/server/server.go.
var HX_CLEAR_AUDIO_CACHE = 'clearAudioCache';
var HX_INVALIDATE_PAD    = 'invalidatePad';

// --- Shared Utilities ---

function escapeHtml(s) {
    var div = document.createElement('div');
    div.appendChild(document.createTextNode(s));
    return div.innerHTML;
}

function escapeAttr(str) {
    return str.replace(/'/g, "\\'").replace(/"/g, '&quot;');
}

function formatBytes(bytes) {
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KB';
    return (bytes / 1048576).toFixed(1) + ' MB';
}

// --- Slider Display ---

// Update range slider value displays
document.addEventListener('input', function(e) {
    if (e.target.classList.contains('slider-input')) {
        const display = e.target.parentElement.querySelector('.value-display');
        if (display) {
            display.textContent = e.target.value;
        }
    }
});

// --- Param Tabs ---

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

// --- Keyboard Shortcuts ---

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

// --- Detail Panel Init ---

// Initialize any detail-panel content that requires JS setup.
// Called from both htmx:afterSettle (HTMX swaps) and tabs.js (fetch-based tab activations).
function initDetailContent(el) {
    if (!el || !el.querySelector) return;
    // Always clean up the WAV detail player when the detail panel content changes,
    // so audio stops and DOM refs are cleared even when switching to a non-WAV tab.
    if (typeof WavDetailPlayer !== 'undefined') WavDetailPlayer.cleanup();
    var wavPanel = el.querySelector('.detail-wav[data-rel-path]');
    if (wavPanel) {
        var relPath = wavPanel.getAttribute('data-rel-path');
        var audioUrl = '/audio/file?path=' + encodeURIComponent(relPath);
        if (typeof WavWaveform !== 'undefined') WavWaveform.load(relPath);
        if (typeof WavDetailPlayer !== 'undefined') WavDetailPlayer.init(audioUrl, relPath);
    }
    var waveformCanvas = el.querySelector('#waveform-canvas');
    if (waveformCanvas && typeof Waveform !== 'undefined') Waveform.load();
    var pgmPanel = el.querySelector('.detail-pgm[data-file-path]');
    if (pgmPanel) {
        var pgmPath = pgmPanel.getAttribute('data-file-path');
        var pathEl = document.getElementById('save-pgm-path');
        if (pathEl && pgmPath) pathEl.value = pgmPath;
    }
}

// Re-initialize UI components when HTMX swaps content
document.addEventListener('htmx:afterSettle', function(e) {
    initTabs();
    initDetailContent(e.detail && e.detail.target ? e.detail.target : document.body);
});

// Audio cache invalidation via server-sent HX-Trigger events
document.body.addEventListener(HX_CLEAR_AUDIO_CACHE, function() { AudioPlayer.clearCache(); });
document.body.addEventListener(HX_INVALIDATE_PAD, function(e) { AudioPlayer.invalidatePad(e.detail.value); });

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
                escapeHtml(path) + '" style="width:100%">' +
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

    htmx.ajax('GET', '/settings', { target: '#settings-content' });
}

function closeSettingsModal() {
    var overlay = document.getElementById('settings-overlay');
    if (overlay) overlay.remove();
}

function saveSettings() {
    var workspace = document.getElementById('settings-workspace');
    var profile = document.getElementById('settings-profile');

    var values = {};
    if (workspace) values.workspace = workspace.value;
    if (profile) values.profile = profile.value;

    htmx.ajax('POST', '/settings/save', { values: values });
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

// --- WAV Preview in Browser Nav ---

var _previewBtn = null;
var _previewAudio = null;

function _clearBrowserPreviewState() {
    if (_previewBtn) {
        _previewBtn.classList.remove('playing');
        _previewBtn.innerHTML = '&#9654;';
        _previewBtn = null;
    }
    _previewAudio = null;
}

// Stop browser preview whenever AudioPlayer takes over playback.
AudioPlayer.onStopAll(function() {
    if (_previewAudio) {
        _previewAudio.pause();
        _clearBrowserPreviewState();
    }
});

function previewWavInBrowser(absPath, btn) {
    // Toggle off if already playing this button.
    if (_previewBtn === btn) {
        if (_previewAudio) _previewAudio.pause();
        _clearBrowserPreviewState();
        return;
    }
    // Stop any existing preview.
    if (_previewAudio) _previewAudio.pause();
    _clearBrowserPreviewState();

    _previewBtn = btn;
    btn.classList.add('playing');
    btn.innerHTML = '&#9646;&#9646;';

    var audio = new Audio('/audio/file?path=' + encodeURIComponent(absPath));
    _previewAudio = audio;
    audio.addEventListener('ended', function() {
        if (_previewAudio === audio) _clearBrowserPreviewState();
    });
    audio.addEventListener('error', function() {
        if (_previewAudio === audio) _clearBrowserPreviewState();
    });
    audio.play().catch(function() {
        if (_previewAudio === audio) _clearBrowserPreviewState();
    });
}

// --- Browser Refresh (triggered by server via HX-Trigger) ---

// Refresh the file nav, re-running any active search query instead of resetting to directory view.
function refreshBrowserNav(dir) {
    var searchInput = document.getElementById('workspace-search');
    var query = searchInput ? searchInput.value.trim() : '';
    if (query) {
        htmx.ajax('GET', '/browse/search?q=' + encodeURIComponent(query), { target: '#file-nav' });
    } else {
        var url = '/browse/nav' + (dir ? '?dir=' + encodeURIComponent(dir) : '');
        htmx.ajax('GET', url, { target: '#file-nav' });
    }
}

document.addEventListener('refreshBrowser', function() {
    refreshBrowserNav();
});

// --- Browser Nav Highlighting ---

// After browser nav re-renders (directory navigation, mkdir), re-apply tab highlighting.
// Also stop any active WAV preview since the nav buttons were replaced and _previewBtn
// would otherwise be a stale reference to a detached element, breaking toggle-off.
document.addEventListener('htmx:afterSettle', function(e) {
    if (e.detail.target && e.detail.target.id === 'file-nav') {
        TabManager.refreshBrowserHighlight();
        if (_previewAudio) _previewAudio.pause();
        _clearBrowserPreviewState();
    }
    if (e.detail.target && e.detail.target.id === 'move-dirs-container') {
        attachMoveDirClickSelection(e.detail.target);
    }
});

// --- Workspace Panel ---

const WorkspacePanel = (function() {
    var collapsed = false;

    function apply() {
        var layout = document.getElementById('browser-layout');
        var btn = document.getElementById('panel-collapse-btn');
        if (!layout) return;
        if (collapsed) {
            layout.classList.add('panel-collapsed');
            if (btn) { btn.innerHTML = '&#x25BA;'; btn.title = 'Expand panel'; }
        } else {
            layout.classList.remove('panel-collapsed');
            if (btn) { btn.innerHTML = '&#x25C4;'; btn.title = 'Collapse panel'; }
        }
    }

    function toggle() {
        collapsed = !collapsed;
        localStorage.setItem('workspace-panel-collapsed', collapsed ? '1' : '0');
        apply();
    }

    document.addEventListener('DOMContentLoaded', function() {
        collapsed = localStorage.getItem('workspace-panel-collapsed') === '1';
        apply();
    });

    return { toggle: toggle };
})();

const LabelPicker = (() => {
    function toggleCategory(cat, btn) {
        const picker = btn.closest('#wav-label-picker');
        if (!picker) return;
        const allSubcatRows = picker.querySelectorAll('.label-subcat-row');
        const allCatBtns = picker.querySelectorAll('.label-cat-btn');
        const targetRow = picker.querySelector('#label-subcats-' + cat);
        const isOpen = targetRow && targetRow.classList.contains('visible');

        allSubcatRows.forEach(r => r.classList.remove('visible'));
        allCatBtns.forEach(b => b.classList.remove('active'));

        if (!isOpen && targetRow) {
            targetRow.classList.add('visible');
            btn.classList.add('active');
        }
    }

    return { toggleCategory };
})();
