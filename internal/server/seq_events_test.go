package server

import (
	"testing"

	"github.com/maxgarvey/mpc_editor/internal/seq"
)

// chromatic maps note byte → pad index using the default MPC chromatic layout (note 35 = pad 0).
func chromatic(note byte) int {
	idx := int(note) - 35
	if idx < 0 || idx >= 64 {
		return 0
	}
	return idx
}

func chromaticNote(pad int) byte { return byte(pad + 35) }

func ev(tick uint32, pad int) seq.Event {
	return seq.Event{Tick: tick, Type: seq.EventNoteOn, Note: chromaticNote(pad), Velocity: 100, Duration: 23}
}

// --- seqToggle ---

func TestSeqToggle_Add(t *testing.T) {
	out := seqToggle(nil, 0, 0, chromaticNote(0), 100, 23, chromatic)
	if len(out) != 1 {
		t.Fatalf("len = %d, want 1", len(out))
	}
	if out[0].Tick != 0 || out[0].Note != chromaticNote(0) {
		t.Errorf("unexpected event: %+v", out[0])
	}
}

func TestSeqToggle_Remove(t *testing.T) {
	events := []seq.Event{ev(0, 0), ev(24, 1)}
	out := seqToggle(events, 0, 0, chromaticNote(0), 100, 23, chromatic)
	if len(out) != 1 {
		t.Fatalf("len = %d, want 1 (removed pad 0 at tick 0)", len(out))
	}
	if out[0].Tick != 24 {
		t.Errorf("wrong event survived: %+v", out[0])
	}
}

func TestSeqToggle_AddPreservesOthers(t *testing.T) {
	events := []seq.Event{ev(24, 1)}
	out := seqToggle(events, 0, 0, chromaticNote(0), 80, 12, chromatic)
	if len(out) != 2 {
		t.Fatalf("len = %d, want 2", len(out))
	}
}

func TestSeqToggle_VelocityDuration(t *testing.T) {
	out := seqToggle(nil, 48, 3, chromaticNote(3), 64, 11, chromatic)
	if out[0].Velocity != 64 || out[0].Duration != 11 {
		t.Errorf("velocity=%d duration=%d, want 64 11", out[0].Velocity, out[0].Duration)
	}
}

// --- seqDelete ---

func TestSeqDelete_RemovesFirst(t *testing.T) {
	events := []seq.Event{ev(0, 0), ev(0, 0), ev(24, 1)}
	out := seqDelete(events, 0, 0, chromatic)
	if len(out) != 2 {
		t.Fatalf("len = %d, want 2 (only first duplicate removed)", len(out))
	}
}

func TestSeqDelete_NoMatch(t *testing.T) {
	events := []seq.Event{ev(0, 1)}
	out := seqDelete(events, 0, 0, chromatic)
	if len(out) != 1 {
		t.Fatalf("len = %d, want 1 (nothing deleted)", len(out))
	}
}

// --- seqMove ---

func TestSeqMove_Basic(t *testing.T) {
	events := []seq.Event{ev(0, 0), ev(24, 1)}
	out := seqMove(events, 0, 0, 48, 2, chromaticNote(2), chromatic)
	if len(out) != 2 {
		t.Fatalf("len = %d, want 2", len(out))
	}
	moved := out[0]
	if moved.Tick != 48 || moved.Note != chromaticNote(2) {
		t.Errorf("moved event wrong: %+v", moved)
	}
}

func TestSeqMove_DiscardsDestination(t *testing.T) {
	// pad 1 already at tick 48; moving pad 0 from tick 0 to (48, pad 1) should discard it
	events := []seq.Event{ev(0, 0), ev(48, 1)}
	out := seqMove(events, 0, 0, 48, 1, chromaticNote(1), chromatic)
	if len(out) != 1 {
		t.Fatalf("len = %d, want 1 (destination displaced)", len(out))
	}
	if out[0].Tick != 48 || chromatic(out[0].Note) != 1 {
		t.Errorf("wrong surviving event: %+v", out[0])
	}
}

func TestSeqMove_NoMatch(t *testing.T) {
	events := []seq.Event{ev(24, 1)}
	out := seqMove(events, 0, 0, 48, 2, chromaticNote(2), chromatic)
	if len(out) != 1 || out[0].Tick != 24 {
		t.Errorf("unmatched move should not change events: %+v", out)
	}
}

// --- seqUpdate ---

func TestSeqUpdate_ChangesVelocityDuration(t *testing.T) {
	events := []seq.Event{ev(0, 0)}
	out := seqUpdate(events, 0, 0, 50, 7, chromatic)
	if out[0].Velocity != 50 || out[0].Duration != 7 {
		t.Errorf("velocity=%d duration=%d, want 50 7", out[0].Velocity, out[0].Duration)
	}
}

func TestSeqUpdate_NoMatch(t *testing.T) {
	events := []seq.Event{ev(0, 1)}
	out := seqUpdate(events, 0, 0, 50, 7, chromatic)
	if out[0].Velocity != 100 {
		t.Errorf("unmatched update changed event: %+v", out[0])
	}
}

func TestSeqUpdate_OnlyFirst(t *testing.T) {
	events := []seq.Event{ev(0, 0), ev(0, 0)}
	out := seqUpdate(events, 0, 0, 50, 7, chromatic)
	// seqUpdate breaks after first match
	if out[0].Velocity != 50 {
		t.Errorf("first not updated: velocity=%d", out[0].Velocity)
	}
}

// --- seqMultiDelete ---

