package bench

import "testing"

func TestWER(t *testing.T) {
	cases := []struct {
		ref, hyp string
		want     float64
	}{
		{"Jeg cykler i skole i morgen.", "jeg cykler i skole i morgen", 0},
		{"Jeg cykler i skole i morgen.", "Jeg søkler i Skål i morgen.", 2.0 / 6},
		{"to ord", "", 1},
	}
	for _, c := range cases {
		if got := WER(c.ref, c.hyp); got != c.want {
			t.Errorf("WER(%q,%q) = %v, ville have %v", c.ref, c.hyp, got, c.want)
		}
	}
}
