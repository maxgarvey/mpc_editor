// --- WAV-to-Pad Assignment ---

(function() {
    var _sampleCache = null;
    var _pickerLayerIndex = null;
    var _padPickerPrograms = null;
    var _padPickerState = null; // { wavPath, pgmPath, bank }
    var _pendingAssign = null;

    document.body.addEventListener('invalidateSampleCache', function() { _sampleCache = null; });

    // Assign a WAV file to the currently selected pad in the active PGM editor.
    // Called from the "Assign to Pad" button in the WAV detail view.
    function assignWavToPad(wavPath) {
        var selectedBtn = document.querySelector('.pad-btn.selected');
        var padIndex = 0;
        var padLabel = 'A1';
        if (selectedBtn) {
            padIndex = parseInt(selectedBtn.getAttribute('data-pad-index') || '0');
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

        _pendingAssign = { wavPath: wavPath, padIndex: padIndex };
    }

    function closeAssignModal() {
        var overlay = document.getElementById('assign-overlay');
        if (overlay) overlay.remove();
        _pendingAssign = null;
    }

    function assignReplace() {
        var a = _pendingAssign;
        closeAssignModal();
        if (a) assignPathToPad(a.wavPath, a.padIndex, 'replace');
    }

    function assignAddLayer() {
        var a = _pendingAssign;
        closeAssignModal();
        if (a) assignPathToPad(a.wavPath, a.padIndex, 'per-layer');
    }

    function assignPathToPad(wavPath, padIndex, mode) {
        console.log('[assign] POST /assign/path — wavPath:', wavPath, 'pad:', padIndex, 'mode:', mode);
        fetch('/assign/path', {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: 'path=' + encodeURIComponent(wavPath) + '&pad=' + padIndex + '&mode=' + mode
        }).then(function(r) {
            console.log('[assign] response status:', r.status);
            if (!r.ok) {
                return r.text().then(function(body) {
                    console.warn('[assign] server error:', r.status, body);
                });
            }
            AudioPlayer.clearCache();
            AudioPlayer.invalidatePad(padIndex);
            refreshPadGridAndParams(padIndex);
        }).catch(function(err) {
            console.warn('[assign] Assign to pad failed:', err);
        });
    }

    // --- Pad Grid / Params Refresh ---

    function refreshPadGridAndParams(padIndex) {
        var bank = Math.floor(padIndex / 16);
        var padGrid = document.querySelector('.pad-grid');
        var padParams = document.getElementById('pad-params');

        if (padGrid) {
            fetch('/partials/pad-grid?bank=' + bank)
                .then(function(r) { return r.text(); })
                .then(function(html) {
                    padGrid.outerHTML = html;
                    var newGrid = document.querySelector('.pad-grid');
                    if (newGrid && typeof htmx !== 'undefined') htmx.process(newGrid);
                    if (typeof initDragDrop === 'function') initDragDrop();
                    // Highlight the assigned pad.
                    var btns = document.querySelectorAll('.pad-btn');
                    btns.forEach(function(b) {
                        b.classList.toggle('selected', parseInt(b.getAttribute('data-pad-index') || '-1') === padIndex);
                    });
                });
        }

        if (padParams) {
            fetch('/partials/pad-params')
                .then(function(r) { return r.text(); })
                .then(function(html) {
                    padParams.innerHTML = html;
                    if (typeof htmx !== 'undefined') htmx.process(padParams);
                    if (typeof initTabs === 'function') initTabs();
                });
        }
    }

    // --- Sample Picker ---

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

        _pickerLayerIndex = layerIndex;

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
        var layerIndex = _pickerLayerIndex;
        closeSamplePicker();

        // Extract just the filename without extension for the sample name.
        // Truncate to 16 chars to match the MPC's limit.
        var parts = relPath.split('/');
        var filename = parts[parts.length - 1];
        var sampleName = filename.replace(/\.[^.]+$/, '');
        if (sampleName.length > 16) {
            sampleName = sampleName.substring(0, 16);
        }

        // Set the sample name via the layer update endpoint
        AudioPlayer.clearCache();
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
            // Refresh pad grid to show new sample assignment.
            var padGrid = document.querySelector('.pad-grid');
            if (padGrid) {
                var activeBank = document.querySelector('.bank-tab.active');
                var bank = activeBank ? activeBank.textContent.trim().replace('Bank ', '').charCodeAt(0) - 65 : 0;
                fetch('/partials/pad-grid?bank=' + bank)
                    .then(function(r) { return r.text(); })
                    .then(function(gridHtml) {
                        padGrid.outerHTML = gridHtml;
                        var newGrid = document.querySelector('.pad-grid');
                        if (newGrid && typeof htmx !== 'undefined') htmx.process(newGrid);
                        if (typeof initDragDrop === 'function') initDragDrop();
                    });
            }
        });
    }

    function clearSampleLayer() {
        var layerIndex = _pickerLayerIndex;
        closeSamplePicker();
        clearSampleDirect(layerIndex);
    }

    function clearSampleDirect(layerIndex) {
        AudioPlayer.clearCache();
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
            // Refresh pad grid to reflect removed sample.
            var padGrid = document.querySelector('.pad-grid');
            if (padGrid) {
                var activeBank = document.querySelector('.bank-tab.active');
                var bank = activeBank ? activeBank.textContent.trim().replace('Bank ', '').charCodeAt(0) - 65 : 0;
                fetch('/partials/pad-grid?bank=' + bank)
                    .then(function(r) { return r.text(); })
                    .then(function(gridHtml) {
                        padGrid.outerHTML = gridHtml;
                        var newGrid = document.querySelector('.pad-grid');
                        if (newGrid && typeof htmx !== 'undefined') htmx.process(newGrid);
                        if (typeof initDragDrop === 'function') initDragDrop();
                    });
            }
        });
    }

    function closeSamplePicker() {
        var overlay = document.getElementById('sample-picker-overlay');
        if (overlay) overlay.remove();
        _pickerLayerIndex = null;
    }

    // --- Pad Picker (Assign to Pad from WAV detail) ---

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
                // Reorder to match MPC 1000 physical layout: pads 13-16 top, 1-4 bottom.
                for (var uiPos = 0; uiPos < 16; uiPos++) {
                    var srcIdx = (3 - Math.floor(uiPos / 4)) * 4 + (uiPos % 4);
                    var p = pads[srcIdx];
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
            .catch(function() {
                grid.innerHTML = '<div style="color:#f88;padding:8px">Error loading pads</div>';
            });
    }

    function pickPad(padIndex) {
        var st = _padPickerState;
        if (!st) return;

        var pgmPath = st.pgmPath;

        fetch('/api/assign-to-program', {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: 'pgm_path=' + encodeURIComponent(pgmPath) +
                  '&wav_path=' + encodeURIComponent(st.wavPath) +
                  '&pad=' + padIndex
        }).then(function(r) { return r.json(); })
        .then(function(data) {
            closePadPicker();
            AudioPlayer.clearCache();
            // Use the absolute path from the response so TabManager.openFile
            // matches any existing tab opened via the file browser (which stores
            // absolute paths) rather than opening a duplicate tab.
            TabManager.openFile(data.pgm_abs || pgmPath);
        })
        .catch(function(err) {
            console.warn('Assign to program failed:', err);
        });
    }

    // Expose public API (called from onclick attributes and other JS modules)
    window.assignWavToPad = assignWavToPad;
    window.openAssignModal = openAssignModal;
    window.closeAssignModal = closeAssignModal;
    window.assignReplace = assignReplace;
    window.assignAddLayer = assignAddLayer;
    window.assignPathToPad = assignPathToPad;
    window.refreshPadGridAndParams = refreshPadGridAndParams;
    window.openSamplePicker = openSamplePicker;
    window.selectSample = selectSample;
    window.clearSampleLayer = clearSampleLayer;
    window.clearSampleDirect = clearSampleDirect;
    window.closeSamplePicker = closeSamplePicker;
    window.openPadPicker = openPadPicker;
    window.closePadPicker = closePadPicker;
    window.toggleProgramDropdown = toggleProgramDropdown;
    window.filterProgramList = filterProgramList;
    window.selectPickerProgram = selectPickerProgram;
    window.switchPickerBank = switchPickerBank;
    window.pickPad = pickPad;
})();
