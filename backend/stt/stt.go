// Package stt definerer tale-til-tekst-motorer.
package stt

import (
	"context"
	"errors"
	"regexp"
	"strings"
)

// Transcript er resultatet af en transskription.
type Transcript struct {
	Text     string
	Language string
}

// Engine er en lokal STT-motor. wav er 16 kHz mono 16-bit PCM.
type Engine interface {
	Transcribe(ctx context.Context, wav []byte, lang string) (Transcript, error)
	Ready() bool
}

var ErrUnavailable = errors.New("stt er ikke tilgængelig")

// Whisper markerer stilhed og lyde med fx [BLANK_AUDIO], (musik) eller *hoster*.
var artefactRe = regexp.MustCompile(`\[[^\]]*\]|\([^)]*\)|\*[^*]*\*`)

// Clean fjerner Whisper-artefakter og samler teksten på én linje.
func Clean(s string) string {
	s = artefactRe.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(s), " ")
}
