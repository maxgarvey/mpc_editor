// MPC Editor - Sequence step playback preview

const SequencePlayer = (function() {
    var playing = false;
    var looping = false;
    var scheduleTimer = null;
    var rafHandle = null;
    var currentStep = 0;
    var startAudioTime = 0;
    var startWallTime = 0;
    var stepDurationSec = 0;
    var totalSteps = 16;
    var seqEvents = [];
    var seqBpm = 120;
    var scheduledUpTo = 0; // audio time up to which we've scheduled

    var SCHEDULE_AHEAD_SEC = 0.12;
    var LOOKAHEAD_MS = 25;
    var selectedPgm = '';

    // View mode: 'grid' | 'continuous'
    var seqViewMode = 'grid';
    var seqTicksPerStep = 24;
    var seqTotalTicks = 0;
    var PX_PER_TICK = 1.5;
    var TRACK_NAME_W = 140;

    // Smooth playhead line (grid view)
    var playheadEl = null;
    var playheadStepLeft = 0;  // content-area x of step-col-0 left edge
    var playheadStepWidth = 0; // width of one step column

    // Continuous view playhead
    var contPlayheadEl = null;

    function initPlayhead() {
        if (seqViewMode === 'continuous') {
            contPlayheadEl = document.getElementById('seq-cont-playhead');
            if (contPlayheadEl) contPlayheadEl.style.display = 'block';
            playheadEl = null;
            return;
        }
        var scrollEl = document.getElementById('seq-grid-scroll');
        var firstStep = document.querySelector('#seq-step-grid thead th:nth-child(2)');
        playheadEl = document.getElementById('seq-playhead');
        if (!scrollEl || !firstStep || !playheadEl) { playheadEl = null; return; }
        var sr = scrollEl.getBoundingClientRect();
        var fr = firstStep.getBoundingClientRect();
        playheadStepLeft = (fr.left - sr.left) + scrollEl.scrollLeft;
        playheadStepWidth = fr.width;
        playheadEl.style.height = scrollEl.scrollHeight + 'px';
        playheadEl.style.display = 'block';
        contPlayheadEl = null;
    }

    function updatePlayhead(fractionalStep) {
        if (seqViewMode === 'continuous') {
            if (!contPlayheadEl) return;
            var fractionalTick = fractionalStep * seqTicksPerStep;
            var left = TRACK_NAME_W + fractionalTick * PX_PER_TICK;
            contPlayheadEl.style.left = left + 'px';
            // Auto-scroll to keep playhead visible.
            var wrap = document.getElementById('seq-continuous-view');
            if (wrap) {
                var visibleLeft = wrap.scrollLeft + TRACK_NAME_W;
                var visibleRight = wrap.scrollLeft + wrap.clientWidth;
                if (left < visibleLeft || left > visibleRight - 30) {
                    wrap.scrollLeft = Math.max(0, left - TRACK_NAME_W - 60);
                }
            }
            return;
        }
        if (!playheadEl) return;
        playheadEl.style.left = (playheadStepLeft + fractionalStep * playheadStepWidth) + 'px';
    }

    function hidePlayhead() {
        if (playheadEl) { playheadEl.style.display = 'none'; playheadEl = null; }
        if (contPlayheadEl) { contPlayheadEl.style.display = 'none'; contPlayheadEl = null; }
    }

    // ---- continuous view helpers ----

    function velocityToColor(vel) {
        if (vel < 43) return '#4488cc';
        if (vel < 86) return '#44aa44';
        return '#cc4444';
    }

    function padLabel(padIdx) {
        var bank = 'ABCD'[Math.floor(padIdx / 16)];
        var num = (padIdx % 16) + 1;
        return bank + (num < 10 ? '0' : '') + num;
    }

    function addGridline(parent, left, cls) {
        var el = document.createElement('div');
        el.className = 'seq-cont-gridline ' + cls;
        el.style.left = left + 'px';
        parent.appendChild(el);
    }

    function renderContinuousView(data) {
        var container = document.getElementById('seq-continuous-view');
        if (!container) return;

        var events = data.events || [];
        var bars = data.bars || 1;
        var ticksPerStep = data.ticksPerStep || 24;
        var stepsPerBar = data.stepsPerBar || 16;
        var ticksPerBar = ticksPerStep * stepsPerBar; // 384
        seqTotalTicks = data.totalTicks || (bars * ticksPerBar);
        seqTicksPerStep = ticksPerStep;

        var pxpt = PX_PER_TICK;
        var timelineW = Math.ceil(seqTotalTicks * pxpt);
        var TRACK_H = 32;
        var HEADER_H = 28;

        // Group events by padIndex, sorted by padIndex.
        var padEvents = {};
        events.forEach(function(e) {
            if (!padEvents[e.padIndex]) padEvents[e.padIndex] = [];
            padEvents[e.padIndex].push(e);
        });
        var padIndices = Object.keys(padEvents).map(Number).sort(function(a, b) { return a - b; });

        // Build inner container.
        var inner = document.createElement('div');
        inner.className = 'seq-cont-inner';
        inner.style.width = (TRACK_NAME_W + timelineW) + 'px';

        // Header row.
        var header = document.createElement('div');
        header.className = 'seq-cont-header';

        var headerName = document.createElement('div');
        headerName.className = 'seq-cont-header-name';
        header.appendChild(headerName);

        var headerTimeline = document.createElement('div');
        headerTimeline.className = 'seq-cont-header-timeline';
        headerTimeline.style.width = timelineW + 'px';

        for (var b = 0; b < bars; b++) {
            var barTick = b * ticksPerBar;
            var barLabel = document.createElement('span');
            barLabel.className = 'seq-cont-bar-label';
            barLabel.style.left = Math.round(barTick * pxpt) + 'px';
            barLabel.textContent = 'B' + (b + 1);
            headerTimeline.appendChild(barLabel);
            // Beat labels (beats 2, 3, 4).
            for (var beat = 1; beat < 4; beat++) {
                var beatTick = barTick + beat * 96;
                var beatLabel = document.createElement('span');
                beatLabel.className = 'seq-cont-beat-label';
                beatLabel.style.left = Math.round(beatTick * pxpt) + 'px';
                beatLabel.textContent = beat + 1;
                headerTimeline.appendChild(beatLabel);
            }
        }

        header.appendChild(headerTimeline);
        inner.appendChild(header);

        // Tracks container (position: relative for playhead).
        var tracksDiv = document.createElement('div');
        tracksDiv.className = 'seq-cont-tracks';

        padIndices.forEach(function(padIdx) {
            var row = document.createElement('div');
            row.className = 'seq-cont-track';
            row.style.height = TRACK_H + 'px';

            var nameCell = document.createElement('div');
            nameCell.className = 'seq-cont-track-name';
            nameCell.textContent = padLabel(padIdx);
            row.appendChild(nameCell);

            var body = document.createElement('div');
            body.className = 'seq-cont-track-body';
            body.style.width = timelineW + 'px';

            // Gridlines: bars, beats, steps.
            for (var b2 = 0; b2 < bars; b2++) {
                var bt = b2 * ticksPerBar;
                addGridline(body, Math.round(bt * pxpt), 'seq-cont-gridline-bar');
                for (var beat2 = 1; beat2 < 4; beat2++) {
                    addGridline(body, Math.round((bt + beat2 * 96) * pxpt), 'seq-cont-gridline-beat');
                }
                for (var s = 1; s < stepsPerBar; s++) {
                    if (s % 4 !== 0) {
                        addGridline(body, Math.round((bt + s * ticksPerStep) * pxpt), 'seq-cont-gridline-step');
                    }
                }
            }
            // Final bar end line.
            addGridline(body, timelineW, 'seq-cont-gridline-bar');

            // Event blocks.
            padEvents[padIdx].forEach(function(e) {
                var evDiv = document.createElement('div');
                evDiv.className = 'seq-cont-event';
                evDiv.style.left = Math.round(e.tick * pxpt) + 'px';
                evDiv.style.width = Math.max(3, Math.round(e.durationTicks * pxpt)) + 'px';
                evDiv.style.backgroundColor = velocityToColor(e.velocity);
                evDiv.style.opacity = 0.5 + e.velocity / 127 * 0.5;
                evDiv.title = padLabel(padIdx) + ' vel:' + e.velocity;
                body.appendChild(evDiv);
            });

            row.appendChild(body);
            tracksDiv.appendChild(row);
        });

        // Show empty state if no events.
        if (padIndices.length === 0) {
            var empty = document.createElement('div');
            empty.className = 'seq-cont-empty';
            empty.textContent = 'No events in this sequence.';
            tracksDiv.appendChild(empty);
        }

        // Playhead element.
        var ph = document.createElement('div');
        ph.id = 'seq-cont-playhead';
        ph.style.height = Math.max(padIndices.length, 1) * TRACK_H + 'px';
        ph.style.display = playing ? 'block' : 'none';
        tracksDiv.appendChild(ph);

        inner.appendChild(tracksDiv);
        container.innerHTML = '';
        container.appendChild(inner);

        contPlayheadEl = ph;
    }

    function loadContinuousView() {
        var grid = document.getElementById('seq-step-grid');
        if (!grid) return;
        var seqPath = grid.dataset.seqPath;
        if (!seqPath) return;
        var pgmParam = selectedPgm ? '&pgm=' + encodeURIComponent(selectedPgm) : '';
        fetch('/sequence/events?path=' + encodeURIComponent(seqPath) + '&bar=0' + pgmParam)
            .then(function(r) { return r.json(); })
            .then(function(data) { renderContinuousView(data); })
            .catch(function(err) { console.warn('Continuous view load failed:', err); });
    }

    // Restores container visibility and button active state for the current view mode.
    // Does NOT fetch — call refreshEvents() or loadContinuousView() separately for data.
    function restoreViewLayout() {
        var gridScroll = document.getElementById('seq-grid-scroll');
        var contView = document.getElementById('seq-continuous-view');
        if (gridScroll) gridScroll.style.display = (seqViewMode === 'grid') ? '' : 'none';
        if (contView) contView.style.display = (seqViewMode === 'continuous') ? '' : 'none';
        document.querySelectorAll('.seq-view-btn').forEach(function(b) { b.classList.remove('active'); });
        var activeBtn = document.querySelector('.seq-view-btn--' + seqViewMode);
        if (activeBtn) activeBtn.classList.add('active');
    }

    function setViewMode(m) {
        seqViewMode = m;
        restoreViewLayout();
        if (m === 'continuous') loadContinuousView();
        // Re-init playhead for the active view.
        if (playing) initPlayhead();
    }

    const mutedPads = new Set();
    const soloPads = new Set();

    var ALL_BANKS = [
        { letter: 'a', rowClass: '.bank-a-row', sepClass: '.bank-a-sep' },
        { letter: 'b', rowClass: '.bank-b-row', sepClass: '.bank-b-sep' },
        { letter: 'c', rowClass: '.bank-c-row', sepClass: '.bank-c-sep' },
        { letter: 'd', rowClass: '.bank-d-row', sepClass: '.bank-d-sep' },
    ];

    // Set of bank letters currently expanded.
    var expandedBanks = new Set(['a']);

    document.addEventListener('htmx:afterSwap', function(evt) {
        var target = evt.detail && evt.detail.target;
        if (!target || target.id !== 'sequence-grid') return;
        var btn = document.getElementById('seq-loop-btn');
        if (btn) btn.classList.toggle('active', looping);
        SequenceEditor.restoreModeButtons();
        restoreBankState();
        restoreViewLayout();
        refreshEvents();
    });

    document.addEventListener('DOMContentLoaded', function() {
        var stored = localStorage.getItem('seq-expanded-banks');
        if (stored !== null) {
            expandedBanks = new Set(stored ? stored.split(',') : []);
        }
        restoreBankState();
    });

    function saveBankState() {
        localStorage.setItem('seq-expanded-banks', Array.from(expandedBanks).join(','));
    }

    function restoreBankState() {
        ALL_BANKS.forEach(function(bank) {
            var expanded = expandedBanks.has(bank.letter);
            document.querySelectorAll(bank.rowClass).forEach(function(el) {
                el.style.display = expanded ? '' : 'none';
            });
            document.querySelectorAll(bank.sepClass).forEach(function(sep) {
                var arrow = sep.querySelector('.bank-sep-arrow');
                if (arrow) arrow.textContent = expanded ? '▼' : '▶';
                sep.classList.toggle('bank-sep-collapsed', !expanded);
            });
        });
    }

    function toggleBank(letter) {
        if (expandedBanks.has(letter)) {
            expandedBanks.delete(letter);
        } else {
            expandedBanks.add(letter);
        }
        saveBankState();
        restoreBankState();
    }

    function isPadAudible(padIndex) {
        if (soloPads.size > 0) return soloPads.has(padIndex);
        return !mutedPads.has(padIndex);
    }

    function play(seqPath, bar) {
        stop();
        var pgmEl = document.getElementById('seq-pgm-select');
        selectedPgm = pgmEl ? pgmEl.value : '';
        var pgmParam = selectedPgm ? '&pgm=' + encodeURIComponent(selectedPgm) : '';
        fetch('/sequence/events?path=' + encodeURIComponent(seqPath) + '&bar=' + bar + pgmParam)
            .then(function(r) { return r.json(); })
            .then(function(data) {
                var padIndices = [];
                var seen = {};
                (data.events || []).forEach(function(e) {
                    if (!seen[e.padIndex]) { seen[e.padIndex] = true; padIndices.push(e.padIndex); }
                });
                return AudioPlayer.prefetchPadParams(padIndices, selectedPgm).then(function() { return data; });
            })
            .then(function(data) { startPlayback(data); })
            .catch(function(err) { console.warn('Sequence fetch failed:', err); });
    }

    function startPlayback(data) {
        seqBpm = data.bpm || 120;
        seqEvents = data.events || [];
        stepDurationSec = (60 / seqBpm) / 4;
        // When bar=0 (all bars), totalSteps spans the entire sequence.
        totalSteps = (data.stepsPerBar || 16) * (data.bars || 1);
        seqTicksPerStep = data.ticksPerStep || 24;
        seqTotalTicks = data.totalTicks || (totalSteps * seqTicksPerStep);

        var ctx = AudioPlayer.getContext();
        startAudioTime = ctx.currentTime + 0.05;
        startWallTime = performance.now();
        scheduledUpTo = startAudioTime;
        currentStep = 0;
        playing = true;

        initPlayhead();
        scheduler();
        drawLoop();
    }

    function scheduler() {
        if (!playing) return;
        var ctx = AudioPlayer.getContext();
        var scheduleUntil = ctx.currentTime + SCHEDULE_AHEAD_SEC;

        while (scheduledUpTo < scheduleUntil) {
            var absStep = Math.round((scheduledUpTo - startAudioTime) / stepDurationSec);
            if (!looping && absStep >= totalSteps) break;
            var step = absStep % totalSteps;
            var stepTime = startAudioTime + absStep * stepDurationSec;
            if (selectedPgm) {
                seqEvents.forEach(function(e) {
                    if (e.step === step && isPadAudible(e.padIndex)) {
                        AudioPlayer.playPadAtTime(e.padIndex, e.velocity, stepTime, selectedPgm);
                    }
                });
            }
            scheduledUpTo = stepTime + stepDurationSec;
        }

        scheduleTimer = setTimeout(scheduler, LOOKAHEAD_MS);
    }

    function drawLoop() {
        if (!playing) return;
        var ctx = AudioPlayer.getContext();
        var elapsed = ctx.currentTime - startAudioTime;
        var absStepFrac = elapsed / stepDurationSec;
        var absStep = Math.floor(absStepFrac);

        if (!looping && absStep >= totalSteps) {
            stop();
            return;
        }

        var fractionalStep = absStepFrac % totalSteps;
        var step = Math.floor(fractionalStep);

        if (step !== currentStep) {
            currentStep = step;
            highlightStep(currentStep);
        }

        updatePlayhead(fractionalStep);

        rafHandle = requestAnimationFrame(drawLoop);
    }

    function highlightStep(step) {
        document.querySelectorAll('.step-cell.step-playing').forEach(function(cell) {
            cell.classList.remove('step-playing');
        });
        // step-col-N uses the globalStep index; bar-1 steps are 0–15
        document.querySelectorAll('.step-col-' + step).forEach(function(cell) {
            cell.classList.add('step-playing');
        });
    }

    function stop() {
        playing = false;
        if (scheduleTimer) { clearTimeout(scheduleTimer); scheduleTimer = null; }
        if (rafHandle) { cancelAnimationFrame(rafHandle); rafHandle = null; }
        currentStep = 0;
        AudioPlayer.stop();
        document.querySelectorAll('.step-cell.step-playing').forEach(function(cell) {
            cell.classList.remove('step-playing');
        });
        hidePlayhead();
    }

    // Re-fetch events and re-attach playhead after a grid DOM update.
    // Called when playing (to update seqEvents for live edits) or in continuous mode.
    function refreshEvents() {
        if (playing) initPlayhead();
        if (!playing && seqViewMode !== 'continuous') return;
        var grid = document.getElementById('seq-step-grid');
        if (!grid) return;
        var seqPath = grid.dataset.seqPath;
        if (!seqPath) return;
        var pgmParam = selectedPgm ? '&pgm=' + encodeURIComponent(selectedPgm) : '';
        fetch('/sequence/events?path=' + encodeURIComponent(seqPath) + '&bar=0' + pgmParam)
            .then(function(r) { return r.json(); })
            .then(function(data) {
                seqEvents = data.events || [];
                totalSteps = (data.stepsPerBar || 16) * (data.bars || 1);
                seqTicksPerStep = data.ticksPerStep || 24;
                seqTotalTicks = data.totalTicks || (totalSteps * seqTicksPerStep);
                if (seqViewMode === 'continuous') renderContinuousView(data);
            })
            .catch(function(err) { console.warn('Event refresh failed:', err); });
    }

    function toggleMutePad(padIndex, btn) {
        if (mutedPads.has(padIndex)) {
            mutedPads.delete(padIndex);
            btn.classList.remove('active');
            btn.closest('tr').classList.remove('track-muted');
        } else {
            mutedPads.add(padIndex);
            btn.classList.add('active');
            btn.closest('tr').classList.add('track-muted');
        }
    }

    function toggleSoloPad(padIndex, btn) {
        if (soloPads.has(padIndex)) {
            soloPads.delete(padIndex);
            btn.classList.remove('active');
        } else {
            soloPads.add(padIndex);
            btn.classList.add('active');
        }
        document.querySelectorAll('.step-grid tbody tr.pad-row').forEach(function(row) {
            var idx = parseInt(row.getAttribute('data-pad'));
            if (soloPads.size > 0 && !soloPads.has(idx)) {
                row.classList.add('track-muted');
            } else if (!mutedPads.has(idx)) {
                row.classList.remove('track-muted');
            }
        });
    }

    return {
        play: play,
        stop: stop,
        isPlaying: function() { return playing; },
        toggleMutePad: toggleMutePad,
        toggleSoloPad: toggleSoloPad,
        toggleBank: toggleBank,
        restoreBankState: restoreBankState,
        refreshEvents: refreshEvents,
        setViewMode: setViewMode,
        restoreViewLayout: restoreViewLayout,
        toggleLoop: function(btn) {
            looping = !looping;
            btn.classList.toggle('active', looping);
        }
    };
})();

