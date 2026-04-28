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
        var beatsPerBar = data.beatsPerBar || 4;
        var ticksPerBar = ticksPerStep * stepsPerBar;
        var ticksPerBeat = Math.round(ticksPerBar / beatsPerBar);
        var stepsPerBeat = Math.round(stepsPerBar / beatsPerBar);
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

        // ── Header: beats row + seconds row ───────────────────────────────────
        var header = document.createElement('div');
        header.className = 'seq-cont-header';

        var headerName = document.createElement('div');
        headerName.className = 'seq-cont-header-name';
        header.appendChild(headerName);

        var rulers = document.createElement('div');
        rulers.className = 'seq-cont-rulers';
        rulers.style.width = timelineW + 'px';

        // Row 1: beat/bar ruler — tick + label at every beat
        var beatsRow = document.createElement('div');
        beatsRow.className = 'seq-cont-beats-row';
        for (var bi = 0; bi < bars; bi++) {
            var biBarTk = bi * ticksPerBar;
            var biBarPx = Math.round(biBarTk * pxpt);
            var barMark = document.createElement('div');
            barMark.style.cssText = 'position:absolute;top:0;bottom:0;left:' + biBarPx + 'px;pointer-events:none;';
            var barTickEl = document.createElement('div');
            barTickEl.style.cssText = 'position:absolute;bottom:0;left:0;width:1px;height:12px;background:rgba(255,255,255,0.5);';
            barMark.appendChild(barTickEl);
            var barLbl = document.createElement('span');
            barLbl.style.cssText = 'position:absolute;top:3px;left:3px;font-size:10px;font-weight:bold;color:#bbb;white-space:nowrap;line-height:1;';
            barLbl.textContent = 'B' + (bi + 1);
            barMark.appendChild(barLbl);
            beatsRow.appendChild(barMark);

            for (var bii = 1; bii < beatsPerBar; bii++) {
                var biiBeatTk = biBarTk + bii * ticksPerBeat;
                var biiBeatPx = Math.round(biiBeatTk * pxpt);
                var beatMark = document.createElement('div');
                beatMark.style.cssText = 'position:absolute;top:0;bottom:0;left:' + biiBeatPx + 'px;pointer-events:none;';
                var beatTickEl = document.createElement('div');
                beatTickEl.style.cssText = 'position:absolute;bottom:0;left:0;width:1px;height:7px;background:rgba(255,255,255,0.25);';
                beatMark.appendChild(beatTickEl);
                var beatLbl = document.createElement('span');
                beatLbl.style.cssText = 'position:absolute;top:4px;left:2px;font-size:9px;color:#777;white-space:nowrap;line-height:1;';
                beatLbl.textContent = bii + 1;
                beatMark.appendChild(beatLbl);
                beatsRow.appendChild(beatMark);
            }
        }
        rulers.appendChild(beatsRow);

        // Row 2: seconds ruler — derived from BPM
        var bpmVal = data.bpm || 120;
        var tksPerSec = bpmVal * 96 / 60;
        var seqDurSec = seqTotalTicks / tksPerSec;
        var pxPerSec = tksPerSec * pxpt;

        var timeRow = document.createElement('div');
        timeRow.className = 'seq-cont-time-row';

        // Adaptive density: widen major interval when zoomed out
        var majSec = 1, minSec = 0.5;
        if (pxPerSec < 40) { majSec = 10; minSec = 5; }
        else if (pxPerSec < 80) { majSec = 5; minSec = 1; }
        else if (pxPerSec < 160) { majSec = 2; minSec = 0.5; }

        // Minor ticks (no label)
        if (minSec * pxPerSec >= 18) {
            var nMinor = Math.ceil(seqDurSec / minSec) + 1;
            for (var mni = 1; mni <= nMinor; mni++) {
                var mnt = mni * minSec;
                if (mnt % majSec < 0.001 || mnt % majSec > majSec - 0.001) continue;
                var mnPx = Math.round(mnt * tksPerSec * pxpt);
                if (mnPx > timelineW) break;
                var mnMark = document.createElement('div');
                mnMark.style.cssText = 'position:absolute;top:0;left:' + mnPx + 'px;width:1px;height:4px;background:rgba(90,160,220,0.3);pointer-events:none;';
                timeRow.appendChild(mnMark);
            }
        }

        // Major ticks + labels
        var nMaj = Math.ceil(seqDurSec / majSec) + 1;
        for (var mji = 0; mji <= nMaj; mji++) {
            var mjt = mji * majSec;
            var mjPx = Math.round(mjt * tksPerSec * pxpt);
            if (mjPx > timelineW + 2) break;
            var mjMark = document.createElement('div');
            mjMark.style.cssText = 'position:absolute;top:0;bottom:0;left:' + mjPx + 'px;pointer-events:none;';
            var mjTick = document.createElement('div');
            mjTick.style.cssText = 'position:absolute;top:0;left:0;width:1px;height:7px;background:rgba(90,160,220,0.6);';
            mjMark.appendChild(mjTick);
            var mjLbl = document.createElement('span');
            mjLbl.style.cssText = 'position:absolute;top:8px;left:2px;font-size:9px;color:#5a9fd4;white-space:nowrap;line-height:1;';
            mjLbl.textContent = mjt === 0 ? '0s' : (majSec >= 1 ? Math.round(mjt) + 's' : mjt.toFixed(1) + 's');
            mjMark.appendChild(mjLbl);
            timeRow.appendChild(mjMark);
        }

        rulers.appendChild(timeRow);
        header.appendChild(rulers);
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
                for (var beat2 = 1; beat2 < beatsPerBar; beat2++) {
                    addGridline(body, Math.round((bt + beat2 * ticksPerBeat) * pxpt), 'seq-cont-gridline-beat');
                }
                for (var s = 1; s < stepsPerBar; s++) {
                    if (s % stepsPerBeat !== 0) {
                        addGridline(body, Math.round((bt + s * ticksPerStep) * pxpt), 'seq-cont-gridline-step');
                    }
                }
            }
            // Final bar end line.
            addGridline(body, timelineW, 'seq-cont-gridline-bar');
            // Second-aligned gridlines (subtle blue tint, distinct from beat/bar).
            var nSecLines = Math.ceil(seqTotalTicks / tksPerSec) + 1;
            for (var scl = 0; scl <= nSecLines; scl++) {
                var sclPx = Math.round(scl * tksPerSec * pxpt);
                if (sclPx > timelineW) break;
                addGridline(body, sclPx, 'seq-cont-gridline-sec');
            }

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

    function getDisplayParams() {
        var grid = document.getElementById('seq-step-grid');
        if (!grid) return '';
        var tsig = grid.dataset.seqTsig || '4_4';
        var div = grid.dataset.seqDivision || '24';
        return '&tsig=' + encodeURIComponent(tsig) + '&division=' + encodeURIComponent(div);
    }

    function loadContinuousView() {
        var grid = document.getElementById('seq-step-grid');
        if (!grid) return;
        var seqPath = grid.dataset.seqPath;
        if (!seqPath) return;
        var pgmParam = selectedPgm ? '&pgm=' + encodeURIComponent(selectedPgm) : '';
        fetch('/sequence/events?path=' + encodeURIComponent(seqPath) + '&bar=0' + pgmParam + getDisplayParams())
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
        { letter: 'a', rowClass: '.bank-a-row' },
        { letter: 'b', rowClass: '.bank-b-row' },
        { letter: 'c', rowClass: '.bank-c-row' },
        { letter: 'd', rowClass: '.bank-d-row' },
    ];

    // Set of bank letters currently expanded.
    var expandedBanks = new Set(['a']);

    function syncLoopFromDOM() {
        var grid = document.getElementById('seq-step-grid');
        if (grid && grid.dataset.seqLoop !== undefined) {
            looping = grid.dataset.seqLoop === 'true';
        }
        var btn = document.getElementById('seq-loop-btn');
        if (btn) btn.classList.toggle('active', looping);
    }

    document.addEventListener('htmx:afterSwap', function(evt) {
        if (!evt.target || !evt.target.closest('#sequence-grid')) return;
        SequenceEditor.clearSelection();
        syncLoopFromDOM();
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
        syncLoopFromDOM();
    });

    function saveBankState() {
        localStorage.setItem('seq-expanded-banks', Array.from(expandedBanks).join(','));
    }

    function restoreBankState() {
        ALL_BANKS.forEach(function(bank) {
            var expanded = expandedBanks.has(bank.letter);
            var rows = Array.from(document.querySelectorAll(bank.rowClass));
            rows.forEach(function(el, idx) {
                if (idx === 0) {
                    // First row is always visible — it acts as the bank header.
                    el.style.display = '';
                    var arrow = el.querySelector('.bank-sep-arrow');
                    if (arrow) arrow.textContent = expanded ? '▼' : '▶';
                } else {
                    el.style.display = expanded ? '' : 'none';
                }
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
        fetch('/sequence/events?path=' + encodeURIComponent(seqPath) + '&bar=' + bar + pgmParam + getDisplayParams())
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
        seqTicksPerStep = data.ticksPerStep || 24;
        stepDurationSec = (60 / seqBpm) * (seqTicksPerStep / 96);
        // When bar=0 (all bars), totalSteps spans the entire sequence.
        totalSteps = (data.stepsPerBar || 16) * (data.bars || 1);
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
        fetch('/sequence/events?path=' + encodeURIComponent(seqPath) + '&bar=0' + pgmParam + getDisplayParams())
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
        syncLoop: syncLoopFromDOM,
        toggleLoop: function() {
            var grid = document.getElementById('seq-step-grid');
            if (!grid) return;
            var path = grid.dataset.seqPath;
            if (!path) return;
            var body = new URLSearchParams({ path: path });
            fetch('/sequence/toggle-loop', { method: 'POST', body: body })
                .then(function(r) { return r.json(); })
                .then(function(data) {
                    looping = !!data.loop;
                    grid.dataset.seqLoop = looping ? 'true' : 'false';
                    var btn = document.getElementById('seq-loop-btn');
                    if (btn) btn.classList.toggle('active', looping);
                })
                .catch(function(err) { console.warn('toggle loop failed:', err); });
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

    // --- multi-select state ---
    var selectedCells = new Set(); // "pad:globalStep" keys
    var detailIsMulti = false;

    // --- detail popover state ---
    var detailPad = -1;
    var detailStep = -1;
    var detailBar = -1;

    // ---- helpers ----

    function cellKey(pad, gs) { return pad + ':' + gs; }

    function getGlobalStep(cell) {
        for (var i = 0; i < cell.classList.length; i++) {
            var m = cell.classList[i].match(/^step-col-(\d+)$/);
            if (m) return parseInt(m[1]);
        }
        return -1;
    }

    function isInSelection(cell) {
        var pad = parseInt(cell.dataset.pad);
        var gs = getGlobalStep(cell);
        return gs >= 0 && selectedCells.has(cellKey(pad, gs));
    }

    function addToSelection(cell) {
        var pad = parseInt(cell.dataset.pad);
        var gs = getGlobalStep(cell);
        if (gs < 0) return;
        selectedCells.add(cellKey(pad, gs));
        cell.classList.add('step-selected');
    }

    function removeFromSelection(cell) {
        var pad = parseInt(cell.dataset.pad);
        var gs = getGlobalStep(cell);
        selectedCells.delete(cellKey(pad, gs));
        cell.classList.remove('step-selected');
    }

    function clearSelection() {
        selectedCells.clear();
        document.querySelectorAll('#seq-step-grid .step-selected').forEach(function(el) {
            el.classList.remove('step-selected');
        });
    }

    function getSelectedEvents() {
        var result = [];
        document.querySelectorAll('#seq-step-grid .step-cell.step-selected').forEach(function(cell) {
            var pad = parseInt(cell.dataset.pad);
            var gs = getGlobalStep(cell);
            if (pad >= 0 && gs >= 0) result.push({ pad: pad, global_step: gs });
        });
        return result;
    }

    function getGrid() {
        return document.getElementById('seq-step-grid');
    }

    function gridMeta() {
        var g = getGrid();
        if (!g) return null;
        return {
            path: g.dataset.seqPath,
            pgm: g.dataset.seqPgm || '',
            tsig: g.dataset.seqTsig || '4_4',
            division: g.dataset.seqDivision || '24'
        };
    }

    function postEdit(params) {
        var meta = gridMeta();
        if (!meta) return;
        var body = new URLSearchParams(Object.assign({ path: meta.path, pgm: meta.pgm, tsig: meta.tsig, division: meta.division }, params));
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
                    clearSelection();
                    SequenceEditor.restoreModeButtons();
                    SequencePlayer.restoreBankState();
                    SequencePlayer.restoreViewLayout();
                    SequencePlayer.syncLoop();
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
        if (e.ctrlKey || e.metaKey) {
            if (cell.classList.contains('step-active')) {
                if (isInSelection(cell)) { removeFromSelection(cell); } else { addToSelection(cell); }
            }
            return;
        }
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

        if (e.ctrlKey || e.metaKey) {
            if (isInSelection(cell)) { removeFromSelection(cell); } else { addToSelection(cell); }
            return;
        }

        e.preventDefault();
        var isMulti = selectedCells.size > 1 && isInSelection(cell);

        drag = {
            pad: parseInt(cell.dataset.pad),
            step: parseInt(cell.dataset.step),
            bar: parseInt(cell.dataset.bar) || 1,
            globalStep: getGlobalStep(cell),
            isMulti: isMulti,
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
        var fromGs = drag.globalStep;
        var isMulti = drag.isMulti;
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
            if (isMulti) {
                var toGs = getGlobalStep(target);
                var toMultiPad = parseInt(target.dataset.pad);
                var stepDelta = toGs - fromGs;
                var padDelta = toMultiPad - fromPad;
                var events = getSelectedEvents().map(function(ev) {
                    return {
                        pad: ev.pad,
                        global_step: ev.global_step,
                        to_pad: Math.max(0, Math.min(63, ev.pad + padDelta)),
                        to_global_step: Math.max(0, ev.global_step + stepDelta)
                    };
                });
                postEdit({ action: 'multi_move', events: JSON.stringify(events) });
            } else {
                var toPad = parseInt(target.dataset.pad);
                var toStep = parseInt(target.dataset.step);
                var toBar = parseInt(target.dataset.bar) || 1;
                postEdit({
                    action: 'move',
                    from_pad: fromPad, from_step: fromStep, from_bar: fromBar,
                    to_pad: toPad, to_step: toStep, to_bar: toBar
                });
            }
        }
    });

    // ---- right-click: event detail popover (single or multi) ----

    document.addEventListener('contextmenu', function(e) {
        var cell = e.target.closest('#seq-step-grid .step-cell');
        if (!cell || !cell.classList.contains('step-active')) return;
        e.preventDefault();

        if (selectedCells.size > 0 && !isInSelection(cell)) {
            // Switch selection to just this cell
            clearSelection();
            addToSelection(cell);
        }

        if (selectedCells.size > 1 && isInSelection(cell)) {
            // Bulk edit
            detailPad = -1; detailStep = -1; detailBar = -1;
            openDetail(e.clientX, e.clientY, 100, 23, true);
        } else {
            detailPad = parseInt(cell.dataset.pad);
            detailStep = parseInt(cell.dataset.step);
            detailBar = parseInt(cell.dataset.bar) || 1;
            var vel = parseInt(cell.dataset.vel) || 100;
            var dur = parseInt(cell.dataset.dur) || 23;
            openDetail(e.clientX, e.clientY, vel, dur, false);
        }
    });

    function openDetail(x, y, vel, dur, multi) {
        var panel = document.getElementById('seq-event-detail');
        if (!panel) return;
        detailIsMulti = !!multi;
        document.getElementById('seq-detail-vel').value = vel;
        document.getElementById('seq-detail-vel-display').textContent = vel;
        document.getElementById('seq-detail-dur').value = dur;
        var title = panel.querySelector('.seq-event-detail-title');
        if (title) {
            title.textContent = multi ? 'Bulk Edit (' + selectedCells.size + ' events)' : 'Event Details';
        }

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
        var vel = parseInt(document.getElementById('seq-detail-vel').value);
        var dur = parseInt(document.getElementById('seq-detail-dur').value);
        if (detailIsMulti) {
            var evs = getSelectedEvents();
            if (evs.length > 0) postEdit({ action: 'multi_update', events: JSON.stringify(evs), velocity: vel, duration: dur });
            closeDetail();
            return;
        }
        if (detailPad < 0 || detailStep < 0 || detailBar < 0) return;
        postEdit({ action: 'update', pad: detailPad, step: detailStep, bar: detailBar, velocity: vel, duration: dur });
        closeDetail();
    }

    function deleteDetail() {
        if (detailIsMulti) {
            var evs = getSelectedEvents();
            if (evs.length > 0) postEdit({ action: 'multi_delete', events: JSON.stringify(evs) });
            closeDetail();
            return;
        }
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
        detailIsMulti = false;
    }

    // Close detail on click outside; clear selection on click outside the grid.
    document.addEventListener('mousedown', function(e) {
        var panel = document.getElementById('seq-event-detail');
        if (panel && panel.style.display !== 'none' && !panel.contains(e.target)) {
            closeDetail();
        }
        if (!e.target.closest('#seq-step-grid') && !e.target.closest('#seq-event-detail')) {
            clearSelection();
        }
    });

    // Close detail on Escape; Delete selected events on Delete key.
    document.addEventListener('keydown', function(e) {
        if (e.key === 'Escape') { closeDetail(); return; }
        if (e.key === 'Delete') {
            var panel = document.getElementById('seq-event-detail');
            if (panel && panel.style.display !== 'none') return; // let the panel handle it
            var tag = document.activeElement && document.activeElement.tagName;
            if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;
            if (selectedCells.size > 0) {
                e.preventDefault();
                var evs = getSelectedEvents();
                if (evs.length > 0) postEdit({ action: 'multi_delete', events: JSON.stringify(evs) });
            }
        }
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

    // View mode: click to preview; ctrl+click to select.
    document.addEventListener('click', function(e) {
        if (mode !== 'view') return;
        var cell = e.target.closest('#seq-step-grid .step-cell');
        if (!cell) return;

        if (e.ctrlKey || e.metaKey) {
            if (cell.classList.contains('step-active')) {
                if (isInSelection(cell)) { removeFromSelection(cell); } else { addToSelection(cell); }
            }
            return;
        }

        clearSelection();
        if (!cell.classList.contains('step-active')) return;
        var globalStep = getGlobalStep(cell);
        if (globalStep >= 0) previewStep(globalStep);
    });

    return {
        setMode: setMode,
        restoreModeButtons: restoreModeButtons,
        clearSelection: clearSelection,
        saveDetail: saveDetail,
        deleteDetail: deleteDetail,
        closeDetail: closeDetail,
        previewPad: previewPad,
        previewStep: previewStep
    };
})();
