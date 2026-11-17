// Package bench er OpenLST's udvikler-benchmark (openlst --bench).
//
// Den måler TTS (Piper) og STT (alle Whisper-modeller i models/stt) på den
// aktuelle computer, så modeller kan sammenlignes på samme hardware, fx
// Tiny Q5, Tiny Q8 og Base Q5 på en Surface Go. Den er ikke synlig i
// børnenes brugerflade.
package bench

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"openlst/backend/audio"
	"openlst/backend/config"
	"openlst/backend/procutil"
	"openlst/backend/server"
	"openlst/backend/stt"
	"openlst/backend/tts"
)

const sttRuns = 3

var sentences = []string{
	"Jeg cykler i skole i morgen.",
	"Peter går ned til skolen i dag.",
	"Danmark har omkring seks millioner indbyggere.",
	"Min hund hedder Bobby, og den kan lide at lege i haven.",
	"Hvornår begynder fodboldtræningen på torsdag?",
	// Lang sætning og dens korte første bid (som brugerfladen sender først):
	"Da Peter kom hjem fra skole i går, var hans mor allerede gået på arbejde, så han lavede selv mad til sig og sin lillesøster.",
	"Da Peter kom hjem fra skole i går,",
}

type clip struct {
	name string
	ref  string // forventet tekst ("" = ukendt)
	wav  []byte // 16 kHz mono
	dur  time.Duration
}

// Run kører benchmarken, skriver rapporten til out og gemmer den som
// Markdown i dataDir. Returnerer stien til rapporten.
func Run(ctx context.Context, cfg *config.Config, dataDir string, out io.Writer) (string, error) {
	var md bytes.Buffer
	w := io.MultiWriter(&md, out)
	p := func(format string, a ...any) { fmt.Fprintf(w, format, a...) }

	p("# OpenLST benchmark\n\n")
	p("| | |\n|---|---|\n")
	p("| Tidspunkt | %s |\n", time.Now().Format("2006-01-02 15:04"))
	p("| OpenLST | %s |\n", server.Version)
	p("| System | %s/%s |\n", runtime.GOOS, runtime.GOARCH)
	p("| CPU | %s |\n", cpuName())
	p("| Logiske kerner | %d |\n", runtime.NumCPU())
	p("| Whisper-tråde | %d |\n\n", cfg.STTThreads)

	// ---------- TTS ----------
	p("## TTS – Piper (`%s`)\n\n", cfg.TTSVoice)
	if cold, err := piperCold(ctx, cfg); err != nil {
		p("⚠️ Koldstart kunne ikke måles: %v\n\n", err)
	} else {
		p("Koldstart (start + indlæs model + én sætning): **%d ms**, CPU %d ms, peak RAM **%s**\n\n",
			cold.Wall.Milliseconds(), cold.CPU.Milliseconds(), mb(cold.PeakRAM))
	}

	piper, err := tts.NewPiper(cfg.PiperPath, cfg.VoicePath())
	if err != nil {
		return "", fmt.Errorf("tts: %w", err)
	}
	defer piper.Close()
	warmStart := time.Now()
	piper.Warm()
	if !piper.Ready() {
		return "", fmt.Errorf("tts: piper kunne ikke starte")
	}
	p("Opvarmning ved opstart af OpenLST (start + model + første syntese): **%d ms**\n\n", time.Since(warmStart).Milliseconds())
	p("Tabellen herunder er *tid til første lyd*, når eleven trykker Læs (Piper er varm).\n\n")

	p("| Sætning | Tegn | Genereringstid | Lydlængde | RTF |\n|---|---:|---:|---:|---:|\n")
	var clips []clip
	for _, s := range sentences {
		start := time.Now()
		res, err := piper.Synthesize(ctx, tts.Request{Text: s, Speed: 1})
		el := time.Since(start)
		if err != nil {
			return "", fmt.Errorf("tts: %w", err)
		}
		samples, f, err := audio.Samples(res.WAV)
		if err != nil {
			return "", fmt.Errorf("tts-lyd: %w", err)
		}
		dur := f.Duration()
		p("| %s | %d | %d ms | %.2f s | %.2f |\n", s, len([]rune(s)), el.Milliseconds(), dur.Seconds(), el.Seconds()/dur.Seconds())
		wav := audio.EncodeSamples(audio.Resample(samples, f.SampleRate, 16000), 16000)
		clips = append(clips, clip{name: "TTS: " + s, ref: s, wav: wav, dur: dur})
	}
	p("\nRTF < 1 betyder hurtigere end realtid.\n\n")

	// Egne optagelser: <root>/bench/*.wav (16 kHz mono) med samme navn .txt som facit.
	own, err := loadClips(filepath.Join(cfg.Root, "bench"))
	if err != nil {
		p("⚠️ bench/: %v\n\n", err)
	}
	clips = append(own, clips...)

	// ---------- STT ----------
	models, _ := filepath.Glob(filepath.Join(cfg.ModelsDir, "stt", "ggml-*.bin"))
	sort.Strings(models)
	if len(models) == 0 {
		p("Ingen Whisper-modeller fundet.\n")
	}
	silence := audio.EncodeSamples(make([]int16, 16000), 16000)

	p("## STT – whisper.cpp\n\n")
	p("Lyd: %d klip (%d egne optagelser i `bench/`, resten er syntetisk TTS-tale).\n", len(clips), len(own))
	p("Hvert klip køres %d gange; tabellen viser gennemsnit.\n\n", sttRuns)
	type row struct {
		model           string
		load, wall, cpu time.Duration
		peak            uint64
		audioDur        time.Duration
		werSum          float64
		werN            int
	}
	var rows []row
	var samples strings.Builder
	for _, m := range models {
		name := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(m), "ggml-"), ".bin")
		eng, err := stt.NewWhisperCLI(cfg.WhisperPath, m, cfg.STTThreads, cfg.STTArgs)
		if err != nil {
			p("⚠️ %s: %v\n", name, err)
			continue
		}
		r := row{model: name}
		// ~Model-load: 1 s stilhed.
		if _, u, err := eng.TranscribeMeasured(ctx, silence, "da"); err == nil {
			r.load = u.Wall
		} else {
			p("⚠️ %s: %v\n", name, err)
			continue
		}
		fmt.Fprintf(out, "  %s ...\n", name)
		for _, c := range clips {
			var text string
			for i := 0; i < sttRuns; i++ {
				t, u, err := eng.TranscribeMeasured(ctx, c.wav, "da")
				if err != nil {
					return "", fmt.Errorf("stt %s: %w", name, err)
				}
				r.wall += u.Wall
				r.cpu += u.CPU
				r.peak = max(r.peak, u.PeakRAM)
				r.audioDur += c.dur
				text = t.Text
			}
			if c.ref != "" {
				r.werSum += WER(c.ref, text)
				r.werN++
			}
			fmt.Fprintf(&samples, "| %s | %s | %s |\n", name, c.name, text)
		}
		rows = append(rows, r)
	}
	if len(rows) > 0 {
		p("| Model | Model-load* | Tid pr. klip | CPU pr. klip | Peak RAM | RTF | WER |\n|---|---:|---:|---:|---:|---:|---:|\n")
		for _, r := range rows {
			n := time.Duration(len(clips) * sttRuns)
			wer := "–"
			if r.werN > 0 {
				wer = fmt.Sprintf("%.0f %%", 100*r.werSum/float64(r.werN))
			}
			p("| %s | %d ms | %d ms | %d ms | %s | %.2f | %s |\n", r.model, r.load.Milliseconds(),
				(r.wall / n).Milliseconds(), (r.cpu / n).Milliseconds(), mb(r.peak),
				r.wall.Seconds()/r.audioDur.Seconds(), wer)
		}
		p("\n\\* Model-load ≈ tid for 1 s stilhed (start + indlæsning + minimal afkodning).\n")
		p("RTF = behandlingstid / lydlængde. WER = ordfejlrate (lavere er bedre).\n\n")
		p("### Transskriptioner\n\n| Model | Klip | Resultat |\n|---|---|---|\n%s\n", samples.String())
	}

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	p("OpenLST-processen selv: %s RAM fra Go-runtime.\n\n", mb(ms.Sys))
	p("Tip: Læg egne optagelser i `bench/` (16 kHz mono WAV + .txt med samme navn) for et realistisk STT-resultat.\n")

	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dataDir, "bench-"+time.Now().Format("20060102-150405")+".md")
	return path, os.WriteFile(path, md.Bytes(), 0o600)
}

