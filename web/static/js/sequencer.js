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
    var STEP_PX = 36;      // pixels per step — constant in both views; piano roll derives PX_PER_TICK from this
    var PX_PER_TICK = 1.5; // updated per render: STEP_PX / ticksPerStep
    var TRACK_NAME_W = 140;
    var TRACK_H = 32;
    var BANK_HEADER_H = 24;

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
        var firstStep = document.querySelector('#seq-step-grid .step-col-0');
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

    // Builds the three shared ruler rows (beats, divisions, seconds) into a wrapper div.
    // p: { bars, beatsPerBar, stepsPerBar, pxPerStep, bpm }
    // Returns { el: wrapper div, totalPx: number }
    function buildRulerContent(p) {
        var stepsPerBeat = p.stepsPerBar / p.beatsPerBar;
        var totalSteps = p.bars * p.stepsPerBar;
        var totalPx = Math.ceil(totalSteps * p.pxPerStep);

        var wrapper = document.createElement('div');

        // Row 1: bars and beats
        var beatsRow = document.createElement('div');
        beatsRow.className = 'seq-ruler-beats-row';
        for (var bi = 0; bi < p.bars; bi++) {
            var barPx = Math.round(bi * p.stepsPerBar * p.pxPerStep);
            var barMark = document.createElement('div');
            barMark.className = 'seq-ruler-mark-bar';
            barMark.style.left = barPx + 'px';
            barMark.textContent = 'B' + (bi + 1);
            beatsRow.appendChild(barMark);
            for (var beat = 1; beat < p.beatsPerBar; beat++) {
                var beatPx = Math.round((bi * p.stepsPerBar + beat * stepsPerBeat) * p.pxPerStep);
                var beatMark = document.createElement('div');
                beatMark.className = 'seq-ruler-mark-beat';
                beatMark.style.left = beatPx + 'px';
                beatMark.textContent = beat + 1;
                beatsRow.appendChild(beatMark);
            }
        }
        wrapper.appendChild(beatsRow);

        // Row 2: step divisions
        var divsRow = document.createElement('div');
        divsRow.className = 'seq-ruler-div-row';
        for (var si = 0; si <= totalSteps; si++) {
            var sPx = Math.round(si * p.pxPerStep);
            var isBar = (si % p.stepsPerBar === 0);
            var isBeat = !isBar && stepsPerBeat > 0 && (si % stepsPerBeat === 0);
            var divMark = document.createElement('div');
            divMark.className = 'seq-ruler-div-mark' + (isBar ? ' bar' : isBeat ? ' beat' : '');
            divMark.style.left = sPx + 'px';
            divsRow.appendChild(divMark);
        }
        wrapper.appendChild(divsRow);

        // Row 3: seconds
        var totalSec = p.bars * p.beatsPerBar * 60 / p.bpm;
        var pxPerSec = totalPx / totalSec;
        var majSec = 1, minSec = 0.5;
        if (pxPerSec < 40)  { majSec = 10; minSec = 5; }
        else if (pxPerSec < 80)  { majSec = 5;  minSec = 1; }
        else if (pxPerSec < 160) { majSec = 2;  minSec = 0.5; }

        var secRow = document.createElement('div');
        secRow.className = 'seq-ruler-sec-row';

        if (minSec * pxPerSec >= 18) {
            var nMinor = Math.ceil(totalSec / minSec) + 1;
            for (var mni = 1; mni <= nMinor; mni++) {
                var mnt = mni * minSec;
                if (mnt % majSec < 0.001 || mnt % majSec > majSec - 0.001) continue;
                var mnPx = Math.round(mnt * pxPerSec);
                if (mnPx > totalPx) break;
                var mnEl = document.createElement('div');
                mnEl.className = 'seq-ruler-sec-minor';
                mnEl.style.left = mnPx + 'px';
                secRow.appendChild(mnEl);
            }
        }
        var nMaj = Math.ceil(totalSec / majSec) + 1;
        for (var mji = 0; mji <= nMaj; mji++) {
            var mjt = mji * majSec;
            var mjPx = Math.round(mjt * pxPerSec);
            if (mjPx > totalPx + 2) break;
            var mjEl = document.createElement('div');
            mjEl.className = 'seq-ruler-sec-major';
            mjEl.style.left = mjPx + 'px';
            var mjLbl = document.createElement('span');
            mjLbl.className = 'seq-ruler-sec-label';
            mjLbl.textContent = mjt === 0 ? '0s' : (majSec >= 1 ? Math.round(mjt) + 's' : mjt.toFixed(1) + 's');
            mjEl.appendChild(mjLbl);
            secRow.appendChild(mjEl);
        }
        wrapper.appendChild(secRow);

        return { el: wrapper, totalPx: totalPx };
    }

    // Renders the three-row ruler above the grid table using data attributes stored on the table element.
    function renderGridRuler() {
        var grid = document.getElementById('seq-step-grid');
        var rulerWrap = document.getElementById('seq-grid-ruler');
        if (!grid || !rulerWrap) return;

        var bpm = parseFloat(grid.dataset.seqBpm) || 120;
        var bars = parseInt(grid.dataset.seqBars) || 1;
        var tsig = grid.dataset.seqTsig || '4_4';
        var ticksPerStep = parseInt(grid.dataset.seqDivision) || 24;

        var tsParts = tsig.split('_');
        var beatsPerBar = parseInt(tsParts[0]) || 4;
        var beatDenom = parseInt(tsParts[1]) || 4;
        var ticksPerBar = beatsPerBar * 96 * 4 / beatDenom;
        var stepsPerBar = Math.round(ticksPerBar / ticksPerStep);

        rulerWrap.innerHTML = '';
        var nameDiv = document.createElement('div');
        nameDiv.className = 'seq-grid-ruler-name';
        rulerWrap.appendChild(nameDiv);

        var timeline = document.createElement('div');
        timeline.className = 'seq-grid-ruler-timeline';
        var built = buildRulerContent({ bars: bars, beatsPerBar: beatsPerBar, stepsPerBar: stepsPerBar, pxPerStep: STEP_PX, bpm: bpm });
        timeline.style.width = built.totalPx + 'px';
        timeline.appendChild(built.el);
        rulerWrap.appendChild(timeline);
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
        var sampleNames = data.padSampleNames || [];

        var pxpt = STEP_PX / ticksPerStep;
        PX_PER_TICK = pxpt;

        container.dataset.ticksPerStep = ticksPerStep;
        container.dataset.ticksPerBar = ticksPerBar;
        container.dataset.stepsPerBar = stepsPerBar;
        container.dataset.pxPerTick = pxpt;

        var timelineW = Math.ceil(seqTotalTicks * pxpt);

        // Group events by padIndex.
        var padEvents = {};
        events.forEach(function(e) {
            if (!padEvents[e.padIndex]) padEvents[e.padIndex] = [];
            padEvents[e.padIndex].push(e);
        });

        // Build inner container.
        var inner = document.createElement('div');
        inner.className = 'seq-cont-inner';
        inner.style.width = (TRACK_NAME_W + timelineW) + 'px';

        // Header: three-row ruler.
        var header = document.createElement('div');
        header.className = 'seq-cont-header';
        var headerName = document.createElement('div');
        headerName.className = 'seq-cont-header-name';
        header.appendChild(headerName);
        var rulers = document.createElement('div');
        rulers.className = 'seq-cont-rulers';
        rulers.style.width = timelineW + 'px';
        var bpmVal = data.bpm || 120;
        var rulerBuilt = buildRulerContent({ bars: bars, beatsPerBar: beatsPerBar, stepsPerBar: stepsPerBar, pxPerStep: STEP_PX, bpm: bpmVal });
        rulers.appendChild(rulerBuilt.el);
        header.appendChild(rulers);
        inner.appendChild(header);

        // Tracks container (position: relative for playhead).
        var tracksDiv = document.createElement('div');
        tracksDiv.className = 'seq-cont-tracks';

        var bankLetters = ['a', 'b', 'c', 'd'];
        var bankNames   = ['A', 'B', 'C', 'D'];

        bankLetters.forEach(function(bankLetter, bankIdx) {
            var expanded = expandedBanks.has(bankLetter);

            // Bank header bar.
            var bankHeader = document.createElement('div');
            bankHeader.className = 'seq-cont-bank-header seq-cont-bank-' + bankLetter + '-header';
            var bhName = document.createElement('div');
            bhName.className = 'seq-cont-bank-header-name';
            bhName.textContent = 'Bank ' + bankNames[bankIdx] + ' ';
            var arrow = document.createElement('span');
            arrow.className = 'bank-sep-arrow';
            arrow.textContent = expanded ? '▼' : '▶';
            bhName.appendChild(arrow);
            var bhBody = document.createElement('div');
            bhBody.className = 'seq-cont-bank-header-body';
            bankHeader.appendChild(bhName);
            bankHeader.appendChild(bhBody);
            bankHeader.onclick = (function(letter) { return function() { SequencePlayer.toggleBank(letter); }; })(bankLetter);
            tracksDiv.appendChild(bankHeader);

            // Bank group: 16 pad rows.
            var bankGroup = document.createElement('div');
            bankGroup.className = 'seq-cont-bank-group seq-cont-bank-' + bankLetter;
            if (!expanded) bankGroup.style.display = 'none';

            for (var padOffset = 0; padOffset < 16; padOffset++) {
                var padIdx = bankIdx * 16 + padOffset;
                if (!sampleNames[padIdx] && !(padEvents[padIdx] && padEvents[padIdx].length)) continue;
                var row = document.createElement('div');
                row.className = 'seq-cont-track';
                row.style.height = TRACK_H + 'px';
                row.setAttribute('data-pad', padIdx);
                if (mutedPads.has(padIdx) || (soloPads.size > 0 && !soloPads.has(padIdx))) {
                    row.classList.add('track-muted');
                }

                var nameCell = document.createElement('div');
                nameCell.className = 'seq-cont-track-name';

                var controls = document.createElement('div');
                controls.className = 'seq-cont-track-controls';

                var prevBtn = document.createElement('button');
                prevBtn.className = 'track-preview-btn';
                prevBtn.innerHTML = '&#9654;';
                prevBtn.title = 'Preview pad';
                prevBtn.onclick = (function(idx) { return function(ev) { ev.stopPropagation(); SequenceEditor.previewPad(idx); }; })(padIdx);
                controls.appendChild(prevBtn);

                var muteBtn = document.createElement('button');
                muteBtn.className = 'track-mute-btn' + (mutedPads.has(padIdx) ? ' active' : '');
                muteBtn.textContent = 'M';
                muteBtn.title = 'Mute';
                muteBtn.onclick = (function(idx, btn) { return function(ev) { ev.stopPropagation(); SequencePlayer.toggleMutePad(idx, btn); }; })(padIdx, muteBtn);
                controls.appendChild(muteBtn);

                var soloBtn = document.createElement('button');
                soloBtn.className = 'track-solo-btn' + (soloPads.has(padIdx) ? ' active' : '');
                soloBtn.textContent = 'S';
                soloBtn.title = 'Solo';
                soloBtn.onclick = (function(idx, btn) { return function(ev) { ev.stopPropagation(); SequencePlayer.toggleSoloPad(idx, btn); }; })(padIdx, soloBtn);
                controls.appendChild(soloBtn);

                var labelSpan = document.createElement('span');
                labelSpan.className = 'track-label';
                labelSpan.textContent = padLabel(padIdx);
                controls.appendChild(labelSpan);

                nameCell.appendChild(controls);

                var sn = sampleNames[padIdx] || '';
                if (sn) {
                    var snSpan = document.createElement('div');
                    snSpan.className = 'seq-cont-track-sample-name';
                    snSpan.title = sn;
                    snSpan.textContent = sn;
                    nameCell.appendChild(snSpan);
                }

                row.appendChild(nameCell);

                var body = document.createElement('div');
                body.className = 'seq-cont-track-body';
                body.style.width = timelineW + 'px';

                // Gridlines.
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
                addGridline(body, timelineW, 'seq-cont-gridline-bar');

                // Event blocks.
                (padEvents[padIdx] || []).forEach(function(e) {
                    var evDiv = document.createElement('div');
                    var evGs = Math.floor(e.tick / ticksPerStep);
                    evDiv.className = 'seq-cont-event';
                    evDiv.dataset.pad = padIdx;
                    evDiv.dataset.tick = e.tick;
                    evDiv.dataset.dur = e.durationTicks;
                    evDiv.dataset.vel = e.velocity;
                    evDiv.dataset.gs = evGs;
                    evDiv.style.left = Math.round(e.tick * pxpt) + 'px';
                    evDiv.style.width = Math.max(3, Math.round(e.durationTicks * pxpt)) + 'px';
                    evDiv.style.backgroundColor = velocityToColor(e.velocity);
                    evDiv.style.opacity = 0.5 + e.velocity / 127 * 0.5;
                    evDiv.title = padLabel(padIdx) + ' vel:' + e.velocity;
                    if (SequenceEditor.isContInSelection(padIdx, evGs)) {
                        evDiv.classList.add('seq-cont-event-selected');
                    }
                    body.appendChild(evDiv);
                });

                row.appendChild(body);
                bankGroup.appendChild(row);
            }

            tracksDiv.appendChild(bankGroup);
        });

        // Playhead — height covers all visible bank rows + headers.
        var totalContH = 4 * BANK_HEADER_H;
        tracksDiv.querySelectorAll('.seq-cont-bank-group').forEach(function(grp) {
            var bl = grp.className.replace(/.*seq-cont-bank-([a-d])$/, '$1');
            if (expandedBanks.has(bl)) totalContH += grp.querySelectorAll('.seq-cont-track').length * TRACK_H;
        });
        var ph = document.createElement('div');
        ph.id = 'seq-cont-playhead';
        ph.style.height = totalContH + 'px';
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
        if (m === 'grid') renderGridRuler();
        // Re-init playhead for the active view.
        if (playing) initPlayhead();
        SequenceEditor.restoreSnapBtn();
    }

    const mutedPads = new Set();
    const soloPads = new Set();

    var ALL_BANKS = [
        { letter: 'a', rowClass: '.bank-a-row', gridHeader: '.bank-a-header', contHeader: '.seq-cont-bank-a-header', contGroup: '.seq-cont-bank-a' },
        { letter: 'b', rowClass: '.bank-b-row', gridHeader: '.bank-b-header', contHeader: '.seq-cont-bank-b-header', contGroup: '.seq-cont-bank-b' },
        { letter: 'c', rowClass: '.bank-c-row', gridHeader: '.bank-c-header', contHeader: '.seq-cont-bank-c-header', contGroup: '.seq-cont-bank-c' },
        { letter: 'd', rowClass: '.bank-d-row', gridHeader: '.bank-d-header', contHeader: '.seq-cont-bank-d-header', contGroup: '.seq-cont-bank-d' },
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

    function afterDetailSwap() {
        SequenceEditor.clearSelection();
        syncLoopFromDOM();
        SequenceEditor.restoreModeButtons();
        SequenceEditor.restoreSnapBtn();
        restoreBankState();
        restoreViewLayout();
        renderGridRuler();
        refreshEvents();
    }

    document.addEventListener('htmx:afterSwap', function(evt) {
        var swapTarget = (evt.detail && evt.detail.target) || evt.target;
        var isSeqGrid = swapTarget && (
            swapTarget.id === 'sequence-grid' ||
            swapTarget.closest('#sequence-grid') ||
            (swapTarget.querySelector && !!swapTarget.querySelector('#sequence-grid'))
        );
        if (!isSeqGrid) return;
        afterDetailSwap();
    });

    document.addEventListener('DOMContentLoaded', function() {
        var stored = localStorage.getItem('seq-expanded-banks');
        if (stored !== null) {
            expandedBanks = new Set(stored ? stored.split(',') : []);
        }
        restoreBankState();
        syncLoopFromDOM();
        renderGridRuler();
        SequenceEditor.restoreSnapBtn();
    });

    function saveBankState() {
        localStorage.setItem('seq-expanded-banks', Array.from(expandedBanks).join(','));
    }

    function restoreBankState() {
        ALL_BANKS.forEach(function(bank) {
            var expanded = expandedBanks.has(bank.letter);

            // Grid: update header arrow.
            var gridHdr = document.querySelector(bank.gridHeader);
            if (gridHdr) {
                var arr = gridHdr.querySelector('.bank-sep-arrow');
                if (arr) arr.textContent = expanded ? '▼' : '▶';
            }
            // Grid: show/hide all pad rows.
            document.querySelectorAll(bank.rowClass).forEach(function(el) {
                el.style.display = expanded ? '' : 'none';
            });

            // Piano roll: update header arrow.
            var contHdr = document.querySelector(bank.contHeader);
            if (contHdr) {
                var arr2 = contHdr.querySelector('.bank-sep-arrow');
                if (arr2) arr2.textContent = expanded ? '▼' : '▶';
            }
            // Piano roll: show/hide bank group.
            var contGrp = document.querySelector(bank.contGroup);
            if (contGrp) contGrp.style.display = expanded ? '' : 'none';
        });

        // Keep piano roll playhead height in sync.
        var ph = document.getElementById('seq-cont-playhead');
        if (ph) {
            var totalH = 4 * BANK_HEADER_H;
            ALL_BANKS.forEach(function(bank) {
                if (expandedBanks.has(bank.letter)) {
                    var grp = document.querySelector(bank.contGroup);
                    if (grp) totalH += grp.querySelectorAll('.seq-cont-track').length * TRACK_H;
                }
            });
            ph.style.height = totalH + 'px';
        }
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
        var row = btn.closest('tr') || btn.closest('.seq-cont-track');
        if (mutedPads.has(padIndex)) {
            mutedPads.delete(padIndex);
            btn.classList.remove('active');
            if (row) row.classList.remove('track-muted');
        } else {
            mutedPads.add(padIndex);
            btn.classList.add('active');
            if (row) row.classList.add('track-muted');
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
        document.querySelectorAll('.seq-cont-track[data-pad]').forEach(function(row) {
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
        getViewMode: function() { return seqViewMode; },
        restoreViewLayout: restoreViewLayout,
        afterDetailSwap: afterDetailSwap,
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

    // --- grid insert drag state ---
    var gridInsert = null; // { pad, step, bar, fromActive, el, moved }
    var gridInsertOverCell = null;
    var gridInsertHandled = false; // suppresses click after mouseup

    // --- piano roll drag state ---
    var contDrag = null; // { pad, gs, tick, el, startX, ticksPerStep, ticksPerBar, isMulti }
    var contDragOverRow = null;
    var contDragPreview = null; // semi-opaque preview block shown at destination during drag

    // --- piano roll insert state ---
    var contInsert = null; // { pad, tick, el, body } — pending insert while mouse held in insert mode

    // --- multi-select state ---
    var selectedCells = new Set(); // "pad:globalStep" keys
    var detailIsMulti = false;
    var detailFromCont = false; // true when detail was opened from piano roll

    // --- piano roll selection state ---
    var contSelectedKeys = new Set(); // "pad:gs"
    var contEventDataByKey = {}; // "pad:gs" → {pad, tick, gs, vel, dur}

    // --- piano roll snap state ---
    var contSnapEnabled = true;

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
        clearContSelection();
    }

    function contKey(pad, gs) { return pad + ':' + gs; }

    function isContInSelection(pad, gs) {
        return contSelectedKeys.has(contKey(pad, gs));
    }

    function addContToSelection(el, pad, gs, tick, vel, dur) {
        var k = contKey(pad, gs);
        contSelectedKeys.add(k);
        contEventDataByKey[k] = { pad: pad, tick: tick, gs: gs, vel: vel, dur: dur };
        el.classList.add('seq-cont-event-selected');
    }

    function removeContFromSelection(el, pad, gs) {
        var k = contKey(pad, gs);
        contSelectedKeys.delete(k);
        delete contEventDataByKey[k];
        el.classList.remove('seq-cont-event-selected');
    }

    function clearContSelection() {
        contSelectedKeys.clear();
        contEventDataByKey = {};
        document.querySelectorAll('.seq-cont-event-selected').forEach(function(el) {
            el.classList.remove('seq-cont-event-selected');
        });
    }

    function removeContDragPreview() {
        if (contDragPreview) { contDragPreview.remove(); contDragPreview = null; }
    }

    function getContSelectedEvents() {
        return Object.keys(contEventDataByKey).map(function(k) {
            var d = contEventDataByKey[k];
            return { pad: d.pad, global_step: d.gs };
        });
    }

    function contUseSnap(shiftKey) {
        // Shift temporarily inverts the snap toggle state.
        return shiftKey ? !contSnapEnabled : contSnapEnabled;
    }

    function toggleSnap(btn) {
        contSnapEnabled = !contSnapEnabled;
        var b = btn || document.getElementById('seq-snap-btn');
        if (b) b.classList.toggle('active', contSnapEnabled);
    }

    function restoreSnapBtn() {
        var b = document.getElementById('seq-snap-btn');
        var sep = document.getElementById('seq-snap-sep');
        if (!b) return;
        var inGrid = (SequencePlayer.getViewMode() === 'grid');
        b.style.display = inGrid ? 'none' : '';
        if (sep) sep.style.display = inGrid ? 'none' : '';
        b.classList.toggle('active', inGrid ? true : contSnapEnabled);
    }

    function snapTick(rawTick, ticksPerStep, useSnap) {
        return useSnap
            ? Math.round(rawTick / ticksPerStep) * ticksPerStep
            : Math.round(rawTick);
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
                    restoreSnapBtn();
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
        if (drag) return; // ignore clicks that follow an edit-mode drag
        if (gridInsertHandled) { gridInsertHandled = false; return; } // handled by mousedown/mouseup drag
        if (e.ctrlKey || e.metaKey) {
            if (cell.classList.contains('step-active')) {
                if (isInSelection(cell)) { removeFromSelection(cell); } else { addToSelection(cell); }
            }
            return;
        }
        var pad = parseInt(cell.dataset.pad);
        var step = parseInt(cell.dataset.step);
        var bar = parseInt(cell.dataset.bar) || 1;
        var params = { action: 'toggle', pad: pad, step: step, bar: bar, velocity: 100, duration: 23 };
        // If the event sits off-grid, pass its raw tick so the backend can locate it.
        if (cell.classList.contains('step-active') && cell.dataset.tick) {
            params.tick = parseInt(cell.dataset.tick);
        }
        postEdit(params);
    });

    // ---- insert mode: mousedown → drag to position, mouseup to commit ----

    document.addEventListener('mousedown', function(e) {
        if (mode === 'insert' && e.button === 0) {
            var insertCell = e.target.closest('#seq-step-grid .step-cell');
            if (insertCell) {
                e.preventDefault();
                gridInsertHandled = false;
                gridInsert = {
                    pad: parseInt(insertCell.dataset.pad),
                    step: parseInt(insertCell.dataset.step),
                    bar: parseInt(insertCell.dataset.bar) || 1,
                    fromActive: insertCell.classList.contains('step-active'),
                    el: insertCell,
                    moved: false
                };
                dragGhost = document.createElement('div');
                dragGhost.className = 'step-drag-ghost';
                dragGhost.style.left = (e.clientX + 14) + 'px';
                dragGhost.style.top = (e.clientY - 14) + 'px';
                document.body.appendChild(dragGhost);
                return;
            }
        }
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
        if (!drag && !contDrag && !contInsert && !gridInsert) return;

        if (dragGhost) {
            dragGhost.style.left = (e.clientX + 14) + 'px';
            dragGhost.style.top = (e.clientY - 14) + 'px';
        }

        if (gridInsert) {
            var elgi = document.elementFromPoint(e.clientX, e.clientY);
            var nextCell = elgi && elgi.closest('#seq-step-grid .step-cell');
            var different = nextCell && nextCell !== gridInsert.el;
            if (different) gridInsert.moved = true;
            var target = different ? nextCell : null;
            if (target !== gridInsertOverCell) {
                if (gridInsertOverCell) gridInsertOverCell.classList.remove('step-drop-target');
                gridInsertOverCell = target;
                if (gridInsertOverCell) gridInsertOverCell.classList.add('step-drop-target');
            }
        }

        if (contDrag) {
            // Track which row the cursor is over.
            var el = document.elementFromPoint(e.clientX, e.clientY);
            var row = el && el.closest('.seq-cont-track');
            var sourceRow = contDrag.el.closest('.seq-cont-track');
            var nextRow = (row && row !== sourceRow) ? row : null;
            if (nextRow !== contDragOverRow) {
                if (contDragOverRow) contDragOverRow.classList.remove('seq-cont-track-drop-target');
                contDragOverRow = nextRow;
                if (contDragOverRow) contDragOverRow.classList.add('seq-cont-track-drop-target');
            }

            // Compute where the event will land and show a preview block there.
            var contContainer = document.getElementById('seq-continuous-view');
            var pxPerTick = contContainer ? (parseFloat(contContainer.dataset.pxPerTick) || 1.5) : 1.5;
            var rawDelta = (e.clientX - contDrag.startX) / pxPerTick;
            var useSnap = contUseSnap(e.shiftKey);
            // Snap the absolute destination tick, not the delta, so off-grid sources land on the grid.
            var rawNewTick = contDrag.tick + rawDelta;
            var previewTick = useSnap
                ? Math.max(0, Math.round(rawNewTick / contDrag.ticksPerStep) * contDrag.ticksPerStep)
                : Math.max(0, Math.round(rawNewTick));
            var targetTrack = contDragOverRow || sourceRow;
            var targetBody = targetTrack ? targetTrack.querySelector('.seq-cont-track-body') : null;
            if (targetBody) {
                if (!contDragPreview) {
                    contDragPreview = document.createElement('div');
                    contDragPreview.className = 'seq-cont-event seq-cont-event-drag-preview';
                    contDragPreview.style.width = contDrag.el.style.width;
                    contDragPreview.style.backgroundColor = contDrag.el.style.backgroundColor;
                }
                if (contDragPreview.parentNode !== targetBody) {
                    targetBody.appendChild(contDragPreview);
                }
                contDragPreview.style.left = Math.round(previewTick * pxPerTick) + 'px';
            } else {
                removeContDragPreview();
            }
        }

        if (contInsert) {
            var contContainer2 = document.getElementById('seq-continuous-view');
            var pxPerTick2 = contContainer2 ? (parseFloat(contContainer2.dataset.pxPerTick) || 1.5) : 1.5;
            var tps2 = contContainer2 ? (parseInt(contContainer2.dataset.ticksPerStep) || 24) : 24;
            // Follow cursor to a different pad row if needed.
            var elHover = document.elementFromPoint(e.clientX, e.clientY);
            var hoverTrack = elHover && elHover.closest('.seq-cont-track');
            var hoverBody = hoverTrack ? hoverTrack.querySelector('.seq-cont-track-body') : null;
            if (hoverBody && hoverBody !== contInsert.body) {
                hoverBody.appendChild(contInsert.el);
                contInsert.body = hoverBody;
                contInsert.pad = parseInt(hoverTrack.dataset.pad) || contInsert.pad;
            }
            var bodyRect2 = contInsert.body.getBoundingClientRect();
            var rawTick2 = Math.max(0, (e.clientX - bodyRect2.left) / pxPerTick2);
            var tick2 = snapTick(rawTick2, tps2, contUseSnap(e.shiftKey));
            contInsert.tick = tick2;
            contInsert.el.style.left = Math.round(tick2 * pxPerTick2) + 'px';
        }

        if (!drag) return;
        // Cursor is never under the ghost, so elementFromPoint is reliable here.
        var el2 = document.elementFromPoint(e.clientX, e.clientY);
        var cell = el2 && el2.closest('#seq-step-grid .step-cell');
        var next = (cell && cell !== drag.el) ? cell : null;
        if (next !== dragOverCell) {
            if (dragOverCell) dragOverCell.classList.remove('step-drop-target');
            dragOverCell = next;
            if (dragOverCell) dragOverCell.classList.add('step-drop-target');
        }
    });

    document.addEventListener('mouseup', function(e) {
        // Piano roll drag
        if (contDrag) {
            var contContainer = document.getElementById('seq-continuous-view');
            var pxPerTick = contContainer ? (parseFloat(contContainer.dataset.pxPerTick) || 1.5) : 1.5;
            var dx = e.clientX - contDrag.startX;
            var useSnap = contUseSnap(e.shiftKey);
            var rawDelta = dx / pxPerTick;
            var fromTick = contDrag.tick;
            // Snap the absolute destination so off-grid events land on a grid step.
            var rawNewTick = fromTick + rawDelta;
            var newTick = useSnap
                ? Math.max(0, Math.round(rawNewTick / contDrag.ticksPerStep) * contDrag.ticksPerStep)
                : Math.max(0, Math.round(rawNewTick));
            var deltaTicks = newTick - fromTick;

            // Resolve target pad from the row under the cursor.
            if (dragGhost) { dragGhost.remove(); dragGhost = null; }
            var targetEl = document.elementFromPoint(e.clientX, e.clientY);
            var targetRow = targetEl ? targetEl.closest('.seq-cont-track') : null;
            var toPad = targetRow ? parseInt(targetRow.dataset.pad) : contDrag.pad;
            if (isNaN(toPad)) toPad = contDrag.pad;

            contDrag.el.classList.remove('seq-cont-event-dragging');
            if (contDragOverRow) { contDragOverRow.classList.remove('seq-cont-track-drop-target'); contDragOverRow = null; }
            removeContDragPreview();
            var savedContDrag = contDrag;
            contDrag = null;

            if (deltaTicks !== 0 || toPad !== savedContDrag.pad) {
                if (savedContDrag.isMulti) {
                    var padDelta = toPad - savedContDrag.pad;
                    var cevs = Object.keys(contEventDataByKey).map(function(k) {
                        var d = contEventDataByKey[k];
                        var dRawNew = d.tick + rawDelta;
                        var dNewTick = useSnap
                            ? Math.max(0, Math.round(dRawNew / savedContDrag.ticksPerStep) * savedContDrag.ticksPerStep)
                            : Math.max(0, Math.round(dRawNew));
                        return {
                            pad: d.pad,
                            global_step: d.gs,
                            to_pad: Math.max(0, Math.min(63, d.pad + padDelta)),
                            to_global_step: Math.max(0, d.gs + Math.round((dNewTick - d.tick) / savedContDrag.ticksPerStep)),
                            from_tick: d.tick,
                            to_tick: dNewTick
                        };
                    });
                    postEdit({ action: 'multi_move', events: JSON.stringify(cevs) });
                } else {
                    var fromBar = Math.floor(fromTick / savedContDrag.ticksPerBar) + 1;
                    var fromStep = Math.floor((fromTick % savedContDrag.ticksPerBar) / savedContDrag.ticksPerStep);
                    var toBar = Math.floor(newTick / savedContDrag.ticksPerBar) + 1;
                    var toStep = Math.floor((newTick % savedContDrag.ticksPerBar) / savedContDrag.ticksPerStep);
                    postEdit({
                        action: 'move',
                        from_pad: savedContDrag.pad, from_step: fromStep, from_bar: fromBar,
                        from_tick: fromTick,
                        to_pad: toPad, to_step: toStep, to_bar: toBar,
                        to_tick: newTick
                    });
                }
            }
            return;
        }

        // Piano roll insert commit
        if (contInsert) {
            var insertTick = contInsert.tick;
            var insertPad = contInsert.pad;
            contInsert.el.remove();
            contInsert = null;
            postEdit({ action: 'toggle', pad: insertPad, tick: insertTick, step: 0, bar: 1, velocity: 100, duration: 23 });
            return;
        }

        // Grid insert drag commit
        if (gridInsert) {
            if (dragGhost) { dragGhost.remove(); dragGhost = null; }
            if (gridInsertOverCell) { gridInsertOverCell.classList.remove('step-drop-target'); }
            var destCell = (gridInsert.moved && gridInsertOverCell) ? gridInsertOverCell : null;
            var savedGI = gridInsert;
            gridInsert = null;
            gridInsertOverCell = null;
            gridInsertHandled = true;

            if (destCell) {
                var giToPad = parseInt(destCell.dataset.pad);
                var giToStep = parseInt(destCell.dataset.step);
                var giToBar = parseInt(destCell.dataset.bar) || 1;
                if (savedGI.fromActive) {
                    var giMoveParams = {
                        action: 'move',
                        from_pad: savedGI.pad, from_step: savedGI.step, from_bar: savedGI.bar,
                        to_pad: giToPad, to_step: giToStep, to_bar: giToBar
                    };
                    if (savedGI.el.dataset.tick) giMoveParams.from_tick = parseInt(savedGI.el.dataset.tick);
                    postEdit(giMoveParams);
                } else {
                    postEdit({ action: 'toggle', pad: giToPad, step: giToStep, bar: giToBar, velocity: 100, duration: 23 });
                }
            } else {
                postEdit({ action: 'toggle', pad: savedGI.pad, step: savedGI.step, bar: savedGI.bar, velocity: 100, duration: 23 });
            }
            return;
        }

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
                var moveParams = {
                    action: 'move',
                    from_pad: fromPad, from_step: fromStep, from_bar: fromBar,
                    to_pad: toPad, to_step: toStep, to_bar: toBar
                };
                // Pass the raw source tick so the backend can find off-grid events.
                if (sourceEl.dataset.tick) moveParams.from_tick = parseInt(sourceEl.dataset.tick);
                postEdit(moveParams);
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
            detailFromCont = false;
            openDetail(e.clientX, e.clientY, 100, 23, true);
        } else {
            detailPad = parseInt(cell.dataset.pad);
            detailStep = parseInt(cell.dataset.step);
            detailBar = parseInt(cell.dataset.bar) || 1;
            detailFromCont = false;
            var vel = parseInt(cell.dataset.vel) || 100;
            var dur = parseInt(cell.dataset.dur) || 23;
            openDetail(e.clientX, e.clientY, vel, dur, false);
        }
    });

    // ---- piano roll right-click: event detail popover ----

    document.addEventListener('contextmenu', function(e) {
        var evDiv = e.target.closest('.seq-cont-event');
        if (!evDiv) return;
        e.preventDefault();

        var pad = parseInt(evDiv.dataset.pad);
        var gs = parseInt(evDiv.dataset.gs);
        var tick = parseInt(evDiv.dataset.tick);
        var vel = parseInt(evDiv.dataset.vel) || 100;
        var dur = parseInt(evDiv.dataset.dur) || 23;

        if (contSelectedKeys.size > 0 && !isContInSelection(pad, gs)) {
            clearContSelection();
            addContToSelection(evDiv, pad, gs, tick, vel, dur);
        } else if (contSelectedKeys.size === 0) {
            addContToSelection(evDiv, pad, gs, tick, vel, dur);
        }

        if (contSelectedKeys.size > 1) {
            detailPad = -1; detailStep = -1; detailBar = -1;
            detailFromCont = true;
            openDetail(e.clientX, e.clientY, 100, 23, true, contSelectedKeys.size);
        } else {
            var container = document.getElementById('seq-continuous-view');
            var tps = container ? (parseInt(container.dataset.ticksPerStep) || 24) : 24;
            var tpb = container ? (parseInt(container.dataset.ticksPerBar) || tps * 16) : tps * 16;
            detailPad = pad;
            detailStep = Math.floor(tick % tpb / tps);
            detailBar = Math.floor(tick / tpb) + 1;
            detailFromCont = false;
            openDetail(e.clientX, e.clientY, vel, dur, false);
        }
    });

    function openDetail(x, y, vel, dur, multi, count) {
        var panel = document.getElementById('seq-event-detail');
        if (!panel) return;
        detailIsMulti = !!multi;
        document.getElementById('seq-detail-vel').value = vel;
        document.getElementById('seq-detail-vel-display').textContent = vel;
        document.getElementById('seq-detail-dur').value = dur;
        var title = panel.querySelector('.seq-event-detail-title');
        var cnt = (count !== undefined) ? count : selectedCells.size;
        if (title) {
            title.textContent = multi ? 'Bulk Edit (' + cnt + ' events)' : 'Event Details';
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
            var evs = detailFromCont ? getContSelectedEvents() : getSelectedEvents();
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
            var evs = detailFromCont ? getContSelectedEvents() : getSelectedEvents();
            if (evs.length > 0) postEdit({ action: 'multi_delete', events: JSON.stringify(evs) });
            closeDetail();
            return;
        }
        if (detailPad < 0 || detailStep < 0 || detailBar < 0) return;
        postEdit({ action: 'delete', pad: detailPad, step: detailStep, bar: detailBar });
        closeDetail();
    }

    function quantizeDetail() {
        var qEl = document.getElementById('seq-detail-quantize');
        var qTicks = qEl ? (parseInt(qEl.value) || 24) : 24;
        if (detailIsMulti) {
            var evs = detailFromCont ? getContSelectedEvents() : getSelectedEvents();
            if (evs.length > 0) postEdit({ action: 'multi_quantize', events: JSON.stringify(evs), quantize_ticks: qTicks });
            closeDetail();
            return;
        }
        if (detailPad < 0 || detailStep < 0 || detailBar < 0) return;
        postEdit({ action: 'quantize', pad: detailPad, step: detailStep, bar: detailBar, quantize_ticks: qTicks });
        closeDetail();
    }

    function closeDetail() {
        var panel = document.getElementById('seq-event-detail');
        if (panel) panel.style.display = 'none';
        detailPad = -1;
        detailStep = -1;
        detailBar = -1;
        detailIsMulti = false;
        detailFromCont = false;
    }

    // Close detail on click outside; clear selection on click outside grid/piano-roll.
    document.addEventListener('mousedown', function(e) {
        var panel = document.getElementById('seq-event-detail');
        if (panel && panel.style.display !== 'none' && !panel.contains(e.target)) {
            closeDetail();
        }
        if (!e.target.closest('#seq-step-grid') && !e.target.closest('#seq-event-detail') && !e.target.closest('.seq-cont-event')) {
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
            } else if (contSelectedKeys.size > 0) {
                e.preventDefault();
                var cevs = getContSelectedEvents();
                if (cevs.length > 0) postEdit({ action: 'multi_delete', events: JSON.stringify(cevs) });
            }
        }
    });

    // ---- piano roll: click (ctrl+click selection / view-mode preview) ----

    document.addEventListener('click', function(e) {
        var evDiv = e.target.closest('.seq-cont-event');
        if (!evDiv) return;
        var pad = parseInt(evDiv.dataset.pad);
        var gs = parseInt(evDiv.dataset.gs);
        var tick = parseInt(evDiv.dataset.tick);
        var vel = parseInt(evDiv.dataset.vel);
        var dur = parseInt(evDiv.dataset.dur);
        if (e.ctrlKey || e.metaKey) {
            e.preventDefault();
            if (isContInSelection(pad, gs)) {
                removeContFromSelection(evDiv, pad, gs);
            } else {
                addContToSelection(evDiv, pad, gs, tick, vel, dur);
            }
            return;
        }
        if (mode === 'view') {
            clearContSelection();
            previewPad(pad);
        } else if (mode === 'insert') {
            // Clicking an existing event in insert mode removes it.
            postEdit({ action: 'toggle', pad: pad, tick: tick, step: 0, bar: 1, velocity: vel, duration: dur });
        }
    });

    // ---- piano roll: insert-mode mousedown on empty track body starts a pending insert ----

    document.addEventListener('mousedown', function(e) {
        if (mode !== 'insert') return;
        if (e.button !== 0) return;
        if (e.target.closest('.seq-cont-event')) return; // handled by click handler (toggle existing)
        var body = e.target.closest('.seq-cont-track-body');
        if (!body) return;
        var track = body.closest('.seq-cont-track');
        if (!track) return;
        var pad = parseInt(track.dataset.pad);
        if (isNaN(pad)) return;
        e.preventDefault();
        var contContainer = document.getElementById('seq-continuous-view');
        var pxPerTick = contContainer ? (parseFloat(contContainer.dataset.pxPerTick) || 1.5) : 1.5;
        var tps = contContainer ? (parseInt(contContainer.dataset.ticksPerStep) || 24) : 24;
        var rect = body.getBoundingClientRect();
        var rawTick = Math.max(0, (e.clientX - rect.left) / pxPerTick);
        var tick = snapTick(rawTick, tps, contUseSnap(e.shiftKey));
        var preview = document.createElement('div');
        preview.className = 'seq-cont-event seq-cont-event-insert-preview';
        preview.style.left = Math.round(tick * pxPerTick) + 'px';
        preview.style.width = Math.max(3, Math.round(23 * pxPerTick)) + 'px';
        preview.style.backgroundColor = '#44aa44'; // default vel 100 = green
        body.appendChild(preview);
        contInsert = { pad: pad, tick: tick, el: preview, body: body };
    });

    // ---- piano roll: edit-mode mousedown for drag ----

    document.addEventListener('mousedown', function(e) {
        if (mode !== 'edit') return;
        if (e.button !== 0) return;
        var evDiv = e.target.closest('.seq-cont-event');
        if (!evDiv) return;
        var pad = parseInt(evDiv.dataset.pad);
        var gs = parseInt(evDiv.dataset.gs);
        var tick = parseInt(evDiv.dataset.tick);
        var vel = parseInt(evDiv.dataset.vel);
        var dur = parseInt(evDiv.dataset.dur);
        if (e.ctrlKey || e.metaKey) {
            if (isContInSelection(pad, gs)) {
                removeContFromSelection(evDiv, pad, gs);
            } else {
                addContToSelection(evDiv, pad, gs, tick, vel, dur);
            }
            return;
        }
        e.preventDefault();
        var container = document.getElementById('seq-continuous-view');
        var tps = container ? (parseInt(container.dataset.ticksPerStep) || 24) : 24;
        var tpb = container ? (parseInt(container.dataset.ticksPerBar) || tps * 16) : tps * 16;
        var isMulti = contSelectedKeys.size > 1 && isContInSelection(pad, gs);
        contDrag = {
            pad: pad, gs: gs, tick: tick, vel: vel, dur: dur,
            el: evDiv, startX: e.clientX,
            ticksPerStep: tps, ticksPerBar: tpb,
            isMulti: isMulti
        };
        evDiv.classList.add('seq-cont-event-dragging');
        dragGhost = document.createElement('div');
        dragGhost.className = 'step-drag-ghost';
        dragGhost.style.left = (e.clientX + 14) + 'px';
        dragGhost.style.top = (e.clientY - 14) + 'px';
        document.body.appendChild(dragGhost);
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
        quantizeDetail: quantizeDetail,
        closeDetail: closeDetail,
        previewPad: previewPad,
        previewStep: previewStep,
        isContInSelection: isContInSelection,
        toggleSnap: toggleSnap,
        restoreSnapBtn: restoreSnapBtn
    };
})();
