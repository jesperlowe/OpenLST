package tts

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"openlst/backend/procutil"
)

// Piper kører Piper som en langlivet proces, så stemmemodellen kun indlæses
// én gang. Protokol (fælles for piper.exe og Python-udgaven):
//
//	piper -m <model> --output_dir <dir> --length_scale <ls>
//
// Hver tekstlinje på stdin bliver til én WAV-fil i <dir>, og stien skrives på
// stdout (piper.exe) eller stderr (Python). Vi læser begge og venter på stien.
//
// length_scale er et procesargument, så en ændret hastighed genstarter
// processen. Hastigheden ændres sjældent (skyderen har trin på 0.1).
type Piper struct {
	bin, model string

	mu      sync.Mutex
	proc    *piperProc
	ls      float64
	outDir  string
	timeout time.Duration
	broken  bool // seneste opstart fejlede
}

type piperProc struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
	paths chan string
	done  chan struct{}
}

// wavNameRe finder filnavnet i en linje som "C:\\...\\1700000000.wav" eller
// "INFO:__main__:Wrote /tmp/.../1700000000.wav".
var wavNameRe = regexp.MustCompile(`[\\/]([0-9A-Za-z_.-]+\.wav)\s*$`)

// NewPiper opretter motoren. Processen startes ved første kald eller via Warm.
func NewPiper(bin, model string) (*Piper, error) {
	if bin == "" {
		return nil, fmt.Errorf("%w: piper blev ikke fundet", ErrUnavailable)
	}
	if _, err := os.Stat(model); err != nil {
		return nil, fmt.Errorf("%w: stemme mangler: %s", ErrUnavailable, filepath.Base(model))
	}
	if _, err := os.Stat(model + ".json"); err != nil {
		return nil, fmt.Errorf("%w: stemmens .onnx.json mangler", ErrUnavailable)
	}
	dir, err := os.MkdirTemp("", "openlst-tts-")
	if err != nil {
		return nil, err
	}
	return &Piper{bin: bin, model: model, outDir: dir, timeout: 60 * time.Second}, nil
}

// Ready er falsk, hvis Piper ikke kunne starte (fx manglende DLL'er).
func (p *Piper) Ready() bool {
	if p == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return !p.broken
}

// Warm starter processen i baggrunden med standardhastighed, så første
// oplæsning ikke skal vente på at modellen indlæses.
func (p *Piper) Warm() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.ensure(lengthScale(DefaultSpeed)); err != nil {
		log.Printf("tts: opstart fejlede: %v", err)
		p.broken = true
		return
	}
	// Opdag hvis processen dør med det samme (fx STATUS_DLL_NOT_FOUND).
	select {
	case <-p.proc.done:
		log.Printf("tts: piper stoppede ved opstart (exit %v)", p.proc.cmd.ProcessState)
		p.broken = true
	case <-time.After(1500 * time.Millisecond):
	}
}

func (p *Piper) Synthesize(ctx context.Context, req Request) (Result, error) {
	text := sanitize(req.Text)
	if text == "" {
		return Result{}, errors.New("tom tekst")
	}
	ls := lengthScale(ClampSpeed(req.Speed))

	p.mu.Lock()
	defer p.mu.Unlock()

	// Ét genforsøg, hvis processen er død undervejs.
	for attempt := 0; ; attempt++ {
		if err := p.ensure(ls); err != nil {
			return Result{}, err
		}
		wav, err := p.roundTrip(ctx, text)
		if err == nil {
			p.broken = false
			return Result{WAV: wav}, nil
		}
		p.stopLocked()
		if attempt > 0 || ctx.Err() != nil {
			return Result{}, err
		}
		log.Printf("tts: genstarter piper efter fejl: %v", err)
	}
}

func (p *Piper) roundTrip(ctx context.Context, text string) ([]byte, error) {
	pr := p.proc
	// Smid gamle meddelelser væk, så svaret hører til denne forespørgsel.
	for len(pr.paths) > 0 {
		<-pr.paths
	}
	if _, err := io.WriteString(pr.stdin, text+"\n"); err != nil {
		return nil, fmt.Errorf("skriv til piper: %w", err)
	}
	timer := time.NewTimer(p.timeout)
	defer timer.Stop()
	select {
	case path := <-pr.paths:
		defer os.Remove(path) // midlertidig lyd slettes altid
		return os.ReadFile(path)
	case <-pr.done:
		return nil, errors.New("piper stoppede uventet")
	case <-timer.C:
		return nil, errors.New("piper svarede ikke")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ensure sørger for, at der kører en proces med den ønskede length_scale.
func (p *Piper) ensure(ls float64) error {
	if p.proc != nil && p.ls == ls {
		select {
		case <-p.proc.done:
		default:
			return nil
		}
	}
	p.stopLocked()

	// piper.exe finder espeak-ng-data ved siden af sig selv, så den køres fra
	// sin egen mappe. Stier gives relativt til den (se procutil.RelArg).
	dir := filepath.Dir(p.bin)
	cmd := exec.Command(p.bin,
		"-m", procutil.RelArg(dir, p.model),
		"--output_dir", procutil.RelArg(dir, p.outDir),
		"--length_scale", fmt.Sprintf("%.2f", ls),
	)
	cmd.Dir = dir
	procutil.Hide(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	procutil.KillWithParent(cmd)
	pr := &piperProc{cmd: cmd, stdin: stdin, paths: make(chan string, 8), done: make(chan struct{})}

	// Begge strømme kan melde stien; piper.exe kan melde den på begge.
	// recent dedupliker, så én fil ikke bliver til to svar.

	var recentMu sync.Mutex
	recent := map[string]bool{}
	scan := func(r io.Reader) {
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			// Kun filnavnet bruges; mappen er altid vores egen. Så betyder
			// tegnsæt og mellemrum i den udskrevne sti ikke noget.
			m := wavNameRe.FindStringSubmatch(sc.Text())
			if m == nil {
				continue
			}
			m[1] = filepath.Join(p.outDir, m[1])
			recentMu.Lock()
			dup := recent[m[1]]
			if len(recent) > 64 {
				clear(recent)
			}
			recent[m[1]] = true
			recentMu.Unlock()
			if !dup {
				select {
				case pr.paths <- m[1]:
				default:
				}
			}
		}
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); scan(stdout) }()
	go func() { defer wg.Done(); scan(stderr) }()
	go func() {
		wg.Wait()
		_ = cmd.Wait()
		close(pr.done)
	}()

	p.proc, p.ls = pr, ls
	return nil
}

func (p *Piper) stopLocked() {
	if p.proc == nil {
		return
	}
	_ = p.proc.stdin.Close()
	select {
	case <-p.proc.done:
	case <-time.After(2 * time.Second):
		_ = p.proc.cmd.Process.Kill()
		<-p.proc.done
	}
	p.proc = nil
	// Ryd eventuelle efterladte filer.
	if ents, err := os.ReadDir(p.outDir); err == nil {
		for _, e := range ents {
			_ = os.Remove(filepath.Join(p.outDir, e.Name()))
		}
	}
}

func (p *Piper) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopLocked()
	return os.RemoveAll(p.outDir)
}

// lengthScale omsætter hastighed til Piper's length_scale (større = langsommere).
func lengthScale(speed float64) float64 {
	return math.Round(100/speed) / 100
}

// sanitize gør teksten til én linje, da Piper læser linje for linje.
func sanitize(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r == ' ' || r == ' ' {
			return ' '
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}
