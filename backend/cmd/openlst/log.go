package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

// setupLog skriver loggen til <datamappe>/openlst.log (på Windows har
// openlst.exe intet konsolvindue). Loggen indeholder aldrig tekst eller lyd –
// kun tekniske hændelser, længder og tider. Den nulstilles over 1 MB.
func setupLog(dataDir string) func() {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return func() {}
	}
	path := filepath.Join(dataDir, "openlst.log")
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if st, err := os.Stat(path); err == nil && st.Size() > 1<<20 {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		return func() {}
	}
	log.SetOutput(io.MultiWriter(f, os.Stderr))
	return func() { f.Close() }
}

// fatal logger fejlen, viser den for brugeren og afslutter.
func fatal(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	log.Print(msg)
	if runtime.GOOS == "windows" {
		showError(msg)
	}
	os.Exit(1)
}
