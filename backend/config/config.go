// Package config finder OpenLST's filer og indstillinger.
//
// Stier løses relativt til installationsmappen (mappen med openlst.exe), aldrig
// til den aktuelle arbejdsmappe. En valgfri config.json i brugerens datamappe
// kan overskrive standardværdierne.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const DefaultPort = 8765

// Config er den samlede konfiguration.
type Config struct {
	// Port som servicen lytter på. Adressen er altid 127.0.0.1.
	Port int `json:"port"`

	// Root er installationsmappen; models/ og runtime/ ligger her.
	Root      string `json:"-"`
	ModelsDir string `json:"models_dir,omitempty"`

	// TTS
	PiperPath string `json:"piper_path,omitempty"`
	TTSVoice  string `json:"tts_voice,omitempty"` // filnavn uden .onnx

	// STT
	WhisperPath string   `json:"whisper_path,omitempty"`
	STTModel    string   `json:"stt_model,omitempty"` // filnavn, fx ggml-tiny-q5_1.bin
	STTThreads  int      `json:"stt_threads,omitempty"`
	STTArgs     []string `json:"stt_extra_args,omitempty"`

	// Ekstra browser-extensions der må kalde API'et (fx Edge Add-ons-ID'et).
	// Kun ID'er, aldrig wildcard. OpenLST's egen extension er altid med.
	ExtensionIDs []string `json:"extension_ids,omitempty"`
}

// ExtensionID er ID'et for OpenLST's egen browser-extension. Det er fastlagt
// af den offentlige nøgle i extension/manifest.json.
const ExtensionID = "bmhpeociclipbhackmagnbakkmipbhpn"

// DataDir er brugerens OpenLST-mappe (%LOCALAPPDATA%\OpenLST på Windows).
func DataDir() (string, error) {
	if runtime.GOOS == "windows" {
		if d := os.Getenv("LOCALAPPDATA"); d != "" {
			return filepath.Join(d, "OpenLST"), nil
		}
	}
	d, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "OpenLST"), nil
}

// Load bygger konfigurationen. root kan være tom; så findes den automatisk.
func Load(root, dataDir string) (*Config, error) {
	if root == "" {
		root = findRoot()
	}
	root, _ = filepath.Abs(root)
	c := &Config{Port: DefaultPort, Root: root}

	if dataDir != "" {
		b, err := os.ReadFile(filepath.Join(dataDir, "config.json"))
		switch {
		case err == nil:
			if err := json.Unmarshal(b, c); err != nil {
				return nil, fmt.Errorf("config.json: %w", err)
			}
		case !errors.Is(err, os.ErrNotExist):
			return nil, err
		}
	}

	c.applyDefaults()
	return c, nil
}

func (c *Config) applyDefaults() {
	if c.Port == 0 {
		c.Port = DefaultPort
	}
	c.ExtensionIDs = append([]string{ExtensionID}, c.ExtensionIDs...)
	if c.ModelsDir == "" {
		c.ModelsDir = filepath.Join(c.Root, "models")
	} else if !filepath.IsAbs(c.ModelsDir) {
		c.ModelsDir = filepath.Join(c.Root, c.ModelsDir)
	}
	if c.TTSVoice == "" {
		c.TTSVoice = "da_DK-talesyntese-medium"
	}
	if c.STTModel == "" {
		c.STTModel = "ggml-tiny-q5_1.bin"
	}
	if c.STTThreads == 0 {
		// Whisper skalerer med fysiske kerner; Surface Go har 2 (4 logiske).
		c.STTThreads = max(1, min(4, runtime.NumCPU()/2))
	}
	if c.PiperPath == "" {
		c.PiperPath = c.findBinary(
			filepath.Join("runtime", "piper", exe("piper")),
			filepath.Join("runtime", "piper-venv", "bin", "piper"), // kun udvikling på macOS
		)
	}
	if c.WhisperPath == "" {
		c.WhisperPath = c.findBinary(filepath.Join("runtime", "whisper", exe("whisper-cli")))
		if c.WhisperPath == "" && runtime.GOOS != "windows" {
			// Udvikling: brug whisper-cli fra PATH (fx Homebrew). På Windows slås
			// PATH-opslag fra for at undgå at et fremmed program kapres.
			if p, err := exec.LookPath("whisper-cli"); err == nil {
				c.WhisperPath = p
			}
		}
	}
}

// VoicePath returnerer stien til den valgte Piper-stemme.
func (c *Config) VoicePath() string {
	return filepath.Join(c.ModelsDir, "tts", c.TTSVoice+".onnx")
}

// STTModelPath returnerer stien til den valgte Whisper-model.
func (c *Config) STTModelPath() string {
	return filepath.Join(c.ModelsDir, "stt", c.STTModel)
}

func (c *Config) findBinary(candidates ...string) string {
	for _, rel := range candidates {
		p := filepath.Join(c.Root, rel)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

func exe(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// findRoot vælger mappen med exe-filen, hvis den indeholder models/.
// Ellers (fx under `go run`) bruges den aktuelle mappe.
func findRoot() string {
	if p, err := os.Executable(); err == nil {
		dir := filepath.Dir(p)
		if st, err := os.Stat(filepath.Join(dir, "models")); err == nil && st.IsDir() {
			return dir
		}
	}
	wd, _ := os.Getwd()
	return wd
}
