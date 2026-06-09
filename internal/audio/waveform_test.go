package audio

import "testing"

func TestDownsamplePeaks(t *testing.T) {
	samples := make([]int, 100)
	for i := range samples {
		samples[i] = i - 50 // ramp from -50 to 49
	}

	peaks := DownsamplePeaks(samples, 10)
	if len(peaks) != 10 {
		t.Fatalf("len(peaks) = %d, want 10", len(peaks))
	}
	// First bucket covers samples[0:10] → min -50, max -41.
	if peaks[0].Min != -50 || peaks[0].Max != -41 {
		t.Errorf("peaks[0] = %+v, want {-50 -41}", peaks[0])
	}
	// Last bucket covers samples[90:100] → min 40, max 49.
	if peaks[9].Min != 40 || peaks[9].Max != 49 {
		t.Errorf("peaks[9] = %+v, want {40 49}", peaks[9])
	}
}

func TestDownsamplePeaks_Edge(t *testing.T) {
	if got := DownsamplePeaks(nil, 10); got != nil {
		t.Errorf("nil samples: got %v, want nil", got)
	}
	if got := DownsamplePeaks([]int{1, 2, 3}, 0); got != nil {
		t.Errorf("zero buckets: got %v, want nil", got)
	}
	// More buckets than samples: fractional buckets are skipped (zero-valued),
	// the rest carry the single sample they cover.
	peaks := DownsamplePeaks([]int{5, -5}, 4)
	if len(peaks) != 4 {
		t.Fatalf("len(peaks) = %d, want 4", len(peaks))
	}
	if peaks[1].Min != 5 || peaks[1].Max != 5 {
		t.Errorf("peaks[1] = %+v, want {5 5}", peaks[1])
	}
	if peaks[3].Min != -5 || peaks[3].Max != -5 {
		t.Errorf("peaks[3] = %+v, want {-5 -5}", peaks[3])
	}
}
