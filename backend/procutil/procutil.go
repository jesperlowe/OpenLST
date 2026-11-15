// Package procutil indeholder platformsspecifik opsætning af underprocesser.
package procutil

import (
	"os/exec"
	"path/filepath"
	"time"
)

// Usage er ressourceforbrug for en afsluttet underproces.
type Usage struct {
	Wall    time.Duration
	CPU     time.Duration // user + kernel
	PeakRAM uint64        // bytes; 0 hvis ukendt
}

// RunMeasured kører cmd til ende og måler tid, CPU og peak RAM.
func RunMeasured(cmd *exec.Cmd) (Usage, error) {
	start := time.Now()
	if err := cmd.Start(); err != nil {
		return Usage{}, err
	}
	peak := watchPeak(cmd)
	err := cmd.Wait()
	u := Usage{Wall: time.Since(start)}
	if cmd.ProcessState != nil {
		u.CPU = cmd.ProcessState.UserTime() + cmd.ProcessState.SystemTime()
	}
	u.PeakRAM = peak()
	return u, err
}

// RelArg returnerer target relativt til base, hvis det kan lade sig gøre.
//
// Underprocesser startes med Dir=base. Relative stier undgår problemer med
// æ/ø/å i Windows-brugernavne (C:\Users\Søren\...), som C++-programmer som
// piper.exe og whisper-cli.exe ikke altid håndterer.
//
// Symlinks/junctions opløses først (fx er macOS' /var et link til /private/var),
// ellers ville den relative sti pege forkert.
func RelArg(base, target string) string {
	b, err1 := filepath.EvalSymlinks(base)
	t, err2 := filepath.EvalSymlinks(target)
	if err1 != nil || err2 != nil {
		return target
	}
	if r, err := filepath.Rel(b, t); err == nil {
		return r
	}
	return target
}
