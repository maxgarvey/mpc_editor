// MPC Editor - Web Audio API playback

const AudioPlayer = (function() {
    let audioCtx = null;
    const bufferCache = new Map();
    let activeSource = null;

    function getContext() {
        if (!audioCtx) {
            audioCtx = new (window.AudioContext || window.webkitAudioContext)();
        }
        // Resume if suspended (browsers require user gesture)
        if (audioCtx.state === 'suspended') {
            audioCtx.resume();
        }
        return audioCtx;
    }

    async function fetchAndDecode(url) {
        // Check cache first
        if (bufferCache.has(url)) {
            return bufferCache.get(url);
        }

        const response = await fetch(url);
        if (!response.ok) {
            throw new Error(`Failed to fetch ${url}: ${response.status}`);
        }

        const arrayBuffer = await response.arrayBuffer();
        const ctx = getContext();
        const audioBuffer = await ctx.decodeAudioData(arrayBuffer);

        bufferCache.set(url, audioBuffer);
        return audioBuffer;
    }

    function play(url) {
        // Stop any currently playing sound
        if (activeSource) {
            try { activeSource.stop(); } catch(e) {}
        }

        fetchAndDecode(url).then(buffer => {
            const ctx = getContext();
            const source = ctx.createBufferSource();
            source.buffer = buffer;
            source.connect(ctx.destination);
            source.start(0);
            activeSource = source;
            source.onended = function() {
                if (activeSource === source) {
                    activeSource = null;
                }
            };
        }).catch(err => {
            console.warn('Audio playback failed:', err.message);
        });
    }

    function stop() {
        if (activeSource) {
            try { activeSource.stop(); } catch(e) {}
            activeSource = null;
        }
    }

    function clearCache() {
        bufferCache.clear();
    }

    // Clear cache when a new program is loaded
    function invalidatePad(padIndex) {
        for (const key of bufferCache.keys()) {
            if (key.includes(`/audio/pad/${padIndex}/`)) {
                bufferCache.delete(key);
            }
        }
    }

    return {
        play: play,
        stop: stop,
        clearCache: clearCache,
        invalidatePad: invalidatePad,
        playPad: function(padIndex, layerIndex) {
            layerIndex = layerIndex || 0;
            play(`/audio/pad/${padIndex}/${layerIndex}`);
        },
        playSlice: function(sliceIndex) {
            play(`/audio/slice/${sliceIndex}`);
        }
    };
})();
