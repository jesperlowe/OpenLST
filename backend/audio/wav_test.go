package audio

import (
	"testing"
	"time"
)

func TestRoundTrip(t *testing.T) {
	b := EncodeWAV(make([]byte, 32000), 16000, 1)
	f, err := ParseWAV(b)
	if err != nil {
		t.Fatal(err)
	}
	if f.SampleRate != 16000 || f.Channels != 1 || f.BitsPerSample != 16 || f.Duration() != time.Second {
		t.Fatalf("%+v %v", f, f.Duration())
	}
}

func TestRejects(t *testing.T) {
	for _, b := range [][]byte{nil, []byte("RIFF"), []byte("RIFF\x00\x00\x00\x00WAVEjunk")} {
		if _, err := ParseWAV(b); err == nil {
			t.Errorf("accepterede %q", b)
		}
	}
	b := EncodeWAV(make([]byte, 10), 16000, 1)
	b[20] = 3 // float-format
	if _, err := ParseWAV(b); err == nil {
		t.Error("accepterede ikke-PCM")
	}
}
