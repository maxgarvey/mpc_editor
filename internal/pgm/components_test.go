package pgm

import "testing"

// Envelope ---------------------------------------------------------------

func TestEnvelopeGetSet(t *testing.T) {
	prog := NewProgram()
	env := prog.Pad(0).Envelope()

	env.SetAttack(50)
	if got := env.GetAttack(); got != 50 {
		t.Errorf("attack = %d, want 50", got)
	}
	env.SetDecay(75)
	if got := env.GetDecay(); got != 75 {
		t.Errorf("decay = %d, want 75", got)
	}
	env.SetDecayMode(1)
	if got := env.GetDecayMode(); got != 1 {
		t.Errorf("decay mode = %d, want 1", got)
	}
	env.SetVelocityToLevel(30)
	if got := env.GetVelocityToLevel(); got != 30 {
		t.Errorf("vel to level = %d, want 30", got)
	}
}

func TestEnvelopeCopyFrom(t *testing.T) {
	prog := NewProgram()
	src := prog.Pad(0).Envelope()
	dst := prog.Pad(1).Envelope()

	src.SetAttack(42)
	src.SetDecay(88)
	src.SetDecayMode(1)
	src.SetVelocityToLevel(55)

	dst.CopyFrom(src)

	if dst.GetAttack() != 42 {
		t.Errorf("copied attack = %d, want 42", dst.GetAttack())
	}
	if dst.GetDecay() != 88 {
		t.Errorf("copied decay = %d, want 88", dst.GetDecay())
	}
	if dst.GetDecayMode() != 1 {
		t.Errorf("copied decay mode = %d, want 1", dst.GetDecayMode())
	}
	if dst.GetVelocityToLevel() != 55 {
		t.Errorf("copied vel to level = %d, want 55", dst.GetVelocityToLevel())
	}
}

// Filter1 ----------------------------------------------------------------

func TestFilter1GetSet(t *testing.T) {
	prog := NewProgram()
	f := prog.Pad(0).Filter1()

	f.SetType(2)
	if got := f.GetType(); got != 2 {
		t.Errorf("type = %d, want 2", got)
	}
	f.SetFrequency(60)
	if got := f.GetFrequency(); got != 60 {
		t.Errorf("freq = %d, want 60", got)
	}
	f.SetResonance(80)
	if got := f.GetResonance(); got != 80 {
		t.Errorf("res = %d, want 80", got)
	}
	f.SetVelocityToFreq(25)
	if got := f.GetVelocityToFreq(); got != 25 {
		t.Errorf("vel to freq = %d, want 25", got)
	}
	f.SetAttenuation(1)
	if got := f.GetAttenuation(); got != 1 {
		t.Errorf("attenuation = %d, want 1", got)
	}
}

func TestFilter1CopyFrom(t *testing.T) {
	prog := NewProgram()
	src := prog.Pad(0).Filter1()
	dst := prog.Pad(1).Filter1()

	src.SetType(1)
	src.SetFrequency(50)
	src.SetResonance(40)
	src.SetVelocityToFreq(10)
	src.SetAttenuation(2)

	dst.CopyFrom(src)

	if dst.GetType() != 1 {
		t.Errorf("type = %d, want 1", dst.GetType())
	}
	if dst.GetFrequency() != 50 {
		t.Errorf("freq = %d, want 50", dst.GetFrequency())
	}
	if dst.GetAttenuation() != 2 {
		t.Errorf("attenuation = %d, want 2", dst.GetAttenuation())
	}
}

// Filter2 ----------------------------------------------------------------

func TestFilter2GetSet(t *testing.T) {
	prog := NewProgram()
	f := prog.Pad(0).Filter2()

	f.SetType(3)
	if got := f.GetType(); got != 3 {
		t.Errorf("type = %d, want 3", got)
	}
	f.SetFrequency(70)
	if got := f.GetFrequency(); got != 70 {
		t.Errorf("freq = %d, want 70", got)
	}
	f.SetResonance(45)
	if got := f.GetResonance(); got != 45 {
		t.Errorf("res = %d, want 45", got)
	}
	f.SetVelocityToFreq(15)
	if got := f.GetVelocityToFreq(); got != 15 {
		t.Errorf("vel to freq = %d, want 15", got)
	}
}

