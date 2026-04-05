// MPC Editor - Sequence step playback preview

const SequencePlayer = (function() {
    let playing = false;
    let stepTimer = null;
    let currentStep = 0;
    let audioCtx = null;
    const mutedTracks = new Set();
    const soloTracks = new Set();

    function getContext() {
        if (!audioCtx) {
            audioCtx = new (window.AudioContext || window.webkitAudioContext)();
        }
        if (audioCtx.state === 'suspended') {
            audioCtx.resume();
        }
        return audioCtx;
    }

    function isTrackAudible(trackIndex) {
        if (soloTracks.size > 0) {
            return soloTracks.has(trackIndex);
        }
        return !mutedTracks.has(trackIndex);
    }

    function play(seqPath, bar) {
        stop();
        fetch('/sequence/events?path=' + encodeURIComponent(seqPath) + '&bar=' + bar)
            .then(function(r) { return r.json(); })
            .then(function(data) { startPlayback(data); })
            .catch(function(err) { console.warn('Sequence fetch failed:', err); });
    }

    function startPlayback(data) {
        playing = true;
        currentStep = 0;

        var bpm = data.bpm || 120;
        var stepDurationMs = (60000 / bpm) / 4; // sixteenth note duration
        var events = data.events || [];

        stepTimer = setInterval(function() {
            if (currentStep >= 16) {
                stop();
                return;
            }

            // Trigger notes on this step
            var stepEvents = events.filter(function(e) { return e.step === currentStep; });
            stepEvents.forEach(function(e) {
                if (isTrackAudible(e.track)) {
                    playTone(e.note, e.velocity, e.durationSteps, bpm);
                }
            });

            highlightStep(currentStep);
            currentStep++;
        }, stepDurationMs);
    }

    function playTone(note, velocity, durationSteps, bpm) {
        var ctx = getContext();
        var osc = ctx.createOscillator();
        var gain = ctx.createGain();

        // MIDI note to frequency
        osc.frequency.value = 440 * Math.pow(2, (note - 69) / 12);
        gain.gain.value = (velocity / 127) * 0.3;

        osc.connect(gain);
        gain.connect(ctx.destination);
        osc.start();

        var stepDur = (60 / bpm) / 4;
        osc.stop(ctx.currentTime + stepDur * durationSteps);
    }

    function highlightStep(step) {
        // Remove previous highlight
        document.querySelectorAll('.step-cell.step-playing').forEach(function(cell) {
            cell.classList.remove('step-playing');
        });
        // Add highlight to current column
        document.querySelectorAll('.step-col-' + step).forEach(function(cell) {
            cell.classList.add('step-playing');
        });
    }

    function stop() {
        playing = false;
        if (stepTimer) {
            clearInterval(stepTimer);
            stepTimer = null;
        }
        currentStep = 0;
        document.querySelectorAll('.step-cell.step-playing').forEach(function(cell) {
            cell.classList.remove('step-playing');
        });
    }

    function toggleMute(trackIndex, btn) {
        if (mutedTracks.has(trackIndex)) {
            mutedTracks.delete(trackIndex);
            btn.classList.remove('active');
            btn.closest('tr').classList.remove('track-muted');
        } else {
            mutedTracks.add(trackIndex);
            btn.classList.add('active');
            btn.closest('tr').classList.add('track-muted');
        }
    }

    function toggleSolo(trackIndex, btn) {
        if (soloTracks.has(trackIndex)) {
            soloTracks.delete(trackIndex);
            btn.classList.remove('active');
        } else {
            soloTracks.add(trackIndex);
            btn.classList.add('active');
        }
        // Update visual mute state on all rows
        document.querySelectorAll('.step-grid tbody tr').forEach(function(row) {
            var muteBtn = row.querySelector('.track-mute-btn');
            if (!muteBtn) return;
            // Determine track index from onclick
            var idx = parseInt(muteBtn.getAttribute('onclick').match(/\d+/)[0]);
            if (soloTracks.size > 0 && !soloTracks.has(idx)) {
                row.classList.add('track-muted');
            } else if (!mutedTracks.has(idx)) {
                row.classList.remove('track-muted');
            }
        });
    }

    return {
        play: play,
        stop: stop,
        isPlaying: function() { return playing; },
        toggleMute: toggleMute,
        toggleSolo: toggleSolo
    };
})();