// MPC Editor - Sequence step editor (insert / edit modes + event detail)

const SequenceEditor = (function() {
    var mode = 'view'; // 'view' | 'insert' | 'edit'

    // --- drag state ---
    var drag = null; // { pad, step, bar, el }
    var dragGhost = null;
    var dragOverCell = null;

    // --- detail popover state ---
    var detailPad = -1;
    var detailStep = -1;
    var detailBar = -1;

    // ---- helpers ----

    function getGrid() {
        return document.getElementById('seq-step-grid');
    }

    function gridMeta() {
        var g = getGrid();
        if (!g) return null;
        return {
            path: g.dataset.seqPath,
            pgm: g.dataset.seqPgm || ''
        };
    }

    function postEdit(params) {
        var meta = gridMeta();
        if (!meta) return;
        var body = new URLSearchParams(Object.assign({ path: meta.path, pgm: meta.pgm }, params));
        fetch('/sequence/event/edit', { method: 'POST', body: body })
            .then(function(r) {
                if (!r.ok) return r.text().then(function(t) { console.error('seq edit error:', t); });
                return r.text();
            })
            .then(function(html) {
                if (!html) return;
                var grid = document.getElementById('sequence-grid');
                if (grid) {
                    grid.innerHTML = html;
                    if (window.htmx) htmx.process(grid);
                    SequenceEditor.restoreModeButtons();
                    SequencePlayer.restoreBankState();
                    SequencePlayer.restoreViewLayout();
                    SequencePlayer.refreshEvents();
                }
            })
            .catch(function(err) { console.error('seq edit fetch failed:', err); });
    }

    // ---- mode management ----

    function setMode(m, btn) {
        mode = m;
        restoreModeButtons();
        var grid = getGrid();
        if (grid) {
            grid.setAttribute('data-seq-mode', m);
        }
    }

    function restoreModeButtons() {
        document.querySelectorAll('.seq-mode-btn').forEach(function(b) { b.classList.remove('active'); });
        var active = document.querySelector('.seq-mode-btn--' + mode);
        if (active) active.classList.add('active');
        var grid = getGrid();
        if (grid) grid.setAttribute('data-seq-mode', mode);
    }

    // ---- insert mode: click to toggle ----

    document.addEventListener('click', function(e) {
        if (mode !== 'insert') return;
        var cell = e.target.closest('#seq-step-grid .step-cell');
        if (!cell) return;
        if (drag) return; // ignore clicks that follow a drag
        var pad = parseInt(cell.dataset.pad);
        var step = parseInt(cell.dataset.step);
        var bar = parseInt(cell.dataset.bar) || 1;
        postEdit({ action: 'toggle', pad: pad, step: step, bar: bar, velocity: 100, duration: 23 });
    });

    // ---- edit mode: mouse-drag to move ----

    document.addEventListener('mousedown', function(e) {
        if (mode !== 'edit') return;
        if (e.button !== 0) return;
        var cell = e.target.closest('#seq-step-grid .step-cell');
        if (!cell || !cell.classList.contains('step-active')) return;
        e.preventDefault();

        drag = {
            pad: parseInt(cell.dataset.pad),
            step: parseInt(cell.dataset.step),
            bar: parseInt(cell.dataset.bar) || 1,
            el: cell
        };
        cell.classList.add('step-dragging');

        dragGhost = document.createElement('div');
        dragGhost.className = 'step-drag-ghost';
        // Offset ghost from cursor so it never sits on top of the cursor position.
        // This keeps elementFromPoint reliable without depending on pointer-events:none.
        dragGhost.style.left = (e.clientX + 14) + 'px';
        dragGhost.style.top = (e.clientY - 14) + 'px';
        document.body.appendChild(dragGhost);
    });

    document.addEventListener('mousemove', function(e) {
        if (!drag) return;
        // Move ghost with the same offset so it tracks the cursor.
        dragGhost.style.left = (e.clientX + 14) + 'px';
        dragGhost.style.top = (e.clientY - 14) + 'px';

        // Cursor is never under the ghost, so elementFromPoint is reliable here.
        var el = document.elementFromPoint(e.clientX, e.clientY);
        var cell = el && el.closest('#seq-step-grid .step-cell');
        var next = (cell && cell !== drag.el) ? cell : null;
        if (next !== dragOverCell) {
            if (dragOverCell) dragOverCell.classList.remove('step-drop-target');
            dragOverCell = next;
            if (dragOverCell) dragOverCell.classList.add('step-drop-target');
        }
    });

    document.addEventListener('mouseup', function(e) {
        if (!drag) return;

        var fromPad = drag.pad;
        var fromStep = drag.step;
        var fromBar = drag.bar;
        var sourceEl = drag.el;

        // Remove ghost first so it cannot interfere with elementFromPoint below.
        if (dragGhost) { dragGhost.remove(); dragGhost = null; }

        // Always use elementFromPoint at the release position as the source of truth.
        var target = null;
        var el = document.elementFromPoint(e.clientX, e.clientY);
        if (el) {
            var candidate = el.closest('#seq-step-grid .step-cell');
            if (candidate && candidate !== sourceEl) target = candidate;
        }

        // Cleanup visual state.
        sourceEl.classList.remove('step-dragging');
        if (dragOverCell) { dragOverCell.classList.remove('step-drop-target'); }
        dragOverCell = null;
        drag = null;

        if (target) {
            var toPad = parseInt(target.dataset.pad);
            var toStep = parseInt(target.dataset.step);
            var toBar = parseInt(target.dataset.bar) || 1;
            postEdit({
                action: 'move',
                from_pad: fromPad, from_step: fromStep, from_bar: fromBar,
                to_pad: toPad, to_step: toStep, to_bar: toBar
            });
        }
    });

    // ---- ctrl/cmd+click or right-click: event detail popover ----

    document.addEventListener('contextmenu', function(e) {
        var cell = e.target.closest('#seq-step-grid .step-cell');
        if (!cell || !cell.classList.contains('step-active')) return;
        e.preventDefault();
        detailPad = parseInt(cell.dataset.pad);
        detailStep = parseInt(cell.dataset.step);
        detailBar = parseInt(cell.dataset.bar) || 1;
        var vel = parseInt(cell.dataset.vel) || 100;
        var dur = parseInt(cell.dataset.dur) || 23;
        openDetail(e.clientX, e.clientY, vel, dur);
    });

    function openDetail(x, y, vel, dur) {
        var panel = document.getElementById('seq-event-detail');
        if (!panel) return;
        document.getElementById('seq-detail-vel').value = vel;
        document.getElementById('seq-detail-vel-display').textContent = vel;
        document.getElementById('seq-detail-dur').value = dur;

        // Position near click, but keep on screen.
        var panelW = 240, panelH = 130;
        var vw = window.innerWidth, vh = window.innerHeight;
        var left = Math.min(x + 8, vw - panelW - 8);
        var top = Math.min(y + 8, vh - panelH - 8);
        panel.style.left = left + 'px';
        panel.style.top = top + 'px';
        panel.style.display = 'block';
    }

    function saveDetail() {
        if (detailPad < 0 || detailStep < 0 || detailBar < 0) return;
        var vel = parseInt(document.getElementById('seq-detail-vel').value);
        var dur = parseInt(document.getElementById('seq-detail-dur').value);
        postEdit({ action: 'update', pad: detailPad, step: detailStep, bar: detailBar, velocity: vel, duration: dur });
        closeDetail();
    }

    function deleteDetail() {
        if (detailPad < 0 || detailStep < 0 || detailBar < 0) return;
        postEdit({ action: 'delete', pad: detailPad, step: detailStep, bar: detailBar });
        closeDetail();
    }

    function closeDetail() {
        var panel = document.getElementById('seq-event-detail');
        if (panel) panel.style.display = 'none';
        detailPad = -1;
        detailStep = -1;
        detailBar = -1;
    }

    // Close detail on click outside.
    document.addEventListener('mousedown', function(e) {
        var panel = document.getElementById('seq-event-detail');
        if (panel && panel.style.display !== 'none' && !panel.contains(e.target)) {
            closeDetail();
        }
    });

    // Close detail on Escape.
    document.addEventListener('keydown', function(e) {
        if (e.key === 'Escape') closeDetail();
    });

    // ---- pad / step preview ----

    function previewPad(padIndex) {
        var meta = gridMeta();
        var pgm = meta ? meta.pgm : '';
        AudioPlayer.stop();
        var ctx = AudioPlayer.getContext();
        AudioPlayer.playPadAtTime(padIndex, 100, ctx.currentTime + 0.01, pgm);
    }

    function previewStep(globalStep) {
        var meta = gridMeta();
        var pgm = meta ? meta.pgm : '';
        AudioPlayer.stop();
        var ctx = AudioPlayer.getContext();
        var atTime = ctx.currentTime + 0.01;
        document.querySelectorAll('.step-col-' + globalStep + '.step-active').forEach(function(cell) {
            var padIdx = parseInt(cell.dataset.pad);
            var vel = parseInt(cell.dataset.vel) || 100;
            if (!isNaN(padIdx)) {
                AudioPlayer.playPadAtTime(padIdx, vel, atTime, pgm);
            }
        });
    }

    // View mode: click active step to preview all events at that step.
    document.addEventListener('click', function(e) {
        if (mode !== 'view') return;
        var cell = e.target.closest('#seq-step-grid .step-cell');
        if (!cell || !cell.classList.contains('step-active')) return;
        var globalStep = null;
        cell.classList.forEach(function(cls) {
            var m = cls.match(/^step-col-(\d+)$/);
            if (m) globalStep = parseInt(m[1]);
        });
        if (globalStep !== null) previewStep(globalStep);
    });

    return {
        setMode: setMode,
        restoreModeButtons: restoreModeButtons,
        saveDetail: saveDetail,
        deleteDetail: deleteDetail,
        closeDetail: closeDetail,
        previewPad: previewPad,
        previewStep: previewStep
    };
})();