func TestSeqMultiDelete_RemovesTargets(t *testing.T) {
	events := []seq.Event{ev(0, 0), ev(24, 1), ev(48, 2)}
	targets := []multiTarget{{Pad: 0, GlobalStep: 0}, {Pad: 1, GlobalStep: 1}}
	out := seqMultiDelete(events, targets, 24, chromatic)
	if len(out) != 1 || chromatic(out[0].Note) != 2 {
		t.Errorf("unexpected result: %+v", out)
	}
}

func TestSeqMultiDelete_DuplicateCounts(t *testing.T) {
	events := []seq.Event{ev(0, 0), ev(0, 0), ev(0, 0)}
	targets := []multiTarget{{Pad: 0, GlobalStep: 0}, {Pad: 0, GlobalStep: 0}}
	out := seqMultiDelete(events, targets, 24, chromatic)
	if len(out) != 1 {
		t.Fatalf("len = %d, want 1 (two deleted, one kept)", len(out))
	}
}

// --- seqMultiMove ---

func TestSeqMultiMove_Basic(t *testing.T) {
	events := []seq.Event{ev(0, 0), ev(24, 1)}
	targets := []multiMoveTarget{
		{Pad: 0, GlobalStep: 0, ToPad: 2, ToGlobalStep: 2},
	}
	out := seqMultiMove(events, targets, 24, chromatic, chromaticNote)
	if len(out) != 2 {
		t.Fatalf("len = %d, want 2", len(out))
	}
	var found bool
	for _, e := range out {
		if e.Tick == 48 && chromatic(e.Note) == 2 {
			found = true
		}
	}
	if !found {
		t.Errorf("moved event not found: %+v", out)
	}
}

func TestSeqMultiMove_ExplicitTicks(t *testing.T) {
	from, to := 5, 99
	events := []seq.Event{ev(uint32(from), 0)}
	targets := []multiMoveTarget{
		{Pad: 0, GlobalStep: 0, ToPad: 1, ToGlobalStep: 0, FromTick: &from, ToTick: &to},
	}
	out := seqMultiMove(events, targets, 24, chromatic, chromaticNote)
	if len(out) != 1 || out[0].Tick != uint32(to) {
		t.Errorf("explicit tick not used: %+v", out)
	}
}

func TestSeqMultiMove_DiscardsPreexistingDest(t *testing.T) {
	events := []seq.Event{ev(0, 0), ev(48, 2)}
	targets := []multiMoveTarget{
		{Pad: 0, GlobalStep: 0, ToPad: 2, ToGlobalStep: 2},
	}
	out := seqMultiMove(events, targets, 24, chromatic, chromaticNote)
	if len(out) != 1 {
		t.Fatalf("len = %d, want 1 (destination displaced)", len(out))
	}
}

// --- seqMultiUpdate ---

func TestSeqMultiUpdate_UpdatesAll(t *testing.T) {
	events := []seq.Event{ev(0, 0), ev(24, 1), ev(48, 2)}
	targets := []multiTarget{{Pad: 0, GlobalStep: 0}, {Pad: 1, GlobalStep: 1}}
	out := seqMultiUpdate(events, targets, 24, 60, 5, chromatic)
	for _, e := range out[:2] {
		if e.Velocity != 60 || e.Duration != 5 {
			t.Errorf("not updated: %+v", e)
		}
	}
	if out[2].Velocity != 100 {
		t.Errorf("untargeted event was modified: %+v", out[2])
	}
}

// --- seqQuantizeOne ---

func TestSeqQuantizeOne_Snaps(t *testing.T) {
	events := []seq.Event{{Tick: 10, Type: seq.EventNoteOn, Note: chromaticNote(0), Velocity: 100, Duration: 23}}
	out := seqQuantizeOne(events, 10, 0, 24, chromatic)
	if out[0].Tick != 0 {
		t.Errorf("tick = %d, want 0 (snapped to nearest 24)", out[0].Tick)
	}
}

func TestSeqQuantizeOne_OnlyFirst(t *testing.T) {
	events := []seq.Event{
		{Tick: 10, Type: seq.EventNoteOn, Note: chromaticNote(0), Velocity: 100, Duration: 23},
		{Tick: 10, Type: seq.EventNoteOn, Note: chromaticNote(0), Velocity: 100, Duration: 23},
	}
	out := seqQuantizeOne(events, 10, 0, 24, chromatic)
	if out[1].Tick != 10 {
		t.Errorf("second event should be unmodified, got tick=%d", out[1].Tick)
	}
}

// --- seqMultiQuantize ---

func TestSeqMultiQuantize_QuantizesTargets(t *testing.T) {
	// Events sit at exact step-24 boundaries; targets identify them by GlobalStep.
	// Quantizing to 96-tick grid: tick 0→0, tick 24→0 (nearest 96), tick 96 untouched.
	events := []seq.Event{
		{Tick: 0, Type: seq.EventNoteOn, Note: chromaticNote(0), Velocity: 100, Duration: 23},
		{Tick: 24, Type: seq.EventNoteOn, Note: chromaticNote(1), Velocity: 100, Duration: 23},
		{Tick: 96, Type: seq.EventNoteOn, Note: chromaticNote(2), Velocity: 100, Duration: 23},
	}
	targets := []multiTarget{{Pad: 0, GlobalStep: 0}, {Pad: 1, GlobalStep: 1}}
	out := seqMultiQuantize(events, targets, 24, 96, chromatic)
	if out[0].Tick != 0 {
		t.Errorf("pad0 tick = %d, want 0", out[0].Tick)
	}
	if out[1].Tick != 0 {
		t.Errorf("pad1 tick = %d, want 0 (tick 24 snaps to nearest 96=0)", out[1].Tick)
	}
	if out[2].Tick != 96 {
		t.Errorf("untargeted event changed: tick = %d", out[2].Tick)
	}
}
