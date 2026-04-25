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

    const mutedPads = new Set();
    const soloPads = new Set();

    var visibleBanks = 0; // 0=A only, 1=A+B, 2=A+B+C, 3=all

    var EXTRA_BANKS = [
        { rowClass: '.bank-b-row', sepClass: '.bank-b-sep' },
        { rowClass: '.bank-c-row', sepClass: '.bank-c-sep' },
        { rowClass: '.bank-d-row', sepClass: '.bank-d-sep' },
    ];

    document.addEventListener('htmx:afterSwap', function(evt) {
        var target = evt.detail && evt.detail.target;
        if (!target || target.id !== 'sequence-grid') return;
        var btn = document.getElementById('seq-loop-btn');
        if (btn) btn.classList.toggle('active', looping);
        SequenceEditor.restoreModeButtons();
        restoreVisibleBanks();
    });

    document.addEventListener('DOMContentLoaded', restoreVisibleBanks);

    function restoreVisibleBanks() {
        var stored = parseInt(localStorage.getItem('seq-visible-banks')) || 0;
        visibleBanks = 0;
        for (var i = 0; i < stored && i < 3; i++) {
            var bank = EXTRA_BANKS[i];
            document.querySelectorAll(bank.rowClass + ', ' + bank.sepClass).forEach(function(el) {
                el.style.display = '';
            });
            visibleBanks++;
        }
        var btn = document.getElementById('seq-show-more-btn');
        if (btn) btn.textContent = visibleBanks >= 3 ? 'Show Less' : 'Show More...';
    }

    function showMoreBanks(btn) {
        if (visibleBanks < 3) {
            var bank = EXTRA_BANKS[visibleBanks];
            document.querySelectorAll(bank.rowClass + ', ' + bank.sepClass).forEach(function(el) {
                el.style.display = '';
            });
            visibleBanks++;
        } else {
            EXTRA_BANKS.forEach(function(bank) {
                document.querySelectorAll(bank.rowClass + ', ' + bank.sepClass).forEach(function(el) {
                    el.style.display = 'none';
                });
            });
            visibleBanks = 0;
        }
        localStorage.setItem('seq-visible-banks', String(visibleBanks));
        btn.textContent = visibleBanks >= 3 ? 'Show Less' : 'Show More...';
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

        var ctx = AudioPlayer.getContext();
        startAudioTime = ctx.currentTime + 0.05;
        startWallTime = performance.now();
        scheduledUpTo = startAudioTime;
        currentStep = 0;
        playing = true;

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
        var absStep = Math.floor(elapsed / stepDurationSec);

        if (!looping && absStep >= totalSteps) {
            stop();
            return;
        }

        var step = absStep % totalSteps;
        if (step !== currentStep) {
            currentStep = step;
            highlightStep(currentStep);
        }

        rafHandle = requestAnimationFrame(drawLoop);
    }

    function highlightStep(step) {
        document.querySelectorAll('.step-cell.step-playing').forEach(function(cell) {
            cell.classList.remove('step-playing');
        });
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
        showMoreBanks: showMoreBanks,
        restoreVisibleBanks: restoreVisibleBanks,
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
    var drag = null; // { pad, step, el }
    var dragGhost = null;
    var dragOverCell = null;

    // --- detail popover state ---
    var detailPad = -1;
    var detailStep = -1;

    // ---- helpers ----

    function getGrid() {
        return document.getElementById('seq-step-grid');
    }

    function gridMeta() {
        var g = getGrid();
        if (!g) return null;
        return {
            path: g.dataset.seqPath,
            bar: g.dataset.seqBar,
            pgm: g.dataset.seqPgm || ''
        };
    }

    function postEdit(params) {
        var meta = gridMeta();
        if (!meta) return;
        var body = new URLSearchParams(Object.assign({ path: meta.path, bar: meta.bar, pgm: meta.pgm }, params));
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
                    SequencePlayer.restoreVisibleBanks();
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
        postEdit({ action: 'toggle', pad: pad, step: step, velocity: 100, duration: 23 });
    });

    // ---- edit mode: mouse-drag to move ----

    document.addEventListener('mousedown', function(e) {
        if (mode !== 'edit') return;
        if (e.button !== 0) return;
        var cell = e.target.closest('#seq-step-grid .step-cell');
        if (!cell || !cell.classList.contains('step-active')) return;
        e.preventDefault();

        drag = { pad: parseInt(cell.dataset.pad), step: parseInt(cell.dataset.step), el: cell };
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
        var sourceEl = drag.el;

        // Remove ghost first so it cannot interfere with elementFromPoint below.
        if (dragGhost) { dragGhost.remove(); dragGhost = null; }

        // Always use elementFromPoint at the release position as the source of truth.
        // Relying on dragOverCell risks using a stale cell from when the cursor last
        // passed over it (e.g. A16) rather than where the mouse actually landed.
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
            postEdit({ action: 'move', from_pad: fromPad, from_step: fromStep, to_pad: toPad, to_step: toStep });
        }
    });

    // ---- ctrl/cmd+click or right-click: event detail popover ----

    document.addEventListener('contextmenu', function(e) {
        var cell = e.target.closest('#seq-step-grid .step-cell');
        if (!cell || !cell.classList.contains('step-active')) return;
        e.preventDefault();
        detailPad = parseInt(cell.dataset.pad);
        detailStep = parseInt(cell.dataset.step);
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
        if (detailPad < 0 || detailStep < 0) return;
        var vel = parseInt(document.getElementById('seq-detail-vel').value);
        var dur = parseInt(document.getElementById('seq-detail-dur').value);
        postEdit({ action: 'update', pad: detailPad, step: detailStep, velocity: vel, duration: dur });
        closeDetail();
    }

    function deleteDetail() {
        if (detailPad < 0 || detailStep < 0) return;
        postEdit({ action: 'delete', pad: detailPad, step: detailStep });
        closeDetail();
    }

    function closeDetail() {
        var panel = document.getElementById('seq-event-detail');
        if (panel) panel.style.display = 'none';
        detailPad = -1;
        detailStep = -1;
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

    return {
        setMode: setMode,
        restoreModeButtons: restoreModeButtons,
        saveDetail: saveDetail,
        deleteDetail: deleteDetail,
        closeDetail: closeDetail
    };
})();
