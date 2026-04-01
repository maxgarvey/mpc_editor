package audio

// samplesL returns the left channel sample data.
func (s *Slicer) samplesL() []int {
	return s.channels[0]
}

// energyHistory computes the energy for each analysis window across the sample.
func (s *Slicer) energyHistory() []int64 {
	samplesL := s.samplesL()
	var samplesR []int
	if len(s.channels) > 1 {
		samplesR = s.channels[1]
	} else {
		samplesR = samplesL
	}

	n := len(samplesL)
	step := s.windowSize / s.overlapRatio
	energyFrameNumber := n / step
	if energyFrameNumber < 1 {
		return nil
	}

	energy := make([]int64, energyFrameNumber)
	windowIndex := 0
	for i := 0; i+s.windowSize < n; i += step {
		var sum int64
		for j := 0; j < s.windowSize; j++ {
			l := int64(samplesL[i+j])
			r := int64(samplesR[i+j])
			sum += l*l + r*r
		}
		if windowIndex < len(energy) {
			energy[windowIndex] = sum
		}
		windowIndex++
	}
	return energy
}

// localEnergy computes the average energy in a local window around position i.
func (s *Slicer) localEnergy(i int, energyHistory []int64) int64 {
	n := len(energyHistory)
	m := s.localEnergyWindowSize

	var from, to int
	if i < m {
		from = 0
		to = m
	} else if i+m < n {
		from = i
		to = i + m
	} else {
		from = n - m
		to = n
	}

	var sum int64
	for j := from; j < to; j++ {
		sum += energyHistory[j]
	}
	return sum / int64(m)
}

// nearestZeroCrossing finds the nearest zero crossing before the given index,
// searching backwards up to excursion samples.
func nearestZeroCrossing(samples []int, index, excursion int) int {
	if index == 0 {
		return 0
	}
	i := index
	lo := index - excursion
	if lo < 0 {
		lo = 0
	}
	for !isZeroCross(samples, i) && i > lo {
		i--
	}
	return i
}

// isZeroCross returns true if the sample at index is a zero crossing point.
func isZeroCross(samples []int, index int) bool {
	if index == 0 {
		return true
	}
	if index >= len(samples)-1 {
		return true
	}
	a := samples[index-1]
	b := samples[index]
	return (a > 0 && b < 0) || (a < 0 && b > 0) || (a == 0 && b != 0)
}
