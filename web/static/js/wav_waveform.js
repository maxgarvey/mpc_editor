// MPC Editor - WAV detail waveform renderer (lightweight, no markers/selection)

const WavWaveform = (function() {
    let canvas = null;
    let ctx = null;
    let data = null;

    function load(relPath) {
        canvas = document.getElementById('wav-waveform-canvas');
        if (!canvas) return;

        ctx = canvas.getContext('2d');

        var container = canvas.parentElement;
        if (container) {
            canvas.width = container.clientWidth || 800;
        }

        fetch('/audio/waveform?path=' + encodeURIComponent(relPath) + '&width=' + canvas.width)
            .then(function(r) { return r.json(); })
            .then(function(d) {
                data = d;
                render();
            })
            .catch(function(err) { console.warn('Waveform load failed:', err); });
    }

    function render() {
        if (!data || !ctx || !canvas) return;

        var w = canvas.width;
        var h = canvas.height;
        var channels = data.channels;

        // Clear
        ctx.fillStyle = '#0f0f0f';
        ctx.fillRect(0, 0, w, h);

        if (!channels || channels.length === 0) return;

        var numChannels = channels.length;
        var channelHeight = h / numChannels;

        // Find global max amplitude for normalization
        var globalMax = 1;
        for (var c = 0; c < numChannels; c++) {
            var peaks = channels[c];
            for (var i = 0; i < peaks.length; i++) {
                var absMin = Math.abs(peaks[i].min);
                var absMax = Math.abs(peaks[i].max);
                if (absMin > globalMax) globalMax = absMin;
                if (absMax > globalMax) globalMax = absMax;
            }
        }

        // Draw each channel
        for (var c = 0; c < numChannels; c++) {
            var peaks = channels[c];
            var yOffset = c * channelHeight;
            var center = yOffset + channelHeight / 2;

            // Center line
            ctx.strokeStyle = '#333';
            ctx.lineWidth = 1;
            ctx.beginPath();
            ctx.moveTo(0, center);
            ctx.lineTo(w, center);
            ctx.stroke();

            // Waveform bars
            ctx.fillStyle = '#a0c840';
            for (var i = 0; i < peaks.length && i < w; i++) {
                var minNorm = (peaks[i].min / globalMax) * (channelHeight / 2);
                var maxNorm = (peaks[i].max / globalMax) * (channelHeight / 2);

                var y1 = center - maxNorm;
                var y2 = center - minNorm;
                var barH = Math.max(y2 - y1, 1);
                ctx.fillRect(i, y1, 1, barH);
            }
        }
    }

    return {
        load: load,
        render: render,
        getCanvas: function() { return canvas; }
    };
})();
