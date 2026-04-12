// MPC Editor - Tab Manager for detail panel

const TabManager = (function() {
    var _tabs = [];       // { id, path, label, ext }
    var _activeTabId = null;
    var _nextId = 1;

    function getExtLabel(path) {
        var dot = path.lastIndexOf('.');
        if (dot < 0) return '';
        return path.substring(dot + 1).toUpperCase();
    }

    function getFileName(path) {
        var slash = path.lastIndexOf('/');
        return slash >= 0 ? path.substring(slash + 1) : path;
    }

    function findTabByPath(path) {
        for (var i = 0; i < _tabs.length; i++) {
            if (_tabs[i].path === path) return _tabs[i];
        }
        return null;
    }

    function findTabById(id) {
        for (var i = 0; i < _tabs.length; i++) {
            if (_tabs[i].id === id) return _tabs[i];
        }
        return null;
    }

    function openFile(path) {
        var existing = findTabByPath(path);
        if (existing) {
            activate(existing.id);
            return;
        }

        var label = getFileName(path);
        var ext = getExtLabel(path);
        var id = _nextId++;

        var tab = { id: id, path: path, label: label, ext: ext };
        _tabs.push(tab);

        renderTabBar();
        activate(id);
    }

    function activate(tabId) {
        var tab = findTabById(tabId);
        if (!tab) return;

        _activeTabId = tabId;

        // Update tab bar active states
        var tabBar = document.getElementById('detail-tabs');
        if (tabBar) {
            var buttons = tabBar.querySelectorAll('.detail-tab');
            buttons.forEach(function(btn) {
                btn.classList.toggle('active', btn.getAttribute('data-tab-id') === String(tabId));
            });
        }

        // Fetch and render the detail content
        var content = document.getElementById('detail-tab-content');
        if (!content) return;

        fetch('/detail?path=' + encodeURIComponent(tab.path))
            .then(function(r) { return r.text(); })
            .then(function(html) {
                content.innerHTML = html;
                // Process HTMX attributes in the new content
                if (typeof htmx !== 'undefined') {
                    htmx.process(content);
                }
                // Re-initialize UI components
                if (typeof initDragDrop === 'function') initDragDrop();
                if (typeof initTabs === 'function') initTabs();
                // Process any inline scripts in the response
                var scripts = content.querySelectorAll('script');
                scripts.forEach(function(oldScript) {
                    if (oldScript.src) {
                        // Skip external scripts already loaded in the page
                        // to avoid redeclaring top-level const/let bindings.
                        var existing = document.querySelector('script[src="' + oldScript.getAttribute('src') + '"]');
                        if (existing) {
                            oldScript.remove();
                            return;
                        }
                    }
                    var newScript = document.createElement('script');
                    if (oldScript.src) {
                        newScript.src = oldScript.src;
                    } else {
                        newScript.textContent = oldScript.textContent;
                    }
                    oldScript.parentNode.replaceChild(newScript, oldScript);
                });
                highlightBrowser();
            })
            .catch(function(err) {
                console.warn('Tab activate failed:', err);
            });

        // Update server-side selected path
        fetch('/detail/select', {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: 'path=' + encodeURIComponent(tab.path)
        }).catch(function() {});
    }

    function close(tabId, evt) {
        if (evt) {
            evt.stopPropagation();
        }

        var idx = -1;
        for (var i = 0; i < _tabs.length; i++) {
            if (_tabs[i].id === tabId) { idx = i; break; }
        }
        if (idx < 0) return;

        _tabs.splice(idx, 1);

        if (_activeTabId === tabId) {
            if (_tabs.length > 0) {
                // Activate the nearest tab
                var nextIdx = Math.min(idx, _tabs.length - 1);
                _activeTabId = null;
                renderTabBar();
                activate(_tabs[nextIdx].id);
            } else {
                _activeTabId = null;
                renderTabBar();
                showWelcome();
            }
        } else {
            renderTabBar();
            highlightBrowser();
        }
    }

    function showWelcome() {
        var content = document.getElementById('detail-tab-content');
        if (content) {
            content.innerHTML =
                '<div class="detail-welcome">' +
                '<h2>Welcome to MPC Editor</h2>' +
                '<p>Select a file from the browser to get started.</p>' +
                '</div>';
        }
        // Clear server-side selected path
        fetch('/detail/select', {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: 'path='
        }).catch(function() {});

        highlightBrowser();
    }

    function renderTabBar() {
        var tabBar = document.getElementById('detail-tabs');
        if (!tabBar) return;

        if (_tabs.length === 0) {
            tabBar.innerHTML = '';
            return;
        }

        var html = '';
        for (var i = 0; i < _tabs.length; i++) {
            var t = _tabs[i];
            var activeClass = t.id === _activeTabId ? ' active' : '';
            html += '<div class="detail-tab' + activeClass + '" data-tab-id="' + t.id + '" onclick="TabManager.activate(' + t.id + ')">' +
                '<span class="detail-tab-type">[' + t.ext + ']</span>' +
                '<span class="detail-tab-name">' + escapeHtml(t.label) + '</span>' +
                '<button class="detail-tab-close" onclick="TabManager.close(' + t.id + ', event)" title="Close">&times;</button>' +
                '</div>';
        }
        tabBar.innerHTML = html;
    }

    function highlightBrowser() {
        var entries = document.querySelectorAll('#file-nav .browser-entry');
        var openPaths = {};
        for (var i = 0; i < _tabs.length; i++) {
            openPaths[_tabs[i].path] = true;
        }
        var activePath = _activeTabId ? (findTabById(_activeTabId) || {}).path : null;

        entries.forEach(function(entry) {
            var entryPath = entry.getAttribute('data-path') || '';
            entry.classList.toggle('active', entryPath === activePath);
            entry.classList.toggle('open', !!openPaths[entryPath] && entryPath !== activePath);
        });
    }

    function escapeHtml(str) {
        var div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }

    // Called after browser nav re-renders (HTMX swap) to re-apply highlighting
    function refreshBrowserHighlight() {
        highlightBrowser();
    }

    function getActiveTab() {
        if (!_activeTabId) return null;
        return findTabById(_activeTabId);
    }

    function getTabs() {
        return _tabs.slice();
    }

    return {
        openFile: openFile,
        activate: activate,
        close: close,
        getActiveTab: getActiveTab,
        getTabs: getTabs,
        refreshBrowserHighlight: refreshBrowserHighlight
    };
})();
