package audio

// Peak represents the min/max amplitude within a downsampled bucket.
type Peak struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// DownsamplePeaks reduces a sample array to the given number of buckets,
// computing min/max peaks for each bucket. Used for waveform visualization.
func DownsamplePeaks(samples []int, buckets int) []Peak {
	if buckets <= 0 || len(samples) == 0 {
		return nil
	}

	peaks := make([]Peak, buckets)
	samplesPerBucket := float64(len(samples)) / float64(buckets)

	for i := range peaks {
		start := int(float64(i) * samplesPerBucket)
		end := int(float64(i+1) * samplesPerBucket)
		if end > len(samples) {
			end = len(samples)
		}
		if start >= end {
			continue
		}
		mn, mx := samples[start], samples[start]
		for _, v := range samples[start:end] {
			if v < mn {
				mn = v
			}
			if v > mx {
				mx = v
			}
		}
		peaks[i] = Peak{Min: mn, Max: mx}
	}
	return peaks
}
