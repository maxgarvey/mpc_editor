package server

import (
	"encoding/json"
	"net/http"

	"github.com/maxgarvey/mpc_editor/internal/seq"
)

// seqToggle adds the event at (targetTick, note) if absent, removes it if present.
func seqToggle(events []seq.Event, targetTick uint32, padIndex int, note byte, velocity, duration int, padForNote func(byte) int) []seq.Event {
	out := make([]seq.Event, 0, len(events))
	found := false
	for _, ev := range events {
		if ev.Tick == targetTick && padForNote(ev.Note) == padIndex {
			found = true
			continue
		}
		out = append(out, ev)
	}
	if !found {
		out = append(out, seq.Event{
			Tick:     targetTick,
			Type:     seq.EventNoteOn,
			Note:     note,
			Velocity: byte(velocity),
			Duration: uint16(duration),
		})
	}
	return out
}

// seqMove relocates the first event at (fromTick, fromPad) to (toTick, toNote),
// discarding any pre-existing event at the destination.
func seqMove(events []seq.Event, fromTick uint32, fromPad int, toTick uint32, toPad int, toNote byte, padForNote func(byte) int) []seq.Event {
	out := make([]seq.Event, 0, len(events))
	moved := false
	for _, ev := range events {
		if ev.Tick == fromTick && padForNote(ev.Note) == fromPad && !moved {
			ev.Tick = toTick
			ev.Note = toNote
			moved = true
			out = append(out, ev)
		} else if ev.Tick == toTick && padForNote(ev.Note) == toPad {
			continue // discard pre-existing event at destination
		} else {
			out = append(out, ev)
		}
	}
	return out
}

// seqDelete removes the first event at (targetTick, padIndex).
func seqDelete(events []seq.Event, targetTick uint32, padIndex int, padForNote func(byte) int) []seq.Event {
	out := make([]seq.Event, 0, len(events))
	removed := false
	for _, ev := range events {
		if ev.Tick == targetTick && padForNote(ev.Note) == padIndex && !removed {
			removed = true
			continue
		}
		out = append(out, ev)
	}
	return out
}

// seqUpdate sets velocity and duration on the first event at (targetTick, padIndex).
func seqUpdate(events []seq.Event, targetTick uint32, padIndex, velocity, duration int, padForNote func(byte) int) []seq.Event {
	for i, ev := range events {
		if ev.Tick == targetTick && padForNote(ev.Note) == padIndex {
			events[i].Velocity = byte(velocity)
			events[i].Duration = uint16(duration)
			break
		}
	}
	return events
}

type multiTarget struct {
	Pad        int `json:"pad"`
	GlobalStep int `json:"global_step"`
}

type multiMoveTarget struct {
	Pad          int  `json:"pad"`
	GlobalStep   int  `json:"global_step"`
	ToPad        int  `json:"to_pad"`
	ToGlobalStep int  `json:"to_global_step"`
	FromTick     *int `json:"from_tick,omitempty"`
	ToTick       *int `json:"to_tick,omitempty"`
}

func parseMultiTargets(r *http.Request) ([]multiTarget, error) {
	var targets []multiTarget
	err := json.Unmarshal([]byte(r.FormValue("events")), &targets)
	return targets, err
}

func parseMultiMoveTargets(r *http.Request) ([]multiMoveTarget, error) {
	var targets []multiMoveTarget
	err := json.Unmarshal([]byte(r.FormValue("events")), &targets)
	return targets, err
}

// seqMultiDelete removes all events matching the given (tick, pad) pairs.
func seqMultiDelete(events []seq.Event, targets []multiTarget, ticksPerStep int, padForNote func(byte) int) []seq.Event {
	type key struct {
		tick uint32
		pad  int
	}
	counts := make(map[key]int, len(targets))
	for _, t := range targets {
		counts[key{uint32(t.GlobalStep * ticksPerStep), t.Pad}]++
	}
	out := make([]seq.Event, 0, len(events))
	for _, ev := range events {
		k := key{ev.Tick, padForNote(ev.Note)}
		if counts[k] > 0 {
			counts[k]--
			continue
		}
		out = append(out, ev)
	}
	return out
}

// seqMultiMove relocates all events specified by targets; discards pre-existing events at destinations.
func seqMultiMove(events []seq.Event, targets []multiMoveTarget, ticksPerStep int, padForNote func(byte) int, padToNote func(int) byte) []seq.Event {
	type key struct {
		tick uint32
		pad  int
	}
	type dest struct {
		toTick uint32
		toNote byte
	}
	moveMap := make(map[key]dest, len(targets))
	destSet := make(map[key]bool, len(targets))
	for _, t := range targets {
		fromTick := uint32(t.GlobalStep * ticksPerStep)
		if t.FromTick != nil {
			fromTick = uint32(*t.FromTick)
		}
		toTick := uint32(t.ToGlobalStep * ticksPerStep)
		if t.ToTick != nil {
			toTick = uint32(*t.ToTick)
		}
		moveMap[key{fromTick, t.Pad}] = dest{toTick, padToNote(t.ToPad)}
		destSet[key{toTick, t.ToPad}] = true
	}
	out := make([]seq.Event, 0, len(events))
	for _, ev := range events {
		k := key{ev.Tick, padForNote(ev.Note)}
		if d, ok := moveMap[k]; ok {
			ev.Tick = d.toTick
			ev.Note = d.toNote
			delete(moveMap, k)
			out = append(out, ev)
		} else if destSet[k] {
			continue // discard pre-existing event at a move destination
		} else {
			out = append(out, ev)
		}
	}
	return out
}

// seqMultiUpdate applies velocity and duration to all events matching the given (tick, pad) pairs.
func seqMultiUpdate(events []seq.Event, targets []multiTarget, ticksPerStep, velocity, duration int, padForNote func(byte) int) []seq.Event {
	type key struct {
		tick uint32
		pad  int
	}
	toUpdate := make(map[key]bool, len(targets))
	for _, t := range targets {
		toUpdate[key{uint32(t.GlobalStep * ticksPerStep), t.Pad}] = true
	}
	for i, ev := range events {
		if toUpdate[key{ev.Tick, padForNote(ev.Note)}] {
			events[i].Velocity = byte(velocity)
			events[i].Duration = uint16(duration)
		}
	}
	return events
}

// seqQuantizeOne quantizes the first event at (sourceTick, padIndex) to the nearest qTicks boundary.
func seqQuantizeOne(events []seq.Event, sourceTick uint32, padIndex, qTicks int, padForNote func(byte) int) []seq.Event {
	for i, ev := range events {
		if ev.Tick == sourceTick && padForNote(ev.Note) == padIndex {
			events[i].Tick = quantizeTick(ev.Tick, qTicks)
			break
		}
	}
	return events
}

// seqMultiQuantize quantizes all events matching the given (tick, pad) pairs.
func seqMultiQuantize(events []seq.Event, targets []multiTarget, ticksPerStep, qTicks int, padForNote func(byte) int) []seq.Event {
	type key struct {
		tick uint32
		pad  int
	}
	toQuantize := make(map[key]bool, len(targets))
	for _, t := range targets {
		toQuantize[key{uint32(t.GlobalStep * ticksPerStep), t.Pad}] = true
	}
	for i, ev := range events {
		if toQuantize[key{ev.Tick, padForNote(ev.Note)}] {
			events[i].Tick = quantizeTick(ev.Tick, qTicks)
		}
	}
	return events
}
