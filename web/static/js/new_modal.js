// --- New Modal ---

(function() {
    var _importFiles = [];
    var _importDirs = [];

    function openNewModal() {
        var dirInput = document.getElementById('browser-current-dir');
        var workspace = dirInput ? dirInput.dataset.workspace : '';
        var destDir = workspace ? workspace + '/sample_library' : (dirInput ? dirInput.value : '');

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
                '<button class="new-modal-tab active" data-tab="new-project">New Program</button>' +
                '<button class="new-modal-tab" data-tab="new-seq">New Sequence</button>' +
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
                        '<button class="btn-primary" id="new-project-btn" onclick="confirmNewProject()" disabled>Create Program</button>' +
                    '</div>' +
                '</div>' +
                '<div id="new-seq-tab" class="new-modal-tab-content" style="display:none">' +
                    '<p style="color:#aaa;margin-bottom:12px">Create a blank 1-bar, 120 BPM sequence.</p>' +
                    '<div style="margin-bottom:12px">' +
                        '<label style="color:#aaa;display:block;margin-bottom:4px">Sequence name <span style="color:#666">(max 16 chars, .SEQ appended)</span></label>' +
                        '<input type="text" id="new-seq-name" class="path-input" maxlength="16" ' +
                            'placeholder="e.g. Sequence01" style="width:100%" ' +
                            'oninput="validateSeqName(this)">' +
                        '<div id="seq-name-hint" style="font-size:11px;margin-top:4px;color:#666"></div>' +
                    '</div>' +
                    '<div style="margin-bottom:12px">' +
                        '<label style="color:#aaa;display:block;margin-bottom:4px">Directory</label>' +
                        '<input type="text" id="new-seq-dir" class="path-input" style="width:100%" placeholder="">' +
                    '</div>' +
                    '<div class="import-actions">' +
                        '<button class="btn-primary" id="new-seq-btn" onclick="confirmNewSeq()" disabled>Create Sequence</button>' +
                    '</div>' +
                '</div>' +
                '<div id="import-files-tab" class="new-modal-tab-content" style="display:none">' +
                    '<div class="import-dest">' +
                        'Import to: <input type="hidden" id="import-dest-path" value="' + escapeHtml(destDir) + '">' +
                        '<span class="import-dest-path" onclick="changeImportDest()">' + (destDir || 'workspace root') + '</span>' +
                    '</div>' +
                    '<div class="import-drop-zone" id="import-drop-zone">' +
                        'Drag and drop files or folders here<br>' +
                        '<span class="import-drop-zone-hint">.wav .mp3 .flac .ogg .aif .m4a .pgm .seq .mid .sng .all</span>' +
                    '</div>' +
                    '<input type="file" id="import-file-input" multiple accept=".wav,.mp3,.flac,.ogg,.aif,.aiff,.m4a,.wma,.opus,.pgm,.seq,.mid,.sng,.all" style="display:none" onchange="handleImportFileSelect(this)">' +
                    '<div style="text-align:center;margin-top:8px">' +
                        '<button class="btn-sm" onclick="document.getElementById(\'import-file-input\').click()">Browse Files</button>' +
                    '</div>' +
                    '<div style="margin-top:10px">' +
                        '<div style="color:#aaa;font-size:12px;margin-bottom:4px">Import folder — paste or drag a folder path here:</div>' +
                        '<div style="display:flex;gap:6px">' +
                            '<input type="text" id="import-folder-path" class="path-input" style="flex:1" placeholder="/path/to/samples" ' +
                                'ondrop="handleFolderPathDrop(event)" ondragover="event.preventDefault()">' +
                            '<button class="btn-sm" onclick="addImportFolderByPath()">Add Folder</button>' +
                        '</div>' +
                    '</div>' +
                    '<div class="import-file-list" id="import-file-list"></div>' +
                    '<div style="margin:6px 0 2px">' +
                        '<label style="display:flex;align-items:center;gap:8px;color:#aaa;cursor:pointer">' +
                            '<input type="checkbox" id="import-flatten"> ' +
                            'Flatten — copy all files into destination without subdirectories' +
                        '</label>' +
                    '</div>' +
                    '<div class="import-attribution">' +
                        '<label class="settings-label">Source / Attribution (optional)</label>' +
                        '<input type="text" id="import-source" class="path-input" style="width:100%" placeholder="e.g. Splice, freesound.org, recorded live">' +
                        '<p class="settings-hint">Applied to all imported WAV files as their source.</p>' +
                    '</div>' +
                    '<div class="import-actions">' +
                        '<button class="btn-primary" id="import-btn" onclick="confirmImportDest()" disabled>Import</button>' +
                    '</div>' +
                    '<div id="import-progress-wrap" style="display:none;margin-top:10px">' +
                        '<div class="upload-progress-track">' +
                            '<div class="upload-progress-fill" id="import-progress-fill"></div>' +
                        '</div>' +
                        '<div id="import-progress-label" class="upload-progress-label">Uploading...</div>' +
                    '</div>' +
                '</div>' +
            '</div>';

        overlay.appendChild(modal);
        document.body.appendChild(overlay);

        // Populate default seq dir from current program path
        var pgmPathEl = document.getElementById('save-pgm-path');
        var seqDirEl = document.getElementById('new-seq-dir');
        if (seqDirEl) {
            var defaultSeqDir = '';
            if (pgmPathEl && pgmPathEl.value) {
                defaultSeqDir = pgmPathEl.value.replace(/\/[^\/]+$/, '');
            } else {
                defaultSeqDir = workspace || '';
            }
            seqDirEl.value = defaultSeqDir;
        }

        // Tab switching
        var tabs = modal.querySelectorAll('.new-modal-tab');
        var tabIds = ['new-project-tab', 'new-seq-tab', 'import-files-tab'];
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
        _importDirs = [];
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
            hint.textContent = 'Creates: programs/' + name + '/' + name + '.pgm';
            hint.style.color = '#666';
        }

        btn.disabled = false;
    }

    function confirmNewProject() {
        var input = document.getElementById('new-project-name');
        var name = input ? input.value.trim() : '';
        if (!name) return;

        closeNewModal();
        fetch('/project/new', {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: 'name=' + encodeURIComponent(name)
        }).then(function(r) {
            if (!r.ok) return r.text().then(function(t) { throw new Error(t); });
            return r.json();
        }).then(function(data) {
            TabManager.openFile(data.pgm_abs);
            htmx.ajax('GET', '/browse/nav?dir=' + encodeURIComponent(data.project_dir), { target: '#file-nav' });
        }).catch(function(err) {
            alert('Create program failed: ' + err.message);
        });
    }

    function validateSeqName(input) {
        var name = input.value.trim();
        var btn = document.getElementById('new-seq-btn');
        var hint = document.getElementById('seq-name-hint');
        if (!btn || !hint) return;

        if (name.length === 0) {
            btn.disabled = true;
            hint.textContent = '';
            hint.style.color = '#666';
            return;
        }

        if (/[\/\\]/.test(name) || name === '.' || name === '..') {
            btn.disabled = true;
            hint.textContent = 'Invalid characters in name';
            hint.style.color = '#ff6b4a';
            return;
        }

        if (/\s/.test(name)) {
            hint.textContent = 'Spaces may cause issues on some MPC firmware';
            hint.style.color = '#c8a040';
        } else {
            hint.textContent = 'Creates: ' + name + '.SEQ';
            hint.style.color = '#666';
        }

        btn.disabled = false;
    }

    function confirmNewSeq() {
        var nameEl = document.getElementById('new-seq-name');
        var dirEl = document.getElementById('new-seq-dir');
        var name = nameEl ? nameEl.value.trim() : '';
        var dir = dirEl ? dirEl.value.trim() : '';
        if (!name) return;

        closeNewModal();
        fetch('/sequence/new', {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: 'name=' + encodeURIComponent(name) + '&dir=' + encodeURIComponent(dir)
        }).then(function(r) {
            if (!r.ok) return r.text().then(function(t) { throw new Error(t); });
            return r.json();
        }).then(function(data) {
            TabManager.openFile(data.seq_abs);
            var dirEl2 = document.getElementById('browser-current-dir');
            if (dirEl2) {
                htmx.ajax('GET', '/browse/nav?dir=' + encodeURIComponent(dir || dirEl2.value), { target: '#file-nav' });
            }
        }).catch(function(err) {
            alert('Create sequence failed: ' + err.message);
        });
    }

    function confirmNewProgram() {
        closeNewModal();
        htmx.ajax('POST', '/program/new', { target: 'body' });
    }

    function changeImportDest() {
        openBrowser('select-dir', 'import-dest-path');
        // Watch for the browser overlay to be removed, then sync the displayed path.
        // MutationObserver fires exactly once on removal — no polling, no leak.
        var observer = new MutationObserver(function() {
            if (!document.getElementById('browser-overlay')) {
                observer.disconnect();
                var input = document.getElementById('import-dest-path');
                var display = document.querySelector('.import-dest-path');
                if (input && display) {
                    display.textContent = input.value || 'workspace root';
                }
            }
        });
        observer.observe(document.body, { childList: true, subtree: false });
    }

    function handleImportFileSelect(input) {
        if (input.files && input.files.length > 0) {
            addImportFiles(input.files);
        }
        input.value = '';
    }

    function handleFolderPathDrop(e) {
        e.preventDefault();
        var text = e.dataTransfer.getData('text');
        if (text) {
            var input = document.getElementById('import-folder-path');
            if (input) input.value = text.trim();
        }
    }

    function addImportFolderByPath() {
        var input = document.getElementById('import-folder-path');
        var path = input ? input.value.trim() : '';
        if (!path) return;
        addImportDir(path);
        if (input) input.value = '';
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

        var hasItems = _importFiles.length > 0 || _importDirs.length > 0;
        if (!hasItems) {
            list.innerHTML = '';
            if (btn) btn.disabled = true;
            return;
        }

        var html = '';
        for (var d = 0; d < _importDirs.length; d++) {
            var dir = _importDirs[d];
            var name = dir.path.replace(/\\/g, '/').split('/').filter(Boolean).pop() || dir.path;
            var extSummary = Object.entries(dir.by_ext || {})
                .sort(function(a, b) { return b[1] - a[1]; })
                .map(function(kv) { return kv[1] + ' ' + kv[0]; })
                .join(', ');
            html += '<div class="import-file-item">' +
                '<span>📂 ' + name + ' <span style="color:#888">(' + dir.count + ' files' + (extSummary ? ': ' + extSummary : '') + ')</span></span>' +
                '<button class="import-file-remove" onclick="removeImportDir(' + d + ')">&times;</button>' +
                '</div>';
        }
        for (var i = 0; i < _importFiles.length; i++) {
            var f = _importFiles[i];
            var ext = f.name.lastIndexOf('.') >= 0 ? f.name.substring(f.name.lastIndexOf('.')) : '';
            var size = f.size < 1024 ? f.size + ' B' :
                       f.size < 1048576 ? Math.round(f.size / 1024) + ' KB' :
                       (f.size / 1048576).toFixed(1) + ' MB';
            var displayName = f.webkitRelativePath || f.name;
            html += '<div class="import-file-item">' +
                '<span>' + ext.toUpperCase().replace('.', '[') + '] ' + displayName + ' <span style="color:#888">(' + size + ')</span></span>' +
                '<button class="import-file-remove" onclick="removeImportFile(' + i + ')">&times;</button>' +
                '</div>';
        }
        list.innerHTML = html;
        if (btn) btn.disabled = false;
    }

    function removeImportDir(index) {
        _importDirs.splice(index, 1);
        renderImportFileList();
    }

    function addImportDir(path) {
        fetch('/workspace/import/scan?dir=' + encodeURIComponent(path))
            .then(function(r) { return r.json(); })
            .then(function(data) {
                if (data.count === 0) {
                    alert('No importable files found in that folder.');
                    return;
                }
                _importDirs.push({ path: data.dir, count: data.count, by_ext: data.by_ext });
                renderImportFileList();
            })
            .catch(function(err) {
                console.warn('Folder scan failed:', err);
            });
    }

    function confirmImportDest() {
        if (_importFiles.length === 0 && _importDirs.length === 0) return;

        var destInput = document.getElementById('import-dest-path');
        var destDir = destInput ? destInput.value : '';
        var displayDest = destDir || 'workspace root';
        var sourceInput = document.getElementById('import-source');
        var source = sourceInput ? sourceInput.value.trim() : '';
        var flattenEl = document.getElementById('import-flatten');
        var flatten = flattenEl ? flattenEl.checked : false;

        var totalFiles = _importFiles.length;
        var totalDirFiles = _importDirs.reduce(function(s, d) { return s + d.count; }, 0);
        var parts = [];
        if (totalFiles > 0) parts.push(totalFiles + ' file' + (totalFiles > 1 ? 's' : ''));
        if (_importDirs.length > 0) parts.push(totalDirFiles + ' files from ' + _importDirs.length + ' folder' + (_importDirs.length > 1 ? 's' : '') + (flatten ? ' (flattened)' : ''));
        var msg = 'Import ' + parts.join(' + ') + ' to:\n' + displayDest;
        if (source) msg += '\n\nSource attribution: ' + source;

        if (confirm(msg)) {
            doWorkspaceImport();
        }
    }

    function doWorkspaceImport() {
        if (_importFiles.length === 0 && _importDirs.length === 0) return;

        var destInput = document.getElementById('import-dest-path');
        var destDir = destInput ? destInput.value : '';
        var sourceInput = document.getElementById('import-source');
        var source = sourceInput ? sourceInput.value.trim() : '';
        var flattenEl = document.getElementById('import-flatten');
        var flatten = flattenEl ? flattenEl.checked : false;

        // Switch to progress UI
        var btn = document.getElementById('import-btn');
        if (btn) btn.style.display = 'none';
        var progressWrap = document.getElementById('import-progress-wrap');
        var progressFill = document.getElementById('import-progress-fill');
        var progressLabel = document.getElementById('import-progress-label');
        if (progressWrap) progressWrap.style.display = 'block';

        function setProgress(pct, label) {
            if (progressFill) progressFill.style.width = Math.min(100, Math.max(0, pct)) + '%';
            if (progressLabel) progressLabel.textContent = label;
        }

        function resetUI(errMsg) {
            if (progressWrap) progressWrap.style.display = 'none';
            if (btn) { btn.style.display = ''; btn.disabled = false; btn.textContent = 'Import'; }
            if (errMsg) alert('Import failed: ' + errMsg);
        }

        var totalBytes = 0;
        for (var i = 0; i < _importFiles.length; i++) totalBytes += _importFiles[i].size;

        var dirTotal = _importDirs.length;
        var dirsDone = 0;
        var promises = [];

        // File upload via XHR for real progress events
        if (_importFiles.length > 0) {
            var filePromise = new Promise(function(resolve, reject) {
                var formData = new FormData();
                for (var i = 0; i < _importFiles.length; i++) formData.append('files', _importFiles[i]);
                formData.append('dest', destDir);
                if (source) formData.append('source', source);

                var xhr = new XMLHttpRequest();
                xhr.open('POST', '/workspace/import');
                xhr.upload.addEventListener('progress', function(e) {
                    if (!e.lengthComputable) { setProgress(50, 'Uploading...'); return; }
                    var pct = (e.loaded / e.total) * 100;
                    setProgress(pct, 'Uploading ' + formatBytes(e.loaded) + ' / ' + formatBytes(e.total) + ' (' + Math.round(pct) + '%)');
                });
                xhr.addEventListener('load', function() {
                    if (xhr.status >= 200 && xhr.status < 300) {
                        try { resolve(JSON.parse(xhr.responseText)); } catch(e) { resolve({}); }
                    } else {
                        reject(new Error(xhr.status + ' ' + xhr.statusText));
                    }
                });
                xhr.addEventListener('error', function() { reject(new Error('Network error')); });
                xhr.addEventListener('abort', function() { reject(new Error('Aborted')); });
                xhr.send(formData);
            });
            promises.push(filePromise);
        }

        // Dir copy requests (server-side, no upload data — show indeterminate progress)
        for (var d = 0; d < _importDirs.length; d++) {
            (function(dir) {
                var params = new URLSearchParams();
                params.append('src_dir', dir.path);
                params.append('dest', destDir);
                params.append('flatten', flatten ? '1' : '0');
                if (source) params.append('source', source);
                promises.push(
                    fetch('/workspace/import/dir', { method: 'POST', body: params })
                        .then(function(r) { return r.json(); })
                        .then(function(data) {
                            dirsDone++;
                            if (_importFiles.length === 0) {
                                setProgress((dirsDone / dirTotal) * 100,
                                    'Copying folder ' + dirsDone + ' of ' + dirTotal + '...');
                            }
                            return data;
                        })
                );
            })(_importDirs[d]);
        }

        if (_importFiles.length === 0 && dirTotal > 0) {
            setProgress(0, 'Copying ' + dirTotal + ' folder' + (dirTotal > 1 ? 's' : '') + '...');
        }

        Promise.all(promises)
            .then(function() {
                setProgress(100, 'Done!');
                setTimeout(function() {
                    closeNewModal();
                    htmx.ajax('GET', '/browse/nav?dir=' + encodeURIComponent(destDir), '#file-nav');
                }, 500);
            })
            .catch(function(err) {
                console.warn('Import failed:', err);
                resetUI(err.message);
            });
    }

    // Expose public API (called from onclick attributes in dynamically created HTML)
    window.openNewModal = openNewModal;
    window.closeNewModal = closeNewModal;
    window.validateProjectName = validateProjectName;
    window.confirmNewProject = confirmNewProject;
    window.validateSeqName = validateSeqName;
    window.confirmNewSeq = confirmNewSeq;
    window.confirmNewProgram = confirmNewProgram;
    window.changeImportDest = changeImportDest;
    window.handleImportFileSelect = handleImportFileSelect;
    window.handleFolderPathDrop = handleFolderPathDrop;
    window.addImportFolderByPath = addImportFolderByPath;
    window.removeImportFile = removeImportFile;
    window.removeImportDir = removeImportDir;
    window.confirmImportDest = confirmImportDest;
})();
