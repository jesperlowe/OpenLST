package stt

import "testing"

func TestClean(t *testing.T) {
	cases := map[string]string{
		" Jeg cykler i skole i morgen.\n":      "Jeg cykler i skole i morgen.",
		"[BLANK_AUDIO]":                        "",
		"(musik) Hej med dig *hoster* [Lyd]  ": "Hej med dig",
	}
	for in, want := range cases {
		if got := Clean(in); got != want {
			t.Errorf("Clean(%q) = %q, ville have %q", in, got, want)
		}
	}
}