func TestFilter2CopyFrom(t *testing.T) {
	prog := NewProgram()
	src := prog.Pad(0).Filter2()
	dst := prog.Pad(1).Filter2()

	src.SetType(2)
	src.SetFrequency(65)
	src.SetResonance(35)
	src.SetVelocityToFreq(20)

	dst.CopyFrom(src)

	if dst.GetType() != 2 {
		t.Errorf("type = %d, want 2", dst.GetType())
	}
	if dst.GetFrequency() != 65 {
		t.Errorf("freq = %d, want 65", dst.GetFrequency())
	}
}

// Mixer ------------------------------------------------------------------

func TestMixerGetSet(t *testing.T) {
	prog := NewProgram()
	m := prog.Pad(0).Mixer()

	m.SetLevel(75)
	if got := m.GetLevel(); got != 75 {
		t.Errorf("level = %d, want 75", got)
	}
	m.SetPan(30)
	if got := m.GetPan(); got != 30 {
		t.Errorf("pan = %d, want 30", got)
	}
	m.SetOutput(1)
	if got := m.GetOutput(); got != 1 {
		t.Errorf("output = %d, want 1", got)
	}
	m.SetFXSend(2)
	if got := m.GetFXSend(); got != 2 {
		t.Errorf("fx send = %d, want 2", got)
	}
	m.SetFXSendLevel(50)
	if got := m.GetFXSendLevel(); got != 50 {
		t.Errorf("fx send level = %d, want 50", got)
	}
}

func TestMixerCopyFrom(t *testing.T) {
	prog := NewProgram()
	src := prog.Pad(0).Mixer()
	dst := prog.Pad(1).Mixer()

	src.SetLevel(88)
	src.SetPan(60)
	src.SetOutput(2)
	src.SetFXSend(1)
	src.SetFXSendLevel(33)

	dst.CopyFrom(src)

	if dst.GetLevel() != 88 {
		t.Errorf("level = %d, want 88", dst.GetLevel())
	}
	if dst.GetPan() != 60 {
		t.Errorf("pan = %d, want 60", dst.GetPan())
	}
	if dst.GetOutput() != 2 {
		t.Errorf("output = %d, want 2", dst.GetOutput())
	}
}

// Layer ------------------------------------------------------------------

func TestLayerIndex(t *testing.T) {
	prog := NewProgram()
	for i := 0; i < 4; i++ {
		if idx := prog.Pad(0).Layer(i).Index(); idx != i {
			t.Errorf("layer[%d].Index() = %d", i, idx)
		}
	}
}

func TestLayerPlayMode(t *testing.T) {
	prog := NewProgram()
	l := prog.Pad(0).Layer(0)

	if !l.IsOneShot() {
		t.Error("default play mode should be one shot")
	}

	l.SetPlayMode(1)
	if l.GetPlayMode() != 1 {
		t.Errorf("play mode = %d, want 1", l.GetPlayMode())
	}
	if l.IsOneShot() {
		t.Error("IsOneShot should be false after SetPlayMode(1)")
	}

	l.SetPlayMode(0)
	if !l.IsOneShot() {
		t.Error("IsOneShot should be true after SetPlayMode(0)")
	}
}

func TestLayerRange(t *testing.T) {
	prog := NewProgram()
	l := prog.Pad(0).Layer(0)

	r := Range{Low: 10, High: 100}
	l.SetRange(r)
	got := l.GetRange()
	if got.Low != 10 || got.High != 100 {
		t.Errorf("range = {%d,%d}, want {10,100}", got.Low, got.High)
	}
}

func TestLayerGetSetParam(t *testing.T) {
	prog := NewProgram()
	l := prog.Pad(0).Layer(0)

	if err := l.SetParam(LayerLevel, 55); err != nil {
		t.Fatalf("SetParam: %v", err)
	}
	if got := l.GetParam(LayerLevel); got != 55 {
		t.Errorf("GetParam = %v, want 55", got)
	}
}

