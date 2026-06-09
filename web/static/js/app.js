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

// --- Toast Notifications ---

// showToast displays a transient message bottom-right. type: '' | 'error'.
function showToast(message, type) {
    var container = document.getElementById('toast-container');
    if (!container) {
        container = document.createElement('div');
        container.id = 'toast-container';
        document.body.appendChild(container);
    }
    var toast = document.createElement('div');
    toast.className = 'toast' + (type ? ' toast-' + type : '');
    toast.textContent = message;
    container.appendChild(toast);
    setTimeout(function() {
        toast.classList.add('toast-hide');
        setTimeout(function() { toast.remove(); }, 350);
    }, 4000);
}

// Surface failed HTMX requests instead of failing silently.
document.addEventListener('htmx:responseError', function(e) {
    var xhr = e.detail.xhr;
    var msg = xhr && xhr.responseText ? xhr.responseText.trim() : '';
    // Plain-text handler errors are short; ignore HTML error pages.
    if (msg.length > 200 || msg.charAt(0) === '<') msg = '';
    showToast(msg || ('Request failed (' + (xhr ? xhr.status : 'no response') + ')'), 'error');
});
document.addEventListener('htmx:sendError', function() {
    showToast('Server unreachable', 'error');
});

// --- Save Indicator ---

// Subtle "Saved" pulse fired by the server (HX-Trigger: programSaved) after
// the program file is written following a pad/layer edit.
document.addEventListener('programSaved', function() {
    var el = document.getElementById('save-indicator');
    if (!el) {
        el = document.createElement('div');
        el.id = 'save-indicator';
        el.textContent = '✓ Saved';
        document.body.appendChild(el);
    }
    el.classList.remove('save-indicator-show');
    void el.offsetWidth; // restart the CSS animation
    el.classList.add('save-indicator-show');
});

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

// --- Search + Filter Chips ---

const SearchChips = (function() {
    var activeChips = new Set();
    var favsActive = false;
    var debounceTimer = null;
    var options = []; // [{label, css}] loaded from DOM on page init

    function loadOptions() {
        document.querySelectorAll('#filter-chip-option-data span').forEach(function(el) {
            options.push({label: el.dataset.label || '', css: el.dataset.css || ''});
        });
    }

    function buildSearchUrl() {
        var input = document.getElementById('workspace-search');
        var q = input ? input.value.trim() : '';
        if (!q && activeChips.size === 0 && !favsActive) return null;
        var params = new URLSearchParams();
        if (q) params.set('q', q);
        activeChips.forEach(function(c) { params.append('chips', c); });
        if (favsActive) params.set('favorites', '1');
        return '/browse/search?' + params.toString();
    }

    function runSearch(dir) {
        var url = buildSearchUrl();
        if (!url) {
            var sortMode = typeof BrowseSort !== 'undefined' ? BrowseSort.getMode() : 'name';
            var navUrl = '/browse/nav?sort=' + encodeURIComponent(sortMode) + (dir ? '&dir=' + encodeURIComponent(dir) : '');
            htmx.ajax('GET', navUrl, {target: '#file-nav'});
            return;
        }
        htmx.ajax('GET', url, {target: '#file-nav'});
    }

    function debounce() {
        clearTimeout(debounceTimer);
        debounceTimer = setTimeout(runSearch, 250);
    }

    function renderChips() {
        var bar = document.getElementById('filter-chips');
        if (!bar) return;
        var input = document.getElementById('workspace-search');
        var q = input ? input.value.trim() : '';
        var isActive = q || activeChips.size > 0 || favsActive;
        var html = '';
        html += '<button class="filter-chip filter-chip-all' + (isActive ? '' : ' active') + '" onclick="SearchChips.clearToAll()">All</button>';
        html += '<button class="filter-chip filter-chip-fav' + (favsActive ? ' active' : '') + '" onclick="SearchChips.toggleFav()">★</button>';
        activeChips.forEach(function(label) {
            var opt = options.find(function(o) { return o.label === label; }) || {};
            html += '<button class="filter-chip filter-chip-active active" onclick="SearchChips.removeChip(\'' + escapeAttr(label) + '\')">';
            if (opt.css) html += '<span class="filter-chip-dot" style="background:' + opt.css + '"></span>';
            html += escapeHtml(label) + ' <span class="chip-remove">×</span></button>';
        });
        html += '<button class="filter-chip filter-chip-add" id="chip-add-btn" onclick="SearchChips.toggleDropdown(event)">+ Add Filter</button>';
        bar.innerHTML = html;
    }

    function addChip(label) {
        activeChips.add(label);
        closeDropdown();
        renderChips();
        runSearch();
    }

    function removeChip(label) {
        activeChips.delete(label);
        renderChips();
        runSearch();
    }

    function toggleFav() {
        favsActive = !favsActive;
        renderChips();
        runSearch();
    }

    function clearToAll() {
        activeChips.clear();
        favsActive = false;
        var search = document.getElementById('workspace-search');
        if (search) search.value = '';
        renderChips();
        runSearch();
    }

    // resetState clears filter state (called when explicitly navigating to directory view).
    function resetState() {
        activeChips.clear();
        favsActive = false;
        var search = document.getElementById('workspace-search');
        if (search) search.value = '';
        closeDropdown();
        renderChips();
    }

    function toggleDropdown(e) {
        if (e) e.stopPropagation();
        var dd = document.getElementById('filter-chip-dropdown');
        if (!dd) return;
        if (!dd.hidden) { dd.hidden = true; return; }
        var btn = document.getElementById('chip-add-btn');
        if (btn) {
            var rect = btn.getBoundingClientRect();
            dd.style.left = rect.left + 'px';
            dd.style.top = (rect.bottom + 4) + 'px';
        }
        dd.hidden = false;
        renderDropdownList('');
        var ds = document.getElementById('chip-dropdown-search');
        if (ds) { ds.value = ''; ds.focus(); }
    }

    function closeDropdown() {
        var dd = document.getElementById('filter-chip-dropdown');
        if (dd) dd.hidden = true;
    }

    function renderDropdownList(filter) {
        var list = document.getElementById('chip-dropdown-list');
        if (!list) return;
        var html = '';
        var shown = 0;
        options.forEach(function(opt) {
            if (activeChips.has(opt.label)) return;
            if (filter && opt.label.toLowerCase().indexOf(filter.toLowerCase()) === -1) return;
            html += '<div class="chip-dropdown-item" onclick="SearchChips.addChip(\'' + escapeAttr(opt.label) + '\')">';
            if (opt.css) html += '<span class="filter-chip-dot" style="background:' + opt.css + '"></span>';
            html += escapeHtml(opt.label) + '</div>';
            shown++;
        });
        if (!shown) html = '<div class="chip-dropdown-empty">No options</div>';
        list.innerHTML = html;
    }

    document.addEventListener('DOMContentLoaded', function() {
        loadOptions();
        var search = document.getElementById('workspace-search');
        if (search) search.addEventListener('input', function() { renderChips(); debounce(); });
        var ds = document.getElementById('chip-dropdown-search');
        if (ds) ds.addEventListener('input', function() { renderDropdownList(ds.value); });
        renderChips();
    });

    // Close dropdown on outside click.
    document.addEventListener('click', function(e) {
        var dd = document.getElementById('filter-chip-dropdown');
        if (dd && !dd.hidden && !dd.contains(e.target)) {
            var btn = document.getElementById('chip-add-btn');
            if (!btn || !btn.contains(e.target)) dd.hidden = true;
        }
    });

    return {
        addChip: addChip,
        removeChip: removeChip,
        toggleFav: toggleFav,
        clearToAll: clearToAll,
        resetState: resetState,
        toggleDropdown: toggleDropdown,
        debounce: debounce,
        runSearch: runSearch
    };
})();

