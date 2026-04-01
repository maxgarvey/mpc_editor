package pgm

import (
	"math"
	"testing"
)

func TestNoteName(t *testing.T) {
	tests := []struct {
		note int
		want string
	}{
		{35, "B0"},
		{36, "C1"},
		{48, "C2"},
		{60, "C3"},
		{69, "A3"},
	}
	for _, tt := range tests {
		got := NoteName(tt.note)
		if got != tt.want {
			t.Errorf("NoteName(%d) = %q, want %q", tt.note, got, tt.want)
		}
	}
}

func TestExtractNote(t *testing.T) {
	tests := []struct {
		name string
		want int
	}{
		{"C3", 60},
		{"C#3", 61},
		{"D 4", 74},
		{"E5", 88},
		{"B0", 35},
		{"nope", -1},
	}
	for _, tt := range tests {
		got := ExtractNote(tt.name)
		if got != tt.want {
			t.Errorf("ExtractNote(%q) = %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestMultisampleAssign(t *testing.T) {
	// Create samples with note names, mimicking "bass_C3", "bass_E3", "bass_G3"
	samples := []*SampleRef{
		{Name: "bass_C3"},
		{Name: "bass_E3"},
		{Name: "bass_G3"},
	}

	builder := &MultisampleBuilder{}
	slots := builder.Assign(samples)
	if slots == nil {
		t.Fatal("Assign returned nil")
	}

	// C3 = note 60, pad index = 60 - 35 = 25
	slot := slots[25]
	if slot == nil {
		t.Fatal("slot for C3 (index 25) is nil")
	}
	if slot.Note != 60 {
		t.Errorf("C3 slot note = %d, want 60", slot.Note)
	}
	if slot.Tuning != 0 {
		t.Errorf("C3 slot tuning = %f, want 0 (exact match)", slot.Tuning)
	}
	if slot.Source.Name != "bass_C3" {
		t.Errorf("C3 slot source = %q, want %q", slot.Source.Name, "bass_C3")
	}

	// E3 = note 64, pad index = 64 - 35 = 29
	slot = slots[29]
	if slot == nil {
		t.Fatal("slot for E3 (index 29) is nil")
	}
	if slot.Tuning != 0 {
		t.Errorf("E3 slot tuning = %f, want 0", slot.Tuning)
	}

	// D3 = note 62, should be filled by transposing C3 or E3
	slot = slots[27] // 62 - 35 = 27
	if slot == nil {
		t.Fatal("slot for D3 (index 27) is nil — gap not filled")
	}

	// All tunings should be within ±36 semitones
	for i, s := range slots {
		if s != nil && math.Abs(s.Tuning) > 36 {
			t.Errorf("slot[%d] (note %d) has tuning %f > ±36", i, s.Note, s.Tuning)
		}
	}

	if len(builder.Warnings) > 0 {
		t.Logf("warnings: %v", builder.Warnings)
	}
}

func TestMultisampleAssign_TooFewSamples(t *testing.T) {
	samples := []*SampleRef{{Name: "solo_C3"}}
	builder := &MultisampleBuilder{}
	slots := builder.Assign(samples)
	if slots != nil {
		t.Error("expected nil for single sample input")
	}
}
