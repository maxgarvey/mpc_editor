// MPC Editor - Web Audio API playback

const AudioPlayer = (function() {
    let audioCtx = null;
    const bufferCache = new Map();
    let activeSources = [];

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

    function stopAll() {
        for (let i = 0; i < activeSources.length; i++) {
            try { activeSources[i].stop(); } catch(e) {}
        }
        activeSources = [];
    }

    function play(url) {
        stopAll();

        fetchAndDecode(url).then(buffer => {
            const ctx = getContext();
            const source = ctx.createBufferSource();
            source.buffer = buffer;
            source.connect(ctx.destination);
            source.start(0);
            activeSources.push(source);
            source.onended = function() {
                const idx = activeSources.indexOf(source);
                if (idx >= 0) activeSources.splice(idx, 1);
            };
        }).catch(err => {
            console.warn('Audio playback failed:', err.message);
        });
    }

    function playLayerSource(url) {
        fetchAndDecode(url).then(buffer => {
            const ctx = getContext();
            const source = ctx.createBufferSource();
            source.buffer = buffer;
            source.connect(ctx.destination);
            source.start(0);
            activeSources.push(source);
            source.onended = function() {
                const idx = activeSources.indexOf(source);
                if (idx >= 0) activeSources.splice(idx, 1);
            };
        }).catch(err => {
            // Silently skip layers that can't be loaded (e.g. missing samples)
        });
    }

    function stop() {
        stopAll();
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
        playAllPadLayers: function(padIndex, layerCount) {
            stopAll();
            for (var i = 0; i < layerCount; i++) {
                playLayerSource(`/audio/pad/${padIndex}/${i}`);
            }
        },
        playSlice: function(sliceIndex) {
            play(`/audio/slice/${sliceIndex}`);
        }
    };
})();