func TestLayerCopyFrom(t *testing.T) {
	prog := NewProgram()
	src := prog.Pad(0).Layer(0)
	dst := prog.Pad(1).Layer(0)

	src.SetLevel(77)
	src.SetPlayMode(1)
	src.SetTuning(6.0)

	dst.CopyFrom(src, nil)

	if dst.GetLevel() != 77 {
		t.Errorf("copied level = %d, want 77", dst.GetLevel())
	}
	if dst.GetPlayMode() != 1 {
		t.Errorf("copied play mode = %d, want 1", dst.GetPlayMode())
	}
	if dst.GetTuning() != 6.0 {
		t.Errorf("copied tuning = %f, want 6.0", dst.GetTuning())
	}
}

func TestLayerCopyFromWithIgnore(t *testing.T) {
	prog := NewProgram()
	src := prog.Pad(0).Layer(0)
	dst := prog.Pad(1).Layer(0)
	dst.SetLevel(99)

	src.SetLevel(77)
	dst.CopyFrom(src, map[string]bool{"Level": true})

	if dst.GetLevel() != 99 {
		t.Errorf("level should be unchanged when ignored, got %d", dst.GetLevel())
	}
}

// Pad --------------------------------------------------------------------

func TestPadIndex(t *testing.T) {
	prog := NewProgram()
	for i := 0; i < 8; i++ {
		if idx := prog.Pad(i).Index(); idx != i {
			t.Errorf("pad[%d].Index() = %d", i, idx)
		}
	}
}

func TestPadGetSetParam(t *testing.T) {
	prog := NewProgram()
	pad := prog.Pad(0)

	if err := pad.SetParam(PadMuteGroup, 3); err != nil {
		t.Fatalf("SetParam: %v", err)
	}
	if got := pad.GetParam(PadMuteGroup); got != 3 {
		t.Errorf("GetParam = %v, want 3", got)
	}
}

func TestPadCopyFrom(t *testing.T) {
	prog := NewProgram()
	src := prog.Pad(0)
	dst := prog.Pad(1)

	src.SetVoiceOverlap(1)
	src.SetMuteGroup(7)
	src.Envelope().SetAttack(33)
	src.Filter1().SetFrequency(44)
	src.Filter2().SetResonance(22)
	src.Mixer().SetLevel(66)
	_ = src.Layer(0).SetSampleName("kick")
	src.Layer(0).SetLevel(90)

	dst.CopyFrom(src, nil)

	if dst.GetVoiceOverlap() != 1 {
		t.Error("voice overlap not copied")
	}
	if dst.GetMuteGroup() != 7 {
		t.Error("mute group not copied")
	}
	if dst.Envelope().GetAttack() != 33 {
		t.Error("envelope not copied")
	}
	if dst.Filter1().GetFrequency() != 44 {
		t.Error("filter1 not copied")
	}
	if dst.Filter2().GetResonance() != 22 {
		t.Error("filter2 not copied")
	}
	if dst.Mixer().GetLevel() != 66 {
		t.Error("mixer not copied")
	}
	if dst.Layer(0).GetSampleName() != "kick" {
		t.Error("layer sample name not copied")
	}
	if dst.Layer(0).GetLevel() != 90 {
		t.Error("layer level not copied")
	}
}

func TestPadCopyFromWithIgnore(t *testing.T) {
	prog := NewProgram()
	src := prog.Pad(0)
	dst := prog.Pad(1)
	dst.SetMuteGroup(5)

	src.SetMuteGroup(99)
	dst.CopyFrom(src, map[string]bool{"Mute Group": true})

	if dst.GetMuteGroup() != 5 {
		t.Errorf("mute group should be unchanged when ignored, got %d", dst.GetMuteGroup())
	}
}

// Buffer -----------------------------------------------------------------

func TestBufferData(t *testing.T) {
	prog := NewProgram()
	data := prog.buf.Data()
	if len(data) != ProgramFileSize {
		t.Errorf("Data() len = %d, want %d", len(data), ProgramFileSize)
	}
	// Mutations to the slice should be visible through the program
	data[0] = 0xFF
	if prog.buf.GetByte(0) != 0xFF {
		t.Error("Data() should return a reference to the underlying slice")
	}
}
