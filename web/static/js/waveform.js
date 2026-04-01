// MPC Editor - Canvas Waveform Renderer

const Waveform = (function() {
    let waveformData = null;
    let canvas = null;
    let ctx = null;

    function load() {
        canvas = document.getElementById('waveform-canvas');
        if (!canvas) return;

        ctx = canvas.getContext('2d');

        // Set canvas to container width
        const container = canvas.parentElement;
        if (container) {
            canvas.width = container.clientWidth || 1000;
        }

        fetch('/slicer/waveform?width=' + canvas.width)
            .then(r => r.json())
            .then(data => {
                waveformData = data;
                render();
            })
            .catch(err => console.warn('Waveform load failed:', err));
    }

    function render() {
        if (!waveformData || !ctx || !canvas) return;

        const w = canvas.width;
        const h = canvas.height;
        const channels = waveformData.channels;
        const markers = waveformData.markers;
        const selected = waveformData.selected;
        const frameLength = waveformData.frameLength;

        // Clear
        ctx.fillStyle = '#0a0a1a';
        ctx.fillRect(0, 0, w, h);

        if (!channels || channels.length === 0) return;

        // Determine channel layout
        const numChannels = channels.length;
        const channelHeight = h / numChannels;

        // Find global max amplitude for normalization
        let globalMax = 1;
        for (const ch of channels) {
            for (const peak of ch) {
                const absMax = Math.max(Math.abs(peak.min), Math.abs(peak.max));
                if (absMax > globalMax) globalMax = absMax;
            }
        }

        // Draw each channel
        for (let c = 0; c < numChannels; c++) {
            const peaks = channels[c];
            const yOffset = c * channelHeight;
            const yCenter = yOffset + channelHeight / 2;
            const scale = (channelHeight / 2) / globalMax;

            // Draw center line
            ctx.strokeStyle = '#333';
            ctx.lineWidth = 0.5;
            ctx.beginPath();
            ctx.moveTo(0, yCenter);
            ctx.lineTo(w, yCenter);
            ctx.stroke();

            // Draw waveform
            ctx.fillStyle = '#2a8a5a';
            for (let i = 0; i < peaks.length && i < w; i++) {
                const peak = peaks[i];
                const top = yCenter - peak.max * scale;
                const bottom = yCenter - peak.min * scale;
                const barHeight = Math.max(1, bottom - top);
                ctx.fillRect(i, top, 1, barHeight);
            }
        }

        // Draw marker lines
        if (markers && markers.length > 0) {
            for (let i = 0; i < markers.length; i++) {
                const x = (markers[i] / frameLength) * w;
                const isSelected = (i === selected);

                ctx.strokeStyle = isSelected ? '#e94560' : '#f0c040';
                ctx.lineWidth = isSelected ? 2 : 1;
                ctx.beginPath();
                ctx.moveTo(x, 0);
                ctx.lineTo(x, h);
                ctx.stroke();

                // Marker number label
                ctx.fillStyle = isSelected ? '#e94560' : '#f0c040';
                ctx.font = '10px sans-serif';
                ctx.fillText(String(i), x + 2, 12);
            }
        }

        // Draw selected region highlight
        if (markers && markers.length > 0 && selected >= 0 && selected < markers.length) {
            const fromX = (markers[selected] / frameLength) * w;
            const toX = selected + 1 < markers.length
                ? (markers[selected + 1] / frameLength) * w
                : w;

            ctx.fillStyle = 'rgba(233, 69, 96, 0.08)';
            ctx.fillRect(fromX, 0, toX - fromX, h);
        }
    }

    // Click on canvas to select nearest marker
    document.addEventListener('click', function(e) {
        if (e.target.id !== 'waveform-canvas') return;
        if (!waveformData || !canvas) return;

        const rect = canvas.getBoundingClientRect();
        const clickX = e.clientX - rect.left;
        const clickFrame = (clickX / canvas.width) * waveformData.frameLength;

        // Find nearest marker
        const markers = waveformData.markers;
        let nearest = 0;
        let minDist = Infinity;
        for (let i = 0; i < markers.length; i++) {
            const dist = Math.abs(markers[i] - clickFrame);
            if (dist < minDist) {
                minDist = dist;
                nearest = i;
            }
        }

        // Use HTMX to select this marker
        htmx.ajax('GET', '/slicer/marker/select?index=' + nearest, {target: '#slicer-panel'});
    });

    // Re-render after HTMX updates
    document.addEventListener('htmx:afterSettle', function(e) {
        const panel = document.getElementById('slicer-panel');
        if (panel && document.getElementById('waveform-canvas')) {
            load();
        }
    });

    return {
        load: load,
        render: render
    };
})();
