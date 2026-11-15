package tts

import "testing"

func TestLengthScale(t *testing.T) {
	for speed, want := range map[float64]float64{0.9: 1.11, 1.0: 1.0, 0.6: 1.67, 1.4: 0.71} {
		if got := lengthScale(speed); got != want {
			t.Errorf("lengthScale(%v) = %v, ville have %v", speed, got, want)
		}
	}
}

func TestClampSpeed(t *testing.T) {
	for in, want := range map[float64]float64{0: 0.9, 0.1: 0.6, 5: 1.4, 1.1: 1.1} {
		if got := ClampSpeed(in); got != want {
			t.Errorf("ClampSpeed(%v) = %v", in, got)
		}
	}
}

func TestSanitize(t *testing.T) {
	if got := sanitize("Hej\nmed\r\ndig\t ! "); got != "Hej med dig !" {
		t.Errorf("%q", got)
	}
}
