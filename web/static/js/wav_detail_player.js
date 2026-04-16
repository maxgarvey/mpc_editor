// MPC Editor - WAV detail view transport (play/pause/seek, region select, crop)

const WavDetailPlayer = (function() {
    var audioUrl = '';
    var relPath = '';
    var buffer = null;
    var source = null;
    var overlayCanvas = null;
    var overlayCtx = null;
    var state = 'stopped'; // 'stopped' | 'playing' | 'paused'
    var startContextTime = 0;
    var pauseOffset = 0;
    var duration = 0;
    var frameLength = 0;
    var animFrameId = null;
    var registered = false;

    // Region selection state
    var selStart = -1; // fraction 0-1, -1 = no selection
    var selEnd = -1;
    var dragging = false;
    var dragOrigin = -1;
    var edgeResizing = null; // null | 'start' | 'end'

    var EDGE_HIT_PX = 6; // pixels from edge to trigger resize

    // Selection buttons (DOM elements overlaid on waveform)
    var selButtonsEl = null;

    // Timestamp DOM widgets
    var tsStartEl = null;
    var tsEndEl = null;
    var NUDGE_STEP = 0.02; // seconds per click

    // Context menu
    var activeMenu = null;

    // Event handler refs for cleanup
    var boundMouseDown = null;
    var boundMouseMove = null;
    var boundMouseUp = null;
    var boundContextMenu = null;
    var boundCursorMove = null;

    function init(url, fileRelPath) {
        cleanup();
        audioUrl = url;
        relPath = fileRelPath || '';

        AudioPlayer.getBuffer(url).then(function(buf) {
            buffer = buf;
            duration = buf.duration;
            createOverlayCanvas();
            fetchFrameLength();
        }).catch(function(err) {
            console.warn('WavDetailPlayer: failed to load buffer:', err);
        });

        if (!registered) {
            AudioPlayer.onStopAll(function() { reset(); });
            registered = true;
        }
    }

    function fetchFrameLength() {
        if (!relPath) return;
        fetch('/audio/waveform?path=' + encodeURIComponent(relPath) + '&width=1')
            .then(function(r) { return r.json(); })
            .then(function(d) {
                if (d.frameLength) frameLength = d.frameLength;
            })
            .catch(function() {});
    }

    function cleanup() {
        if (state === 'playing' && source) {
            try { source.stop(); } catch(e) {}
        }
        source = null;
        state = 'stopped';
        pauseOffset = 0;
        buffer = null;
        duration = 0;
        frameLength = 0;
        selStart = -1;
        selEnd = -1;
        dragging = false;
        if (animFrameId) {
            cancelAnimationFrame(animFrameId);
            animFrameId = null;
        }
        dismissMenu();
        removeSelButtons();
        removeTimestamps();
        removeOverlay();
    }

    function createOverlayCanvas() {
        removeOverlay();
        var waveCanvas = document.getElementById('wav-waveform-canvas');
        if (!waveCanvas) return;

        var container = waveCanvas.parentElement;
        if (!container) return;

        overlayCanvas = document.createElement('canvas');
        overlayCanvas.className = 'wav-playhead-overlay';
        overlayCanvas.width = waveCanvas.width;
        overlayCanvas.height = waveCanvas.height;
        overlayCanvas.style.width = waveCanvas.style.width || '100%';
        overlayCanvas.style.height = waveCanvas.style.height || '100%';
        container.appendChild(overlayCanvas);
        overlayCtx = overlayCanvas.getContext('2d');

        // Mouse events on the waveform canvas for seek + region select.
        boundMouseDown = function(e) { handleMouseDown(e); };
        boundMouseMove = function(e) { handleMouseMove(e); };
        boundMouseUp = function(e) { handleMouseUp(e); };
        boundContextMenu = function(e) { handleContextMenu(e); };

        boundCursorMove = function(e) { updateCursor(e); };

        waveCanvas.addEventListener('mousedown', boundMouseDown);
        waveCanvas.addEventListener('mousemove', boundCursorMove);
        window.addEventListener('mousemove', boundMouseMove);
        window.addEventListener('mouseup', boundMouseUp);
        waveCanvas.addEventListener('contextmenu', boundContextMenu);
    }

    function removeOverlay() {
        var waveCanvas = document.getElementById('wav-waveform-canvas');
        if (waveCanvas) {
            if (boundMouseDown) waveCanvas.removeEventListener('mousedown', boundMouseDown);
            if (boundCursorMove) waveCanvas.removeEventListener('mousemove', boundCursorMove);
            if (boundContextMenu) waveCanvas.removeEventListener('contextmenu', boundContextMenu);
        }
        if (boundMouseMove) window.removeEventListener('mousemove', boundMouseMove);
        if (boundMouseUp) window.removeEventListener('mouseup', boundMouseUp);
        boundMouseDown = null;
        boundCursorMove = null;
        boundMouseMove = null;
        boundMouseUp = null;
        boundContextMenu = null;

        if (overlayCanvas && overlayCanvas.parentElement) {
            overlayCanvas.parentElement.removeChild(overlayCanvas);
        }
        overlayCanvas = null;
        overlayCtx = null;
    }

    // --- Mouse interaction ---

    function getFraction(e) {
        var waveCanvas = document.getElementById('wav-waveform-canvas');
        if (!waveCanvas) return 0;
        var rect = waveCanvas.getBoundingClientRect();
        var f = (e.clientX - rect.left) / rect.width;
        if (f < 0) f = 0;
        if (f > 1) f = 1;
        return f;
    }

    // Returns 'start', 'end', or null depending on proximity to selection edges.
    function hitTestEdge(e) {
        if (!hasSelection()) return null;
        var waveCanvas = document.getElementById('wav-waveform-canvas');
        if (!waveCanvas) return null;
        var rect = waveCanvas.getBoundingClientRect();
        var px = e.clientX - rect.left;
        var lo = Math.min(selStart, selEnd);
        var hi = Math.max(selStart, selEnd);
        var startPx = lo * rect.width;
        var endPx = hi * rect.width;

        if (Math.abs(px - startPx) <= EDGE_HIT_PX) return 'start';
        if (Math.abs(px - endPx) <= EDGE_HIT_PX) return 'end';
        return null;
    }

    function updateCursor(e) {
        if (dragging || edgeResizing) return; // cursor set by drag handlers
        var waveCanvas = document.getElementById('wav-waveform-canvas');
        if (!waveCanvas) return;
        var edge = hitTestEdge(e);
        waveCanvas.style.cursor = edge ? 'col-resize' : 'crosshair';
    }

    function handleMouseDown(e) {
        if (!buffer) return;
        // Right-click or Ctrl+Click = context menu, don't start drag.
        if (e.button === 2 || (e.button === 0 && e.ctrlKey)) return;
        if (e.button !== 0) return;

        dismissMenu();

        // Check if clicking on a selection edge to resize.
        var edge = hitTestEdge(e);
        if (edge) {
            edgeResizing = edge;
            removeSelButtons();
            removeTimestamps();
            // Set cursor on body so it stays col-resize even if mouse leaves canvas.
            document.body.style.cursor = 'col-resize';
            e.preventDefault();
            return;
        }

        // Clicking inside a selection is a no-op (don't deselect).
        if (hasSelection()) {
            var f = getFraction(e);
            var lo = Math.min(selStart, selEnd);
            var hi = Math.max(selStart, selEnd);
            if (f >= lo && f <= hi) return;
        }

        removeSelButtons();
        removeTimestamps();
        dragging = true;
        dragOrigin = getFraction(e);
        selStart = dragOrigin;
        selEnd = dragOrigin;
        drawOverlay();
    }

    function handleMouseMove(e) {
        if (edgeResizing) {
            var f = getFraction(e);
            var lo = Math.min(selStart, selEnd);
            var hi = Math.max(selStart, selEnd);

            if (edgeResizing === 'start') {
                lo = f;
                // Don't let start cross past end.
                if (lo >= hi - 0.005) lo = hi - 0.005;
                if (lo < 0) lo = 0;
            } else {
                hi = f;
                if (hi <= lo + 0.005) hi = lo + 0.005;
                if (hi > 1) hi = 1;
            }
            selStart = lo;
            selEnd = hi;
            drawOverlay();
            return;
        }
        if (!dragging) return;
        selEnd = getFraction(e);
        drawOverlay();
    }

    function handleMouseUp(e) {
        if (edgeResizing) {
            edgeResizing = null;
            document.body.style.cursor = '';
            // Normalize.
            var lo = Math.min(selStart, selEnd);
            var hi = Math.max(selStart, selEnd);
            selStart = lo;
            selEnd = hi;
            drawOverlay();
            showSelButtons();
            showTimestamps();
            return;
        }

        if (!dragging) return;
        dragging = false;
        selEnd = getFraction(e);

        var lo = Math.min(selStart, selEnd);
        var hi = Math.max(selStart, selEnd);

        // If the drag was tiny (< 1% of width), treat as a click-to-seek.
        if (hi - lo < 0.01) {
            selStart = -1;
            selEnd = -1;
            seek(lo * duration);
            return;
        }

        selStart = lo;
        selEnd = hi;
        drawOverlay();
        showSelButtons();
        showTimestamps();
    }

    function handleContextMenu(e) {
        if (!buffer) return;
        // Only show crop menu if there's a selection.
        if (!hasSelection()) return;
        e.preventDefault();
        e.stopPropagation();
        showCropMenu(e.clientX, e.clientY);
    }

    function hasSelection() {
        return selStart >= 0 && selEnd >= 0 && Math.abs(selEnd - selStart) >= 0.01;
    }

    // --- Context menu ---

    function showCropMenu(x, y) {
        dismissMenu();

        var menu = document.createElement('div');
        menu.className = 'context-menu';

        var cropItem = document.createElement('div');
        cropItem.className = 'context-menu-item';
        cropItem.textContent = 'Crop to Selection';
        cropItem.addEventListener('click', function() {
            dismissMenu();
            doCrop('replace');
        });
        menu.appendChild(cropItem);

        var copyItem = document.createElement('div');
        copyItem.className = 'context-menu-item';
        copyItem.textContent = 'Save Selection As New File';
        copyItem.addEventListener('click', function() {
            dismissMenu();
            doCrop('copy');
        });
        menu.appendChild(copyItem);

        var cancelItem = document.createElement('div');
        cancelItem.className = 'context-menu-item';
        cancelItem.textContent = 'Clear Selection';
        cancelItem.addEventListener('click', function() {
            dismissMenu();
            clearSelection();
        });
        menu.appendChild(cancelItem);

        document.body.appendChild(menu);

        // Position keeping on-screen.
        var rect = menu.getBoundingClientRect();
        if (x + rect.width > window.innerWidth) x = window.innerWidth - rect.width - 4;
        if (y + rect.height > window.innerHeight) y = window.innerHeight - rect.height - 4;
        menu.style.left = x + 'px';
        menu.style.top = y + 'px';
        activeMenu = menu;

        // Dismiss on click outside or Escape.
        setTimeout(function() {
            window.addEventListener('mousedown', dismissOnClickOutside);
            window.addEventListener('keydown', dismissOnEscape);
        }, 0);
    }

    function dismissOnClickOutside(e) {
        if (activeMenu && !activeMenu.contains(e.target)) {
            dismissMenu();
        }
    }

    function dismissOnEscape(e) {
        if (e.key === 'Escape') dismissMenu();
    }

    function dismissMenu() {
        if (activeMenu) {
            activeMenu.remove();
            activeMenu = null;
        }
        window.removeEventListener('mousedown', dismissOnClickOutside);
        window.removeEventListener('keydown', dismissOnEscape);
    }

    function clearSelection() {
        selStart = -1;
        selEnd = -1;
        removeSelButtons();
        removeTimestamps();
        drawOverlay();
    }

    // --- Selection buttons ---

    function removeSelButtons() {
        if (selButtonsEl) {
            selButtonsEl.remove();
            selButtonsEl = null;
        }
    }

    function showSelButtons() {
        removeSelButtons();
        if (!hasSelection()) return;

        var container = document.querySelector('.wav-waveform-container');
        if (!container) return;

        var containerRect = container.getBoundingClientRect();
        var lo = Math.min(selStart, selEnd);
        var hi = Math.max(selStart, selEnd);
        var selX1 = lo * containerRect.width;
        var selX2 = hi * containerRect.width;
        var selWidth = selX2 - selX1;

        // Create buttons container.
        var wrap = document.createElement('div');
        wrap.className = 'wav-sel-buttons';

        var playBtn = document.createElement('button');
        playBtn.textContent = '\u25B6';
        playBtn.title = 'Play selection';
        playBtn.addEventListener('click', function(e) {
            e.stopPropagation();
            playSelection();
        });
        wrap.appendChild(playBtn);

        var cropBtn = document.createElement('button');
        cropBtn.textContent = 'Crop';
        cropBtn.title = 'Crop options';
        cropBtn.addEventListener('click', function(e) {
            e.stopPropagation();
            var btnRect = cropBtn.getBoundingClientRect();
            showCropMenu(btnRect.right, btnRect.top);
        });
        wrap.appendChild(cropBtn);

        container.appendChild(wrap);
        selButtonsEl = wrap;

        // Measure the buttons to decide placement.
        var btnRect = wrap.getBoundingClientRect();
        var btnW = btnRect.width;
        var btnH = btnRect.height;
        var padding = 4;
        var bottomY = containerRect.height - btnH - padding;

        // Preferred: inside selection, bottom-right corner.
        if (selWidth >= btnW + padding * 2) {
            wrap.style.left = (selX2 - btnW - padding) + 'px';
            wrap.style.top = bottomY + 'px';
        }
        // Fallback: outside selection, to the right of bottom-right corner.
        else if (selX2 + btnW + padding <= containerRect.width) {
            wrap.style.left = (selX2 + padding) + 'px';
            wrap.style.top = bottomY + 'px';
        }
        // Last resort: to the left of the bottom-left corner.
        else {
            wrap.style.left = Math.max(0, selX1 - btnW - padding) + 'px';
            wrap.style.top = bottomY + 'px';
        }
    }

    function playSelection() {
        if (!buffer || !hasSelection()) return;
        // Stop any current playback.
        if (source) {
            try { source.stop(); } catch(e) {}
            source = null;
        }
        if (animFrameId) {
            cancelAnimationFrame(animFrameId);
            animFrameId = null;
        }

        var lo = Math.min(selStart, selEnd);
        var hi = Math.max(selStart, selEnd);
        var startSec = lo * duration;
        var endSec = hi * duration;
        var regionDur = endSec - startSec;

        var ctx = AudioPlayer.getContext();
        source = ctx.createBufferSource();
        source.buffer = buffer;
        source.connect(ctx.destination);
        source.start(0, startSec, regionDur);
        startContextTime = ctx.currentTime;
        pauseOffset = startSec;
        state = 'playing';

        source.onended = function() {
            if (state === 'playing') {
                state = 'paused';
                pauseOffset = endSec;
                if (animFrameId) {
                    cancelAnimationFrame(animFrameId);
                    animFrameId = null;
                }
                drawOverlay();
                updateButton('Play');
            }
        };

        updateButton('Pause');
        animationLoop();
    }

    // --- Timestamp widgets ---

    function removeTimestamps() {
        if (tsStartEl) { tsStartEl.remove(); tsStartEl = null; }
        if (tsEndEl) { tsEndEl.remove(); tsEndEl = null; }
    }

    function showTimestamps() {
        removeTimestamps();
        if (!hasSelection()) return;

        var container = document.querySelector('.wav-waveform-container');
        if (!container) return;

        tsStartEl = createTimestampWidget('start');
        tsEndEl = createTimestampWidget('end');
        container.appendChild(tsStartEl);
        container.appendChild(tsEndEl);
        positionTimestamps();
    }

    function createTimestampWidget(edge) {
        var wrap = document.createElement('div');
        wrap.className = 'wav-ts';
        wrap.setAttribute('data-edge', edge);

        var label = document.createElement('span');
        label.className = 'wav-ts-label';
        label.textContent = edge + ':';
        label.style.display = 'none';
        wrap.appendChild(label);

        var valueSpan = document.createElement('span');
        valueSpan.className = 'wav-ts-value';
        valueSpan.addEventListener('click', function(e) {
            e.stopPropagation();
            startTimestampEdit(wrap, edge);
        });
        wrap.appendChild(valueSpan);

        var nudge = document.createElement('div');
        nudge.className = 'wav-ts-nudge';

        var upBtn = document.createElement('button');
        upBtn.textContent = '\u25B2';
        upBtn.title = '+' + NUDGE_STEP + 's';
        upBtn.addEventListener('click', function(e) {
            e.stopPropagation();
            nudgeEdge(edge, NUDGE_STEP);
        });
        nudge.appendChild(upBtn);

        var downBtn = document.createElement('button');
        downBtn.textContent = '\u25BC';
        downBtn.title = '-' + NUDGE_STEP + 's';
        downBtn.addEventListener('click', function(e) {
            e.stopPropagation();
            nudgeEdge(edge, -NUDGE_STEP);
        });
        nudge.appendChild(downBtn);

        wrap.appendChild(nudge);
        return wrap;
    }

    function positionTimestamps() {
        if (!tsStartEl || !tsEndEl) return;
        if (!hasSelection()) return;

        var container = document.querySelector('.wav-waveform-container');
        if (!container) return;
        var cw = container.clientWidth;

        var lo = Math.min(selStart, selEnd);
        var hi = Math.max(selStart, selEnd);
        var x1 = lo * cw;
        var x2 = hi * cw;

        // Update displayed values.
        var startVal = tsStartEl.querySelector('.wav-ts-value');
        var endVal = tsEndEl.querySelector('.wav-ts-value');
        if (startVal) startVal.textContent = formatTime(lo * duration);
        if (endVal) endVal.textContent = formatTime(hi * duration);

        // First, hide labels and reset top to measure natural widths.
        var startLabel = tsStartEl.querySelector('.wav-ts-label');
        var endLabel = tsEndEl.querySelector('.wav-ts-label');
        if (startLabel) startLabel.style.display = 'none';
        if (endLabel) endLabel.style.display = 'none';
        tsStartEl.style.top = '2px';
        tsEndEl.style.top = '2px';

        // Position start at left edge.
        tsStartEl.style.left = (x1 + 3) + 'px';

        // Measure widths.
        var startW = tsStartEl.offsetWidth;
        var endW = tsEndEl.offsetWidth;

        // Position end at right edge.
        var endLeft = x2 - endW - 3;

        // Check for overlap.
        var overlapping = endLeft < x1 + startW + 6;

        if (overlapping) {
            // Stack: start on top, end below, both at left edge, with labels.
            if (startLabel) startLabel.style.display = '';
            if (endLabel) endLabel.style.display = '';
            tsStartEl.style.left = (x1 + 3) + 'px';
            tsStartEl.style.top = '2px';
            tsEndEl.style.left = (x1 + 3) + 'px';
            tsEndEl.style.top = '18px';
        } else {
            tsEndEl.style.left = endLeft + 'px';
        }
    }

    function nudgeEdge(edge, delta) {
        if (!hasSelection() || duration <= 0) return;
        var lo = Math.min(selStart, selEnd);
        var hi = Math.max(selStart, selEnd);
        var loSec = lo * duration;
        var hiSec = hi * duration;
        var minGap = 0.005; // minimum selection duration in seconds

        if (edge === 'start') {
            loSec += delta;
            if (loSec < 0) loSec = 0;
            if (loSec >= hiSec - minGap) loSec = hiSec - minGap;
            selStart = loSec / duration;
        } else {
            hiSec += delta;
            if (hiSec > duration) hiSec = duration;
            if (hiSec <= loSec + minGap) hiSec = loSec + minGap;
            selEnd = hiSec / duration;
        }
        drawOverlay();
        showSelButtons();
        showTimestamps();
    }

    function startTimestampEdit(wrap, edge) {
        var valueSpan = wrap.querySelector('.wav-ts-value');
        if (!valueSpan) return;

        var lo = Math.min(selStart, selEnd);
        var hi = Math.max(selStart, selEnd);
        var currentSec = (edge === 'start') ? lo * duration : hi * duration;

        // Hide value span and nudge buttons.
        valueSpan.style.display = 'none';
        var nudge = wrap.querySelector('.wav-ts-nudge');
        if (nudge) nudge.style.display = 'none';

        var input = document.createElement('input');
        input.className = 'wav-ts-input';
        input.type = 'text';
        input.value = currentSec.toFixed(3);
        wrap.insertBefore(input, valueSpan);
        input.focus();
        input.select();

        var committed = false;

        function commit() {
            if (committed) return;
            committed = true;
            var raw = parseFloat(input.value);
            input.remove();
            valueSpan.style.display = '';
            if (nudge) nudge.style.display = '';

            if (isNaN(raw)) return; // revert, keep current selection

            applyTimestampValue(edge, raw);
        }

        function cancel() {
            committed = true;
            input.remove();
            valueSpan.style.display = '';
            if (nudge) nudge.style.display = '';
        }

        input.addEventListener('keydown', function(e) {
            if (e.key === 'Enter') {
                e.preventDefault();
                commit();
            } else if (e.key === 'Escape') {
                e.preventDefault();
                cancel();
            }
        });
        input.addEventListener('blur', function() {
            commit();
        });
    }

    function applyTimestampValue(edge, rawSec) {
        if (duration <= 0) return;
        var lo = Math.min(selStart, selEnd);
        var hi = Math.max(selStart, selEnd);
        var loSec = lo * duration;
        var hiSec = hi * duration;
        var minGap = 0.005;

        if (edge === 'start') {
            // Clamp: can't be < 0, can't be >= end
            if (rawSec < 0) rawSec = 0;
            if (rawSec >= hiSec - minGap) rawSec = hiSec - minGap;
            if (rawSec < 0) rawSec = 0;
            selStart = rawSec / duration;
        } else {
            // Clamp: can't be > duration, can't be <= start
            if (rawSec > duration) rawSec = duration;
            if (rawSec <= loSec + minGap) rawSec = loSec + minGap;
            if (rawSec > duration) rawSec = duration;
            selEnd = rawSec / duration;
        }
        drawOverlay();
        showSelButtons();
        showTimestamps();
    }

    // --- Crop ---

    function doCrop(mode) {
        if (!relPath || !hasSelection()) return;

        var lo = Math.min(selStart, selEnd);
        var hi = Math.max(selStart, selEnd);
        var fromFrame = Math.round(lo * frameLength);
        var toFrame = Math.round(hi * frameLength);

        if (toFrame <= fromFrame) return;

        var params = new URLSearchParams();
        params.set('path', relPath);
        params.set('from', fromFrame);
        params.set('to', toFrame);
        params.set('mode', mode);

        fetch('/audio/crop', { method: 'POST', body: params })
            .then(function(r) { return r.json(); })
            .then(function(data) {
                clearSelection();
                // Invalidate audio buffer cache for this URL.
                AudioPlayer.clearCache();

                if (mode === 'replace') {
                    // Reload the same file detail.
                    if (typeof TabManager !== 'undefined') {
                        TabManager.openFile(relPath);
                    }
                } else {
                    // Open the new file.
                    if (typeof TabManager !== 'undefined' && data.path) {
                        TabManager.openFile(data.path);
                    }
                }
            })
            .catch(function(err) {
                console.warn('Crop failed:', err);
            });
    }

    // --- Playback ---

    function play() {
        if (!buffer) return;
        var ctx = AudioPlayer.getContext();

        source = ctx.createBufferSource();
        source.buffer = buffer;
        source.connect(ctx.destination);
        source.start(0, pauseOffset);
        startContextTime = ctx.currentTime;
        state = 'playing';

        source.onended = function() {
            if (state === 'playing') {
                state = 'stopped';
                pauseOffset = 0;
                if (animFrameId) {
                    cancelAnimationFrame(animFrameId);
                    animFrameId = null;
                }
                drawOverlay();
                updateButton('Play');
            }
        };

        updateButton('Pause');
        animationLoop();
    }

    function pause() {
        if (state !== 'playing') return;
        var ctx = AudioPlayer.getContext();
        var elapsed = ctx.currentTime - startContextTime + pauseOffset;
        if (elapsed > duration) elapsed = duration;

        if (source) {
            try { source.stop(); } catch(e) {}
            source = null;
        }

        pauseOffset = elapsed;
        state = 'paused';

        if (animFrameId) {
            cancelAnimationFrame(animFrameId);
            animFrameId = null;
        }

        drawOverlay();
        updateButton('Play');
    }

    function stopPlayback() {
        if (source) {
            try { source.stop(); } catch(e) {}
            source = null;
        }
        state = 'stopped';
        pauseOffset = 0;

        if (animFrameId) {
            cancelAnimationFrame(animFrameId);
            animFrameId = null;
        }

        drawOverlay();
        updateButton('Play');
    }

    function seek(seconds) {
        if (!buffer) return;
        if (seconds < 0) seconds = 0;
        if (seconds > duration) seconds = duration;

        if (state === 'playing') {
            if (source) {
                try { source.stop(); } catch(e) {}
                source = null;
            }
            if (animFrameId) {
                cancelAnimationFrame(animFrameId);
                animFrameId = null;
            }
            pauseOffset = seconds;
            play();
        } else {
            pauseOffset = seconds;
            state = 'paused';
            drawOverlay();
            updateButton('Play');
        }
    }

    function toggle() {
        if (state === 'playing') {
            pause();
        } else {
            play();
        }
    }

    function reset() {
        if (source) { source = null; }
        state = 'stopped';
        pauseOffset = 0;
        if (animFrameId) {
            cancelAnimationFrame(animFrameId);
            animFrameId = null;
        }
        drawOverlay();
        updateButton('Play');
    }

    // --- Drawing ---

    function drawOverlay() {
        if (!overlayCtx || !overlayCanvas) return;
        var w = overlayCanvas.width;
        var h = overlayCanvas.height;
        overlayCtx.clearRect(0, 0, w, h);

        // Draw selection region.
        if (hasSelection()) {
            var lo = Math.min(selStart, selEnd);
            var hi = Math.max(selStart, selEnd);
            var x1 = Math.round(lo * w);
            var x2 = Math.round(hi * w);

            // Dim the areas outside the selection.
            overlayCtx.fillStyle = 'rgba(0, 0, 0, 0.45)';
            overlayCtx.fillRect(0, 0, x1, h);
            overlayCtx.fillRect(x2, 0, w - x2, h);

            // Bright fill inside selection.
            overlayCtx.fillStyle = 'rgba(80, 180, 255, 0.18)';
            overlayCtx.fillRect(x1, 0, x2 - x1, h);

            // Selection box outline.
            overlayCtx.strokeStyle = '#50b4ff';
            overlayCtx.lineWidth = 2;
            overlayCtx.strokeRect(x1, 1, x2 - x1, h - 2);

            // Position DOM timestamp widgets (created by showTimestamps).
            positionTimestamps();
        }

        // Draw playhead.
        if (state === 'playing') {
            var ctx = AudioPlayer.getContext();
            var elapsed = ctx.currentTime - startContextTime + pauseOffset;
            var fraction = elapsed / duration;
            if (fraction > 1) fraction = 1;
            drawPlayheadLine(fraction);
        } else if (state === 'paused' && duration > 0) {
            drawPlayheadLine(pauseOffset / duration);
        }
    }

    function formatTime(seconds) {
        if (seconds < 0) seconds = 0;
        var mins = Math.floor(seconds / 60);
        var secs = seconds - mins * 60;
        if (mins > 0) {
            return mins + ':' + (secs < 10 ? '0' : '') + secs.toFixed(2);
        }
        return secs.toFixed(2) + 's';
    }

    function drawPlayheadLine(fraction) {
        if (!overlayCtx || !overlayCanvas) return;
        var w = overlayCanvas.width;
        var h = overlayCanvas.height;
        var x = Math.round(fraction * w);

        overlayCtx.strokeStyle = '#ffffff';
        overlayCtx.lineWidth = 2;
        overlayCtx.beginPath();
        overlayCtx.moveTo(x, 0);
        overlayCtx.lineTo(x, h);
        overlayCtx.stroke();
    }

    function animationLoop() {
        if (state !== 'playing') return;
        drawOverlay();

        var ctx = AudioPlayer.getContext();
        var elapsed = ctx.currentTime - startContextTime + pauseOffset;
        if (elapsed / duration >= 1) return;

        animFrameId = requestAnimationFrame(animationLoop);
    }

    function updateButton(text) {
        var btn = document.getElementById('wav-play-pause-btn');
        if (btn) btn.textContent = text;
    }

    return {
        init: init,
        toggle: toggle,
        stop: stopPlayback,
        seek: seek,
        clearSelection: clearSelection
    };
})();
