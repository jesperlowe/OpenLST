// Command openlst starter OpenLST's lokale service på http://localhost:8765.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"openlst/backend/bench"
	"openlst/backend/browser"
	"openlst/backend/config"
	"openlst/backend/install"
	"openlst/backend/server"
	"openlst/backend/stt"
	"openlst/backend/tts"
	"openlst/frontend"
)

func main() {
	root := flag.String("root", "", "installationsmappe med models/ og runtime/ (standard: exe-mappen)")
	open := flag.Bool("open", true, "åbn browseren når servicen er klar")
	background := flag.Bool("background", false, "start uden at åbne browseren (autostart)")
	runBench := flag.Bool("bench", false, "kør udvikler-benchmark og gem resultatet")
	uninstall := flag.Bool("uninstall", false, "afinstallér OpenLST (Windows)")
	flag.Parse()
	if *background {
		*open = false
	}

	dataDir, err := config.DataDir()
	if err != nil {
		log.Fatalf("datamappe: %v", err)
	}
	if *uninstall {
		if err := install.Uninstall(dataDir); err != nil {
			fatal("Afinstallation fejlede: %v", err)
		}
		return
	}
	closeLog := setupLog(dataDir)
	defer closeLog()

	cfg, err := config.Load(*root, dataDir)
	if err != nil {
		fatal("Konfigurationen kunne ikke læses: %v", err)
	}
	if missing := install.MissingVCRuntime(); len(missing) > 0 {
		log.Printf("ADVARSEL: Visual C++ Runtime mangler (%v) – piper/whisper kan ikke starte", missing)
	}
	if *runBench {
		path, err := bench.Run(context.Background(), cfg, dataDir, os.Stdout)
		if err != nil {
			fatal("Benchmark fejlede: %v", err)
		}
		showFile(path)
		return
	}
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(cfg.Port))
	url := fmt.Sprintf("http://localhost:%d/", cfg.Port)

	// Kører OpenLST allerede? Så åbn blot browseren.
	if running, err := alreadyRunning(cfg.Port); running {
		log.Printf("OpenLST kører allerede på %s", url)
		if *open {
			_ = browser.Open(url)
		}
		return
	} else if err != nil {
		fatal("OpenLST kan ikke starte: %v", err)
	}

	// Lyt kun på loopback. Aldrig 0.0.0.0.
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fatal("OpenLST kan ikke starte på %s: %v", addr, err)
	}

	srv := &server.Server{
		Frontend: frontend.Files,
		Security: server.NewSecurity(cfg.Port, cfg.ExtensionIDs),
	}

	var piper *tts.Piper
	if p, err := tts.NewPiper(cfg.PiperPath, cfg.VoicePath()); err != nil {
		log.Printf("tts: %v", err)
	} else {
		piper = p
		srv.TTS = p
		go p.Warm()
	}
	if w, err := stt.NewWhisperCLI(cfg.WhisperPath, cfg.STTModelPath(), cfg.STTThreads, cfg.STTArgs); err != nil {
		log.Printf("stt: %v", err)
	} else {
		srv.STT = w
	}

	hs := &http.Server{
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      180 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("OpenLST %s klar på %s (tts=%v, stt=%v)", server.Version, url, srv.TTS != nil, srv.STT != nil)
		if err := hs.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()
	if *open {
		if err := browser.Open(url); err != nil {
			log.Printf("kunne ikke åbne browser: %v", err)
		}
	}

	<-ctx.Done()
	log.Printf("lukker ned")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = hs.Shutdown(shutdownCtx)
	if piper != nil {
		_ = piper.Close()
	}
}

// alreadyRunning svarer true, hvis OpenLST allerede lytter på porten.
// Svarer et andet program, returneres en fejl – vi overtager ikke porten.
func alreadyRunning(port int) (bool, error) {
	c := &http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := c.Get(fmt.Sprintf("http://127.0.0.1:%d/api/status", port))
	if err != nil {
		return false, nil // ingen lytter
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if server.IsOpenLST(body) {
		return true, nil
	}
	return false, fmt.Errorf("port %d: %w", port, server.ErrPortBusy)
}
