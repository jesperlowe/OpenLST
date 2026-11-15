// Package audio indeholder små hjælpere til WAV-lyd (16-bit PCM).
package audio

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

// Format beskriver det PCM-format, en WAV-fil indeholder.
type Format struct {
	SampleRate    int
	Channels      int
	BitsPerSample int
	DataBytes     int
	DataOffset    int
}

// Duration returnerer lydens længde.
func (f Format) Duration() time.Duration {
	bytesPerSec := f.SampleRate * f.Channels * f.BitsPerSample / 8
	if bytesPerSec == 0 {
		return 0
	}
	return time.Duration(float64(f.DataBytes) / float64(bytesPerSec) * float64(time.Second))
}

var ErrNotWAV = errors.New("ikke en gyldig WAV-fil")

// ParseWAV læser RIFF-headeren og finder fmt- og data-chunk.
// Den accepterer kun ukomprimeret PCM (format 1).
func ParseWAV(b []byte) (Format, error) {
	var f Format
	if len(b) < 12 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return f, ErrNotWAV
	}
	pos := 12
	gotFmt := false
	for pos+8 <= len(b) {
		id := string(b[pos : pos+4])
		size := int(binary.LittleEndian.Uint32(b[pos+4 : pos+8]))
		body := pos + 8
		if size < 0 || body+size > len(b) {
			// Tillad at data-chunk er afkortet (nogle encodere skriver forkert størrelse).
			if id == "data" && gotFmt {
				f.DataBytes = len(b) - body
				f.DataOffset = body
				return f, nil
			}
			return f, ErrNotWAV
		}
		switch id {
		case "fmt ":
			if size < 16 {
				return f, ErrNotWAV
			}
			if binary.LittleEndian.Uint16(b[body:]) != 1 {
				return f, fmt.Errorf("%w: kun PCM understøttes", ErrNotWAV)
			}
			f.Channels = int(binary.LittleEndian.Uint16(b[body+2:]))
			f.SampleRate = int(binary.LittleEndian.Uint32(b[body+4:]))
			f.BitsPerSample = int(binary.LittleEndian.Uint16(b[body+14:]))
			gotFmt = true
		case "data":
			if !gotFmt {
				return f, ErrNotWAV
			}
			f.DataBytes = size
			f.DataOffset = body
			return f, nil
		}
		pos = body + size + size%2 // chunks er 2-byte-alignede
	}
	return f, ErrNotWAV
}

// EncodeWAV pakker rå 16-bit little-endian PCM i en WAV-header.
func EncodeWAV(pcm []byte, sampleRate, channels int) []byte {
	const bits = 16
	out := make([]byte, 44+len(pcm))
	copy(out[0:], "RIFF")
	binary.LittleEndian.PutUint32(out[4:], uint32(36+len(pcm)))
	copy(out[8:], "WAVE")
	copy(out[12:], "fmt ")
	binary.LittleEndian.PutUint32(out[16:], 16)
	binary.LittleEndian.PutUint16(out[20:], 1)
	binary.LittleEndian.PutUint16(out[22:], uint16(channels))
	binary.LittleEndian.PutUint32(out[24:], uint32(sampleRate))
	binary.LittleEndian.PutUint32(out[28:], uint32(sampleRate*channels*bits/8))
	binary.LittleEndian.PutUint16(out[32:], uint16(channels*bits/8))
	binary.LittleEndian.PutUint16(out[34:], bits)
	copy(out[36:], "data")
	binary.LittleEndian.PutUint32(out[40:], uint32(len(pcm)))
	copy(out[44:], pcm)
	return out
}

// Samples returnerer 16-bit mono-samples fra en WAV-fil.
func Samples(b []byte) ([]int16, Format, error) {
	f, err := ParseWAV(b)
	if err != nil {
		return nil, f, err
	}
	if f.BitsPerSample != 16 || f.Channels != 1 {
		return nil, f, fmt.Errorf("%w: kun 16-bit mono", ErrNotWAV)
	}
	data := b[f.DataOffset : f.DataOffset+f.DataBytes]
	out := make([]int16, len(data)/2)
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(data[2*i:]))
	}
	return out, f, nil
}

// Resample omsætter mono-samples til en ny samplerate med lineær
// interpolation. Godt nok til benchmark-lyd; browseren resampler selv ved
// diktering.
func Resample(in []int16, from, to int) []int16 {
	if from == to || len(in) == 0 {
		return in
	}
	n := int(int64(len(in)) * int64(to) / int64(from))
	out := make([]int16, n)
	step := float64(from) / float64(to)
	for i := range out {
		pos := float64(i) * step
		j := int(pos)
		if j+1 >= len(in) {
			out[i] = in[len(in)-1]
			continue
		}
		frac := pos - float64(j)
		out[i] = int16(float64(in[j])*(1-frac) + float64(in[j+1])*frac)
	}
	return out
}

// EncodeSamples pakker 16-bit mono-samples som WAV.
func EncodeSamples(s []int16, rate int) []byte {
	pcm := make([]byte, 2*len(s))
	for i, v := range s {
		binary.LittleEndian.PutUint16(pcm[2*i:], uint16(v))
	}
	return EncodeWAV(pcm, rate, 1)
}
