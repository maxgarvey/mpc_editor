// --- Device Transfer Modal ---

(function() {
    var _deviceState = null;

    function openDeviceModal() {
        _deviceState = {
            tab: 'to_mpc',
            to_mpc:   { srcDir: '', destDir: '', selected: {} },
            from_mpc: { srcDir: '', destDir: '', selected: {} }
        };

        var overlay = document.createElement('div');
        overlay.id = 'device-modal-overlay';
        overlay.className = 'file-browser-overlay';
        overlay.addEventListener('click', function(e) {
            if (e.target === overlay) closeDeviceModal();
        });

        var modal = document.createElement('div');
        modal.className = 'device-modal';
        modal.innerHTML =
            '<div class="save-confirm-header">' +
                '<span>Transfer Files</span>' +
                '<button class="new-modal-close" onclick="closeDeviceModal()">&times;</button>' +
            '</div>' +
            '<div class="device-modal-tabs">' +
                '<button class="device-tab active" onclick="switchDeviceTab(\'to_mpc\')">Workspace &rarr; MPC</button>' +
                '<button class="device-tab" onclick="switchDeviceTab(\'from_mpc\')">MPC &rarr; Workspace</button>' +
            '</div>' +
            '<div class="device-modal-panels">' +
                '<div class="device-panel">' +
                    '<div class="device-panel-header" id="device-src-title">Source</div>' +
                    '<div class="device-breadcrumbs" id="device-src-crumbs"></div>' +
                    '<div class="device-file-list" id="device-src-list"></div>' +
                '</div>' +
                '<div class="device-panel device-dest-panel">' +
                    '<div class="device-panel-header" id="device-dest-title">Destination</div>' +
                    '<div class="device-breadcrumbs" id="device-dest-crumbs"></div>' +
                    '<div class="device-file-list" id="device-dest-list"></div>' +
                    '<div class="device-dest-path-row">' +
                        '<span>Dest:</span> <span class="device-dest-current" id="device-dest-display">/</span>' +
                        '<button class="btn-sm" onclick="openDeviceMkdir()" style="margin-left:auto">New Folder</button>' +
                    '</div>' +
                '</div>' +
            '</div>' +
            '<div class="device-modal-footer">' +
                '<span id="device-sel-count" style="color:#888;font-size:12px">0 items selected</span>' +
                '<span id="device-result-msg" style="font-size:12px"></span>' +
                '<button class="btn-primary" id="device-xfer-btn" onclick="executeDeviceTransfer()" disabled>Copy &rarr;</button>' +
            '</div>';

        overlay.appendChild(modal);
        document.body.appendChild(overlay);

        _deviceLoadSource('');
        _deviceLoadDest('');
        _deviceUpdateTitles();
    }

    function closeDeviceModal() {
        var el = document.getElementById('device-modal-overlay');
        if (el) el.remove();
        _deviceState = null;
    }

    function switchDeviceTab(tab) {
        if (!_deviceState) return;
        _deviceState.tab = tab;
        document.querySelectorAll('.device-tab').forEach(function(t) {
            t.classList.toggle('active', t.textContent.trim().startsWith(tab === 'to_mpc' ? 'Workspace' : 'MPC'));
        });
        _deviceUpdateTitles();
        _deviceUpdateFooter();
        _deviceLoadSource(_deviceState[tab].srcDir);
        _deviceLoadDest(_deviceState[tab].destDir);
    }

    function _deviceUpdateTitles() {
        var st = document.getElementById('device-src-title');
        var dt = document.getElementById('device-dest-title');
        if (!st || !dt || !_deviceState) return;
        if (_deviceState.tab === 'to_mpc') {
            st.textContent = 'Workspace (source)';
            dt.textContent = 'MPC (destination)';
        } else {
            st.textContent = 'MPC (source)';
            dt.textContent = 'Workspace (destination)';
        }
    }

    function _deviceLoadSource(dir) {
        if (!_deviceState) return;
        var state = _deviceState[_deviceState.tab];
        state.srcDir = dir;
        var root = _deviceState.tab === 'to_mpc' ? 'workspace' : 'mpc';
        var list = document.getElementById('device-src-list');
        var crumbs = document.getElementById('device-src-crumbs');
        if (!list) return;
        list.innerHTML = '<div style="color:#888;padding:8px">Loading...</div>';
        _deviceRenderCrumbs(crumbs, dir, 'src');
        fetch('/device/ls?root=' + root + '&dir=' + encodeURIComponent(dir))
            .then(function(r) {
                if (!r.ok) return r.text().then(function(t) { throw new Error(t); });
                return r.json();
            })
            .then(function(entries) {
                var state = _deviceState && _deviceState[_deviceState.tab];
                if (!state) return;
                if (!entries || entries.length === 0) {
                    list.innerHTML = '<div style="color:#888;padding:8px;font-style:italic">Empty</div>';
                    return;
                }
                var html = '';
                entries.forEach(function(e) {
                    var checked = !!state.selected[e.rel];
                    var size = e.is_dir ? '' : ' <span style="color:#666;font-size:10px">(' + _fmtBytes(e.size) + ')</span>';
                    html += '<div class="device-file-item"><label>' +
                        '<input type="checkbox"' + (checked ? ' checked' : '') +
                            ' onchange="toggleDeviceSel(\'' + escapeAttr(e.rel) + '\',this.checked)">' +
                        (e.is_dir
                            ? '<span class="device-item-dir" onclick="event.preventDefault();_deviceLoadSource(\'' + escapeAttr(e.rel) + '\')">' + escapeHtml(e.name) + '/</span>'
                            : '<span>' + escapeHtml(e.name) + size + '</span>') +
                        '</label></div>';
                });
                list.innerHTML = html;
                _deviceUpdateFooter();
            })
            .catch(function(err) {
                list.innerHTML = '<div style="color:#f88;padding:8px">' + escapeHtml(err.message) + '</div>';
            });
    }

    function _deviceLoadDest(dir) {
        if (!_deviceState) return;
        var state = _deviceState[_deviceState.tab];
        state.destDir = dir;
        var root = _deviceState.tab === 'to_mpc' ? 'mpc' : 'workspace';
        var list = document.getElementById('device-dest-list');
        var crumbs = document.getElementById('device-dest-crumbs');
        var display = document.getElementById('device-dest-display');
        if (!list) return;
        list.innerHTML = '<div style="color:#888;padding:8px">Loading...</div>';
        _deviceRenderCrumbs(crumbs, dir, 'dest');
        if (display) display.textContent = '/' + dir;
        fetch('/device/ls?root=' + root + '&dir=' + encodeURIComponent(dir))
            .then(function(r) {
                if (!r.ok) return r.text().then(function(t) { throw new Error(t); });
                return r.json();
            })
            .then(function(entries) {
                var state = _deviceState && _deviceState[_deviceState.tab];
                if (!state) return;
                var dirs = (entries || []).filter(function(e) { return e.is_dir; });
                if (dirs.length === 0) {
                    list.innerHTML = '<div style="color:#888;padding:8px;font-style:italic">No subdirectories</div>';
                    return;
                }
                var html = '';
                dirs.forEach(function(e) {
                    var active = state.destDir === e.rel;
                    html += '<div class="device-dest-item' + (active ? ' active' : '') +
                        '" onclick="_deviceLoadDest(\'' + escapeAttr(e.rel) + '\')">' +
                        escapeHtml(e.name) + '/' +
                        '</div>';
                });
                list.innerHTML = html;
            })
            .catch(function(err) {
                list.innerHTML = '<div style="color:#f88;padding:8px">' + escapeHtml(err.message) + '</div>';
            });
    }

    function _deviceRenderCrumbs(el, dir, side) {
        if (!el) return;
        var parts = dir ? dir.split('/').filter(Boolean) : [];
        var fn = side === 'src' ? '_deviceLoadSource' : '_deviceLoadDest';
        var html = '<span class="device-crumb" onclick="' + fn + '(\'\')">Root</span>';
        var accum = '';
        parts.forEach(function(p) {
            accum = accum ? accum + '/' + p : p;
            var path = accum;
            html += '<span class="device-crumb-sep">/</span>' +
                '<span class="device-crumb" onclick="' + fn + '(\'' + escapeAttr(path) + '\')">' +
                escapeHtml(p) + '</span>';
        });
        el.innerHTML = html;
    }

    function toggleDeviceSel(relPath, checked) {
        if (!_deviceState) return;
        var state = _deviceState[_deviceState.tab];
        if (checked) { state.selected[relPath] = true; } else { delete state.selected[relPath]; }
        _deviceUpdateFooter();
    }

    function _deviceUpdateFooter() {
        if (!_deviceState) return;
        var state = _deviceState[_deviceState.tab];
        var count = Object.keys(state.selected).length;
        var countEl = document.getElementById('device-sel-count');
        var btn = document.getElementById('device-xfer-btn');
        if (countEl) countEl.textContent = count + (count === 1 ? ' item' : ' items') + ' selected';
        if (btn) {
            btn.disabled = count === 0;
            btn.textContent = _deviceState.tab === 'to_mpc' ? 'Copy to MPC →' : '← Copy to Workspace';
        }
    }

    function openDeviceMkdir() {
        if (!_deviceState) return;
        var state = _deviceState[_deviceState.tab];
        var name = prompt('New folder name:');
        if (!name || !name.trim()) return;
        name = name.trim();
        var root = _deviceState.tab === 'to_mpc' ? 'mpc' : 'workspace';
        var newDir = state.destDir ? state.destDir + '/' + name : name;
        fetch('/device/mkdir', {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: 'root=' + encodeURIComponent(root) + '&dir=' + encodeURIComponent(newDir)
        }).then(function(r) {
            if (!r.ok) return r.text().then(function(t) { throw new Error(t); });
            _deviceLoadDest(newDir);
        }).catch(function(err) {
            alert('Create folder failed: ' + err.message);
        });
    }

    function executeDeviceTransfer() {
        if (!_deviceState) return;
        var state = _deviceState[_deviceState.tab];
        var srcPaths = Object.keys(state.selected);
        if (srcPaths.length === 0) return;

        var destLabel = '/' + (state.destDir || '');
        var n = srcPaths.length;
        if (!confirm('Copy ' + n + (n === 1 ? ' item' : ' items') + ' to ' + destLabel + '?')) return;

        var btn = document.getElementById('device-xfer-btn');
        var msg = document.getElementById('device-result-msg');
        if (btn) btn.disabled = true;
        if (msg) { msg.textContent = 'Copying…'; msg.style.color = '#aaa'; }

        var body = 'direction=' + encodeURIComponent(_deviceState.tab) +
                   '&dest_dir=' + encodeURIComponent(state.destDir) +
                   '&create_dest=true';
        srcPaths.forEach(function(p) { body += '&src=' + encodeURIComponent(p); });

        fetch('/device/transfer', {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: body
        }).then(function(r) { return r.json(); })
        .then(function(data) {
            if (msg) {
                if (data.failed > 0) {
                    msg.style.color = '#f88';
                    var failMsg = data.transferred + ' copied, ' + data.failed + ' failed';
                    if (data.messages && data.messages.length > 0) failMsg += ': ' + data.messages.join('; ');
                    msg.textContent = failMsg;
                } else {
                    msg.style.color = '#a0c840';
                    msg.textContent = '✓ ' + data.transferred + ' file(s) copied';
                }
            }
            if (btn) btn.disabled = false;
            _deviceLoadDest(state.destDir);
            if (_deviceState.tab === 'from_mpc') {
                htmx.ajax('GET', '/browse/nav', { target: '#file-nav' });
            }
        })
        .catch(function(err) {
            if (msg) { msg.style.color = '#f88'; msg.textContent = 'Error: ' + err.message; }
            if (btn) btn.disabled = false;
        });
    }

    function _fmtBytes(b) {
        if (b < 1024) return b + 'B';
        if (b < 1048576) return (b / 1024).toFixed(1) + 'K';
        return (b / 1048576).toFixed(1) + 'M';
    }

    // Expose public API (called from onclick attributes and templates)
    window.openDeviceModal = openDeviceModal;
    window.closeDeviceModal = closeDeviceModal;
    window.switchDeviceTab = switchDeviceTab;
    window.toggleDeviceSel = toggleDeviceSel;
    window.openDeviceMkdir = openDeviceMkdir;
    window.executeDeviceTransfer = executeDeviceTransfer;
    // These are referenced by name in generated onclick strings
    window._deviceLoadSource = _deviceLoadSource;
    window._deviceLoadDest = _deviceLoadDest;
})();
