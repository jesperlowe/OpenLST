package stt

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"

	"openlst/backend/procutil"
)

// WhisperCLI kører whisper-cli én gang pr. diktering.
//
// Valgt frem for whisper-server, fordi serveren åbner en ekstra HTTP-port, som
// hjemmesider i browseren kan nå. Desuden frigives RAM mellem dikteringer.
// Prisen er at modellen indlæses ved hvert kald (lille for tiny/base).
type WhisperCLI struct {
	bin, model string
	threads    int
	extraArgs  []string

	mu sync.Mutex // én transskription ad gangen – CPU'en er den begrænsende faktor
}

func NewWhisperCLI(bin, model string, threads int, extraArgs []string) (*WhisperCLI, error) {
	if bin == "" {
		return nil, fmt.Errorf("%w: whisper-cli blev ikke fundet", ErrUnavailable)
	}
	if _, err := os.Stat(model); err != nil {
		return nil, fmt.Errorf("%w: model mangler: %s", ErrUnavailable, filepath.Base(model))
	}
	return &WhisperCLI{bin: bin, model: model, threads: threads, extraArgs: extraArgs}, nil
}

func (w *WhisperCLI) Ready() bool { return w != nil }

func (w *WhisperCLI) Transcribe(ctx context.Context, wav []byte, lang string) (Transcript, error) {
	t, _, err := w.TranscribeMeasured(ctx, wav, lang)
	return t, err
}

// TranscribeMeasured transskriberer og måler tid, CPU og peak RAM (benchmark).
func (w *WhisperCLI) TranscribeMeasured(ctx context.Context, wav []byte, lang string) (Transcript, procutil.Usage, error) {
	if lang == "" {
		lang = "da"
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	// whisper-cli læser fra fil. Filen ligger i en privat mappe og slettes
	// altid – også ved fejl.
	dir, err := os.MkdirTemp("", "openlst-stt-")
	if err != nil {
		return Transcript{}, procutil.Usage{}, err
	}
	defer os.RemoveAll(dir)
	if err := os.WriteFile(filepath.Join(dir, "in.wav"), wav, 0o600); err != nil {
		return Transcript{}, procutil.Usage{}, err
	}

	// Kør fra temp-mappen med relative stier (æ/ø/å i brugernavne, se RelArg).
	args := []string{
		"-m", procutil.RelArg(dir, w.model),
		"-l", lang,
		"-t", strconv.Itoa(w.threads),
		"-nt", // ingen tidsstempler
		"-np", // kun resultatet på stdout
		"-f", "in.wav",
	}
	args = append(args, w.extraArgs...)
	cmd := exec.CommandContext(ctx, w.bin, args...)
	cmd.Dir = dir
	procutil.Hide(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	u, err := procutil.RunMeasured(cmd)
	if err != nil {
		if ctx.Err() != nil {
			return Transcript{}, u, ctx.Err()
		}
		// stderr kan indeholde fejlbeskeder, men aldrig brugerens tale.
		msg := stderr.String()
		if len(msg) > 300 {
			msg = msg[len(msg)-300:]
		}
		return Transcript{}, u, fmt.Errorf("whisper-cli fejlede: %v: %s", err, msg)
	}
	return Transcript{Text: Clean(stdout.String()), Language: lang}, u, nil
}
