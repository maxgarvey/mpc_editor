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

    const mutedPads = new Set();
    const soloPads = new Set();

    function isPadAudible(padIndex) {
        if (soloPads.size > 0) return soloPads.has(padIndex);
        return !mutedPads.has(padIndex);
    }

    function play(seqPath, bar) {
        stop();
        fetch('/sequence/events?path=' + encodeURIComponent(seqPath) + '&bar=' + bar)
            .then(function(r) { return r.json(); })
            .then(function(data) {
                var padIndices = [];
                var seen = {};
                (data.events || []).forEach(function(e) {
                    if (!seen[e.padIndex]) { seen[e.padIndex] = true; padIndices.push(e.padIndex); }
                });
                return AudioPlayer.prefetchPadParams(padIndices).then(function() { return data; });
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
            seqEvents.forEach(function(e) {
                if (e.step === step && isPadAudible(e.padIndex)) {
                    AudioPlayer.playPadAtTime(e.padIndex, e.velocity, stepTime);
                }
            });
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
        toggleLoop: function(btn) {
            looping = !looping;
            btn.classList.toggle('active', looping);
        }
    };
})();
