package pgm

import "testing"

func TestValidate_TypeInt(t *testing.T) {
	p := LayerLevel // TypeInt, range [0, 100]

	if !p.Validate(42) {
		t.Error("int 42 should be valid")
	}
	if p.Validate("bad") {
		t.Error("string should be invalid for TypeInt")
	}
	if !p.Validate(byte(50)) {
		t.Error("byte should be valid")
	}
	if !p.Validate(int16(50)) {
		t.Error("int16 should be valid")
	}
	if !p.Validate(int32(50)) {
		t.Error("int32 should be valid")
	}
	if !p.Validate(int64(50)) {
		t.Error("int64 should be valid")
	}
	if p.Validate(int64(999)) {
		t.Error("int64 out of range should be invalid")
	}
}

func TestValidate_TypeString(t *testing.T) {
	p := LayerSampleName // TypeString

	if !p.Validate("hello") {
		t.Error("short string should be valid")
	}
	if p.Validate(42) {
		t.Error("int should be invalid for TypeString")
	}
}

func TestValidate_TypeTuning(t *testing.T) {
	p := LayerTuning // TypeTuning, range [-36, 36]

	if !p.Validate(12.5) {
		t.Error("float64 12.5 should be valid")
	}
	if !p.Validate(float32(12.5)) {
		t.Error("float32 should be valid")
	}
	if !p.Validate(int(12)) {
		t.Error("int should be valid as tuning")
	}
	if p.Validate("bad") {
		t.Error("string should be invalid for TypeTuning")
	}
}

func TestValidate_TypeRange(t *testing.T) {
	p := LayerRange

	if !p.Validate(Range{Low: 0, High: 127}) {
		t.Error("Range value should be valid")
	}
	if p.Validate(42) {
		t.Error("int should be invalid for TypeRange")
	}
}

func TestValidate_UnknownType(t *testing.T) {
	p := Parameter{Type: 255}
	if p.Validate(42) {
		t.Error("unknown type should always be invalid")
	}
}

func TestProgramBuffer(t *testing.T) {
	prog := NewProgram()
	buf := prog.Buffer()
	if buf == nil {
		t.Error("Buffer() should return non-nil")
	}
	if buf.Len() != ProgramFileSize {
		t.Errorf("Buffer().Len() = %d, want %d", buf.Len(), ProgramFileSize)
	}
}

func TestProgramSlider(t *testing.T) {
	prog := NewProgram()
	s := prog.Slider(0)
	if s == nil {
		t.Error("Slider(0) should return non-nil")
	}
}

func TestProfileBankCount(t *testing.T) {
	if n := ProfileMPC1000.BankCount(); n != 4 {
		t.Errorf("MPC1000 BankCount = %d, want 4", n)
	}
	if n := ProfileMPC500.BankCount(); n != 5 {
		t.Errorf("MPC500 BankCount = %d, want 5 (64/12 rounded up)", n)
	}
}
