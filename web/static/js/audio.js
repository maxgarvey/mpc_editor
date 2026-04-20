// MPC Editor - Web Audio API playback with parameter processing

const AudioPlayer = (function() {
    let audioCtx = null;
    const bufferCache = new Map();
    let activeSources = [];
    let stopCallbacks = [];
    let playGeneration = 0;

    function getContext() {
        if (!audioCtx) {
            audioCtx = new (window.AudioContext || window.webkitAudioContext)();
        }
        if (audioCtx.state === 'suspended') {
            audioCtx.resume();
        }
        return audioCtx;
    }

    async function fetchAndDecode(url) {
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
        playGeneration++;
        for (let i = 0; i < activeSources.length; i++) {
            try { activeSources[i].stop(); } catch(e) {}
        }
        activeSources = [];
        for (let i = 0; i < stopCallbacks.length; i++) {
            try { stopCallbacks[i](); } catch(e) {}
        }
    }

    // --- Parameter mapping functions ---

    function tuningToRate(semitones) {
        return Math.pow(2, semitones / 12);
    }

    function filterFreqHz(value) {
        // 0-100 -> 20Hz to 20kHz (log scale)
        return 20 * Math.pow(1000, value / 100);
    }

    function filterQ(value) {
        // 0-100 -> 0.5 to 25
        return 0.5 + (value / 100) * 24.5;
    }

    var filterTypeMap = ['off', 'lowpass', 'bandpass', 'highpass'];

    function attackSeconds(value) {
        return 0.001 + (value / 100) * 2.0;
    }

    function decaySeconds(value) {
        return 0.01 + (value / 100) * 3.0;
    }

    // --- Signal chain builders ---

    function buildPadChain(ctx, params, startTime, velScale) {
        var nodes = {};
        var now = Math.max(startTime !== undefined ? startTime : ctx.currentTime, ctx.currentTime);

        // Filter (bypass when type=0/Off)
        if (params.filter1.type > 0) {
            nodes.filter = ctx.createBiquadFilter();
            nodes.filter.type = filterTypeMap[params.filter1.type];
            nodes.filter.frequency.value = filterFreqHz(params.filter1.freq);
            nodes.filter.Q.value = filterQ(params.filter1.resonance);
        }

        // Envelope gain
        nodes.envelope = ctx.createGain();
        var atk = params.envelope.attack;
        var dec = params.envelope.decay;

        if (atk === 0 && dec === 0) {
            // No envelope shaping - constant gain
            nodes.envelope.gain.value = 1.0;
        } else if (params.envelope.decayMode === 1) {
            // Start mode: decay from note start
            nodes.envelope.gain.setValueAtTime(1.0, now);
            if (dec > 0) {
                nodes.envelope.gain.exponentialRampToValueAtTime(0.001, now + decaySeconds(dec));
            }
        } else {
            // End mode: attack then decay
            var atkTime = attackSeconds(atk);
            nodes.envelope.gain.setValueAtTime(0.001, now);
            nodes.envelope.gain.linearRampToValueAtTime(1.0, now + atkTime);
            if (dec > 0) {
                nodes.envelope.gain.exponentialRampToValueAtTime(0.001, now + atkTime + decaySeconds(dec));
            }
        }

        // Mixer gain
        nodes.mixerGain = ctx.createGain();
        nodes.mixerGain.gain.value = (params.mixer.level / 100) * (velScale !== undefined ? velScale : 1.0);

        // Panner
        nodes.panner = ctx.createStereoPanner();
        nodes.panner.pan.value = (params.mixer.pan - 50) / 50;

        // Connect: filter? -> envelope -> mixerGain -> panner -> destination
        var chainStart;
        if (nodes.filter) {
            chainStart = nodes.filter;
            nodes.filter.connect(nodes.envelope);
        } else {
            chainStart = nodes.envelope;
        }
        nodes.envelope.connect(nodes.mixerGain);
        nodes.mixerGain.connect(nodes.panner);
        nodes.panner.connect(ctx.destination);

        nodes.input = chainStart;
        return nodes;
    }

    function playLayerWithParams(ctx, url, layerParams, padChainInput) {
        return fetchAndDecode(url).then(function(buffer) {
            var source = ctx.createBufferSource();
            source.buffer = buffer;
            source.playbackRate.value = tuningToRate(layerParams.tuning);

            var layerGain = ctx.createGain();
            layerGain.gain.value = layerParams.level / 100;

            source.connect(layerGain);
            layerGain.connect(padChainInput);
            source.start(0);

            activeSources.push(source);
            source.onended = function() {
                var idx = activeSources.indexOf(source);
                if (idx >= 0) activeSources.splice(idx, 1);
            };
        });
    }

    function playLayerAtTime(ctx, url, layerParams, padChainInput, atTime) {
        return fetchAndDecode(url).then(function(buffer) {
            var source = ctx.createBufferSource();
            source.buffer = buffer;
            source.playbackRate.value = tuningToRate(layerParams.tuning);

            var layerGain = ctx.createGain();
            layerGain.gain.value = layerParams.level / 100;

            source.connect(layerGain);
            layerGain.connect(padChainInput);
            source.start(Math.max(atTime, ctx.currentTime));

            activeSources.push(source);
            source.onended = function() {
                var idx = activeSources.indexOf(source);
                if (idx >= 0) activeSources.splice(idx, 1);
            };
        });
    }

    // --- Raw playback (no params, for file browser / slicer) ---

    function play(url) {
        stopAll();
        fetchAndDecode(url).then(function(buffer) {
            var ctx = getContext();
            var source = ctx.createBufferSource();
            source.buffer = buffer;
            source.connect(ctx.destination);
            source.start(0);
            activeSources.push(source);
            source.onended = function() {
                var idx = activeSources.indexOf(source);
                if (idx >= 0) activeSources.splice(idx, 1);
            };
        }).catch(function(err) {
            console.warn('Audio playback failed:', err.message);
        });
    }

    function playLayerSource(url) {
        fetchAndDecode(url).then(function(buffer) {
            var ctx = getContext();
            var source = ctx.createBufferSource();
            source.buffer = buffer;
            source.connect(ctx.destination);
            source.start(0);
            activeSources.push(source);
            source.onended = function() {
                var idx = activeSources.indexOf(source);
                if (idx >= 0) activeSources.splice(idx, 1);
            };
        }).catch(function() {});
    }

    // --- Fetch params from API for non-selected pads ---

    var paramsCache = {};

    function getParams(padIndex) {
        var embedded = window.__padParams;
        if (embedded && embedded.padIndex === padIndex) {
            return Promise.resolve(embedded);
        }
        if (paramsCache[padIndex]) {
            return Promise.resolve(paramsCache[padIndex]);
        }
        return fetch('/api/pad-params/' + padIndex)
            .then(function(r) { return r.json(); })
            .then(function(data) {
                paramsCache[padIndex] = data;
                return data;
            })
            .catch(function() { return null; });
    }

    function stop() {
        stopAll();
    }

    function clearCache() {
        bufferCache.clear();
        paramsCache = {};
    }

    function invalidatePad(padIndex) {
        for (var key of bufferCache.keys()) {
            if (key.includes('/audio/pad/' + padIndex + '/')) {
                bufferCache.delete(key);
            }
        }
        delete paramsCache[padIndex];
    }

    return {
        play: play,
        stop: stop,
        clearCache: clearCache,
        invalidatePad: invalidatePad,
        getContext: getContext,
        getBuffer: fetchAndDecode,
        onStopAll: function(cb) { stopCallbacks.push(cb); },

        playPad: function(padIndex, layerIndex) {
            stopAll();
            layerIndex = layerIndex || 0;
            getParams(padIndex).then(function(params) {
                if (!params) {
                    play('/audio/pad/' + padIndex + '/' + layerIndex);
                    return;
                }
                var ctx = getContext();
                var chain = buildPadChain(ctx, params);
                var lp = params.layers[layerIndex];
                playLayerWithParams(ctx, '/audio/pad/' + padIndex + '/' + layerIndex, lp, chain.input)
                    .catch(function(err) {
                        console.warn('Audio playback failed:', err.message);
                    });
            });
        },

        playAllPadLayers: function(padIndex) {
            stopAll();
            getParams(padIndex).then(function(params) {
                if (!params) {
                    for (var i = 0; i < 4; i++) {
                        playLayerSource('/audio/pad/' + padIndex + '/' + i);
                    }
                    return;
                }
                var ctx = getContext();
                var chain = buildPadChain(ctx, params);
                for (var i = 0; i < 4; i++) {
                    if (params.layers[i].hasSample) {
                        playLayerWithParams(ctx, '/audio/pad/' + padIndex + '/' + i, params.layers[i], chain.input)
                            .catch(function() {});
                    }
                }
            });
        },

        playSlice: function(sliceIndex) {
            play('/audio/slice/' + sliceIndex);
        },

        prefetchPadParams: function(padIndices) {
            return Promise.all(padIndices.map(function(i) { return getParams(i); }));
        },

        playPadAtTime: function(padIndex, velocity, atTime) {
            var gen = playGeneration;
            var velScale = velocity / 127;
            getParams(padIndex).then(function(params) {
                if (playGeneration !== gen) return;
                var ctx = getContext();
                var chain = buildPadChain(ctx, params, atTime, velScale);
                var layers = params ? params.layers : [];
                for (var i = 0; i < layers.length; i++) {
                    if (!layers[i].hasSample) continue;
                    (function(li) {
                        playLayerAtTime(ctx, '/audio/pad/' + padIndex + '/' + li, layers[li], chain.input, atTime)
                            .catch(function() {});
                    })(i);
                }
            });
        }
    };
})();
