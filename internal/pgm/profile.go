package pgm

// Profile defines hardware differences between MPC models.
type Profile struct {
	Name        string
	PadsPerBank int // 12 (MPC500: 4x3) or 16 (MPC1000: 4x4)
	SliderCount int // 1 (MPC500) or 2 (MPC1000)
	FilterCount int // 1 (MPC500) or 2 (MPC1000)
}

var (
	ProfileMPC500  = Profile{Name: "MPC500", PadsPerBank: 12, SliderCount: 1, FilterCount: 1}
	ProfileMPC1000 = Profile{Name: "MPC1000", PadsPerBank: 16, SliderCount: 2, FilterCount: 2}
)

// BankCount returns how many banks are needed to display all 64 pads.
func (p Profile) BankCount() int {
	return 64 / p.PadsPerBank
}
