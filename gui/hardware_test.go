package gui

import "testing"

func TestDetectSystemCapacityPositiveRAM(t *testing.T) {
	cap := detectSystemCapacity()
	if cap.TotalRAMGB <= 0 {
		t.Errorf("expected positive RAM, got %.2f GB", cap.TotalRAMGB)
	}
}