// --- Browser Refresh (triggered by server via HX-Trigger) ---

function refreshBrowserNav(dir) {
    SearchChips.runSearch(dir);
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
        BrowseGroups.restore();
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

    function headerClick() {
        if (collapsed) toggle();
    }

    return { toggle: toggle, headerClick: headerClick };
})();

// --- Browse Groups (collapsible type sections in directory view) ---

const BrowseGroups = (function() {
    var PREFIX = 'browse-group-';

    function isCollapsed(key) {
        return localStorage.getItem(PREFIX + key) === '1';
    }

    function applyGroup(key, collapsed) {
        document.querySelectorAll('#file-nav .browser-entry[data-group="' + key + '"]').forEach(function(e) {
            e.style.display = collapsed ? 'none' : '';
        });
        var divider = document.querySelector('#file-nav .nav-divider[data-group-key="' + key + '"]');
        if (divider) divider.classList.toggle('is-collapsed', collapsed);
    }

    function toggle(key) {
        var nowCollapsed = !isCollapsed(key);
        localStorage.setItem(PREFIX + key, nowCollapsed ? '1' : '0');
        applyGroup(key, nowCollapsed);
    }

    function restore() {
        document.querySelectorAll('#file-nav .nav-divider[data-group-key]').forEach(function(d) {
            var key = d.dataset.groupKey;
            if (key) applyGroup(key, isCollapsed(key));
        });
    }

    return { toggle: toggle, restore: restore };
})();

// --- Browse Sort ---

const BrowseSort = (function() {
    var mode = 'name';

    function apply() {
        var btns = document.querySelectorAll('#sort-toggle .sort-btn');
        btns.forEach(function(b) {
            b.classList.toggle('active', b.dataset.sort === mode);
        });
    }

    function set(newMode, btn) {
        mode = newMode;
        localStorage.setItem('browse-sort-mode', mode);
        apply();
        htmx.ajax('GET', '/browse/nav?sort=' + encodeURIComponent(mode), { target: '#file-nav' });
    }

    function getMode() { return mode; }

    document.addEventListener('DOMContentLoaded', function() {
        mode = localStorage.getItem('browse-sort-mode') || 'name';
        apply();
    });

    // On any /browse/nav request (breadcrumbs, sort change): inject sort param and clear chip state.
    document.body.addEventListener('htmx:configRequest', function(e) {
        var path = e.detail.path || '';
        if (path.startsWith('/browse/nav')) {
            if (!e.detail.parameters.sort) e.detail.parameters.sort = mode;
            if (typeof SearchChips !== 'undefined') SearchChips.resetState();
        }
    });

    return { set: set, getMode: getMode };
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
