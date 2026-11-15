// Package tts definerer tekst-til-tale-motorer.
package tts

import (
	"context"
	"errors"
)

// Request er én oplæsning. Teksten er typisk én sætning.
type Request struct {
	Text  string
	Speed float64 // 0.6–1.4, 1.0 = normal
}

// Result er genereret lyd som WAV.
type Result struct {
	WAV []byte
	// Senere: ord-timings til ord-for-ord-highlight.
}

// Engine er en lokal TTS-motor. Implementeringer skal være sikre at kalde fra
// flere goroutines.
type Engine interface {
	Synthesize(ctx context.Context, req Request) (Result, error)
	Ready() bool
	Close() error
}

const (
	MinSpeed     = 0.6
	MaxSpeed     = 1.4
	DefaultSpeed = 0.9
)

var ErrUnavailable = errors.New("tts er ikke tilgængelig")

// ClampSpeed holder hastigheden inden for det tilladte område.
func ClampSpeed(s float64) float64 {
	if s == 0 {
		return DefaultSpeed
	}
	return min(MaxSpeed, max(MinSpeed, s))
}