// piperCold starter Piper fra bunden og læser én sætning, målt som én proces.
func piperCold(ctx context.Context, cfg *config.Config) (procutil.Usage, error) {
	if cfg.PiperPath == "" {
		return procutil.Usage{}, tts.ErrUnavailable
	}
	tmp, err := os.MkdirTemp("", "openlst-bench-")
	if err != nil {
		return procutil.Usage{}, err
	}
	defer os.RemoveAll(tmp)
	dir := filepath.Dir(cfg.PiperPath)
	cmd := exec.CommandContext(ctx, cfg.PiperPath,
		"-m", procutil.RelArg(dir, cfg.VoicePath()),
		"--output_dir", procutil.RelArg(dir, tmp))
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(sentences[0] + "\n")
	procutil.Hide(cmd)
	return procutil.RunMeasured(cmd)
}

func loadClips(dir string) ([]clip, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.wav"))
	if err != nil || len(files) == 0 {
		return nil, nil
	}
	sort.Strings(files)
	var out []clip
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return out, err
		}
		s, fm, err := audio.Samples(b)
		if err != nil {
			return out, fmt.Errorf("%s: %w (skal være 16-bit mono WAV)", filepath.Base(f), err)
		}
		ref, _ := os.ReadFile(strings.TrimSuffix(f, ".wav") + ".txt")
		out = append(out, clip{
			name: filepath.Base(f),
			ref:  strings.TrimSpace(string(ref)),
			wav:  audio.EncodeSamples(audio.Resample(s, fm.SampleRate, 16000), 16000),
			dur:  fm.Duration(),
		})
	}
	return out, nil
}

func mb(b uint64) string {
	if b == 0 {
		return "–"
	}
	return fmt.Sprintf("%.0f MB", float64(b)/(1<<20))
}

func cpuName() string {
	if n := platformCPUName(); n != "" {
		return n
	}
	switch runtime.GOOS {
	case "windows":
		if v := os.Getenv("PROCESSOR_IDENTIFIER"); v != "" {
			return v
		}
	case "darwin":
		if b, err := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output(); err == nil {
			return strings.TrimSpace(string(b))
		}
	}
	return "ukendt"
}
