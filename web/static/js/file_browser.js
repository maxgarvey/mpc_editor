// --- Context Menu for File Browser ---

(function() {
    var activeMenu = null;
    var _moveSrcPath;
    var _moveDestDir;

    function dismissMenu() {
        if (activeMenu) {
            activeMenu.remove();
            activeMenu = null;
        }
    }

    function showContextMenu(e, entry) {
        e.preventDefault();
        e.stopPropagation();
        dismissMenu();

        var path = entry.getAttribute('data-path');
        if (!path) return;

        var menu = document.createElement('div');
        menu.className = 'context-menu';

        var renameItem = document.createElement('div');
        renameItem.className = 'context-menu-item';
        renameItem.textContent = 'Rename';
        renameItem.addEventListener('click', function() {
            dismissMenu();
            startInlineRename(entry, path);
        });

        var moveItem = document.createElement('div');
        moveItem.className = 'context-menu-item';
        moveItem.textContent = 'Move';
        moveItem.addEventListener('click', function() {
            dismissMenu();
            var isDir = entry.classList.contains('is-dir');
            openMoveModal(path, isDir);
        });

        var deleteItem = document.createElement('div');
        deleteItem.className = 'context-menu-item context-menu-item-danger';
        deleteItem.textContent = 'Delete';
        deleteItem.addEventListener('click', function() {
            dismissMenu();
            var isDir = entry.classList.contains('is-dir');
            openDeleteModal(path, isDir);
        });

        menu.appendChild(renameItem);
        menu.appendChild(moveItem);
        menu.appendChild(deleteItem);

        // Position at cursor, keep on screen
        document.body.appendChild(menu);
        var x = e.clientX;
        var y = e.clientY;
        var rect = menu.getBoundingClientRect();
        if (x + rect.width > window.innerWidth) x = window.innerWidth - rect.width - 4;
        if (y + rect.height > window.innerHeight) y = window.innerHeight - rect.height - 4;
        menu.style.left = x + 'px';
        menu.style.top = y + 'px';
        activeMenu = menu;
    }

    // Delegated listeners on #file-nav (persists across HTMX swaps)
    document.addEventListener('contextmenu', function(e) {
        var entry = e.target.closest('#file-nav .browser-entry');
        if (entry) showContextMenu(e, entry);
        else dismissMenu();
    });

    // Ctrl+Click for macOS
    document.addEventListener('click', function(e) {
        if (e.ctrlKey) {
            var entry = e.target.closest('#file-nav .browser-entry');
            if (entry) {
                showContextMenu(e, entry);
                return;
            }
        }
        dismissMenu();
    });

    document.addEventListener('keydown', function(e) {
        if (e.key === 'Escape') dismissMenu();
    });

    // --- Inline Rename ---

    function startInlineRename(entry, path) {
        var nameSpan = entry.querySelector('.entry-name');
        if (!nameSpan) return;

        var oldName = nameSpan.textContent;
        var input = document.createElement('input');
        input.type = 'text';
        input.className = 'rename-input';
        input.value = oldName;

        nameSpan.textContent = '';
        nameSpan.appendChild(input);
        input.focus();
        input.select();

        // Prevent the entry's click/hx handlers from firing while renaming
        entry.style.pointerEvents = 'none';
        input.style.pointerEvents = 'auto';

        var committed = false;

        function commit() {
            if (committed) return;
            committed = true;
            var newName = input.value.trim();
            entry.style.pointerEvents = '';

            if (!newName || newName === oldName) {
                // Cancel — restore original name
                nameSpan.textContent = oldName;
                return;
            }

            nameSpan.textContent = newName;

            // POST rename to server
            htmx.ajax('POST', '/workspace/rename', {
                target: '#file-nav',
                values: { path: path, name: newName }
            });
        }

        function cancel() {
            if (committed) return;
            committed = true;
            entry.style.pointerEvents = '';
            nameSpan.textContent = oldName;
        }

        input.addEventListener('keydown', function(e) {
            if (e.key === 'Enter') { e.preventDefault(); commit(); }
            if (e.key === 'Escape') { e.preventDefault(); cancel(); }
            e.stopPropagation();
        });

        input.addEventListener('blur', function() {
            // Small delay so click on another element registers first
            setTimeout(commit, 100);
        });

        // Prevent click on the input from bubbling to the entry
        input.addEventListener('click', function(e) { e.stopPropagation(); });
    }

    // --- Delete Modal ---

    function openDeleteModal(path, isDir) {
        var itemType = isDir ? 'directory' : 'file';
        var name = path.split('/').pop();

        var overlay = document.createElement('div');
        overlay.id = 'delete-overlay';
        overlay.className = 'file-browser-overlay';
        overlay.addEventListener('click', function(e) {
            if (e.target === overlay) closeDeleteModal();
        });

        var modal = document.createElement('div');
        modal.className = 'file-browser-modal';
        modal.style.maxWidth = '420px';
        modal.innerHTML =
            '<h3>Delete ' + itemType + '</h3>' +
            '<p style="margin:8px 0;word-break:break-all"><strong>' + name + '</strong></p>' +
            '<p style="margin:8px 0;color:#aaa;font-size:12px">' + path + '</p>' +
            '<div class="save-confirm-actions" style="flex-direction:column;gap:8px;margin-top:16px">' +
                '<button class="btn-primary" style="background:#c44" onclick="confirmDelete(\'' + escapeAttr(path) + '\', \'disk\')">Delete from disk</button>' +
                '<button class="btn-primary" onclick="confirmDelete(\'' + escapeAttr(path) + '\', \'catalog\')">Remove from catalog only</button>' +
                '<button class="btn-primary" onclick="closeDeleteModal()">Cancel</button>' +
            '</div>';

        overlay.appendChild(modal);
        document.body.appendChild(overlay);
    }

    function closeDeleteModal() {
        var overlay = document.getElementById('delete-overlay');
        if (overlay) overlay.remove();
    }

    function confirmDelete(path, mode) {
        closeDeleteModal();
        fetch('/workspace/delete', {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: 'path=' + encodeURIComponent(path) + '&mode=' + mode
        }).then(function(r) {
            if (!r.ok) return r.text().then(function(t) { throw new Error(t); });
            var prefix = path + '/';
            TabManager.getTabs().forEach(function(tab) {
                if (tab.path === path || tab.path.startsWith(prefix)) {
                    TabManager.close(tab.id);
                }
            });
            refreshBrowserNav();
        }).catch(function(err) {
            alert('Delete failed: ' + err.message);
        });
    }

    // --- Move Modal ---

    function openMoveModal(srcPath, isDir) {
        var overlay = document.createElement('div');
        overlay.id = 'move-overlay';
        overlay.className = 'file-browser-overlay';
        overlay.addEventListener('click', function(e) {
            if (e.target === overlay) closeMoveModal();
        });

        var fileName = srcPath.split('/').pop();

        var modal = document.createElement('div');
        modal.className = 'save-confirm-modal';
        modal.innerHTML =
            '<div class="save-confirm-header">Move "' + escapeHtml(fileName) + '"</div>' +
            '<div id="move-dirs-container" class="save-confirm-body" style="padding:0; max-height:400px; overflow-y:auto;">' +
                '<div style="padding:16px; color:#888;">Loading...</div>' +
            '</div>' +
            '<div class="move-dest-label">Destination: <strong id="move-dest-display">(select a folder)</strong></div>' +
            '<div class="save-confirm-actions">' +
                '<button class="btn-primary" id="move-confirm-btn" onclick="confirmMove()" disabled>Move</button>' +
                '<button class="btn-primary" onclick="closeMoveModal()">Cancel</button>' +
            '</div>';

        overlay.appendChild(modal);
        document.body.appendChild(overlay);

        _moveSrcPath = srcPath;
        _moveDestDir = null;

        // Load directory listing
        loadMoveDirs('');

        document.addEventListener('keydown', moveDirKeyHandler);
    }

    function moveDirKeyHandler(e) {
        if (e.key === 'Escape') closeMoveModal();
    }

    function closeMoveModal() {
        var overlay = document.getElementById('move-overlay');
        if (overlay) overlay.remove();
        document.removeEventListener('keydown', moveDirKeyHandler);
    }

    function loadMoveDirs(dir) {
        var url = '/workspace/dirs?dir=' + encodeURIComponent(dir);
        fetch(url).then(function(r) { return r.text(); }).then(function(html) {
            var container = document.getElementById('move-dirs-container');
            if (container) {
                container.innerHTML = html;
                if (typeof htmx !== 'undefined') htmx.process(container);
                attachMoveDirClickSelection(container);
            }
        });
    }

    function attachMoveDirClickSelection(container) {
        var entries = (container || document).querySelectorAll('#move-dirs-container .move-dir-entry');
        entries.forEach(function(entry) {
            entry.addEventListener('click', function() {
                entries.forEach(function(e) { e.classList.remove('selected'); });
                entry.classList.add('selected');
                var path = entry.getAttribute('data-path');
                _moveDestDir = path;
                var display = document.getElementById('move-dest-display');
                if (display) display.textContent = entry.querySelector('.entry-name').textContent;
                var btn = document.getElementById('move-confirm-btn');
                if (btn) btn.disabled = false;
            });
        });
        var currentDirInput = document.getElementById('move-current-dir');
        if (currentDirInput && currentDirInput.value) {
            _moveDestDir = currentDirInput.value;
            var display = document.getElementById('move-dest-display');
            var dirName = currentDirInput.value.split('/').pop();
            if (display) display.textContent = dirName + ' (current folder)';
            var btn = document.getElementById('move-confirm-btn');
            if (btn) btn.disabled = false;
        }
    }

    function confirmMove() {
        var dest = _moveDestDir;
        var src = _moveSrcPath;
        if (!dest || !src) return;

        closeMoveModal();

        htmx.ajax('POST', '/workspace/move', {
            target: '#file-nav',
            values: { path: src, dest: dest }
        });
    }

    function openOrganizeConfirm(dir) {
        var overlay = document.createElement('div');
        overlay.id = 'organize-overlay';
        overlay.className = 'file-browser-overlay';
        overlay.addEventListener('click', function(e) {
            if (e.target === overlay) overlay.remove();
        });

        var dirName = dir.split('/').pop() || dir;
        var modal = document.createElement('div');
        modal.className = 'save-confirm-modal';
        modal.innerHTML =
            '<div class="save-confirm-header">Organize by label</div>' +
            '<div class="save-confirm-body" style="font-size:13px;line-height:1.5">' +
                'Move labeled WAV files in <strong>' + escapeHtml(dirName) + '</strong> into ' +
                'subcategory subfolders (kick/, snare/, hihat/, …).<br>' +
                'Unlabeled files are not moved.' +
            '</div>' +
            '<div class="save-confirm-actions">' +
                '<button class="btn-primary" id="organize-confirm-btn">Organize</button>' +
                '<button class="btn-sm" onclick="document.getElementById(\'organize-overlay\').remove()">Cancel</button>' +
            '</div>';

        overlay.appendChild(modal);
        document.body.appendChild(overlay);

        document.getElementById('organize-confirm-btn').addEventListener('click', function() {
            overlay.remove();
            htmx.ajax('POST', '/workspace/organize', {
                target: '#file-nav',
                values: { dir: dir }
            });
        });
    }

    // Expose public API (called from onclick attributes and app.js)
    window.openDeleteModal = openDeleteModal;
    window.closeDeleteModal = closeDeleteModal;
    window.confirmDelete = confirmDelete;
    window.openMoveModal = openMoveModal;
    window.closeMoveModal = closeMoveModal;
    window.loadMoveDirs = loadMoveDirs;
    window.attachMoveDirClickSelection = attachMoveDirClickSelection;
    window.confirmMove = confirmMove;
    window.openOrganizeConfirm = openOrganizeConfirm;
})();
