// Package server er OpenLST's lokale HTTP-service.
package server

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"openlst/backend/audio"
	"openlst/backend/stt"
	"openlst/backend/tts"
)

const (
	maxTTSBody  = 20 << 10 // JSON med én sætning
	maxTTSChars = 2000
	maxSTTBody  = 4 << 20 // ~2 min ved 16 kHz mono 16-bit
	maxSTTAudio = 90 * time.Second
	sttRate     = 16000
)

// Version sættes ved build (-ldflags "-X openlst/backend/server.Version=...").
var Version = "dev"

type Server struct {
	TTS      tts.Engine // kan være nil
	STT      stt.Engine // kan være nil
	Frontend fs.FS
	Security *Security
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", s.status)
	mux.HandleFunc("POST /api/tts", s.tts)
	mux.HandleFunc("POST /api/stt", s.stt)
	mux.Handle("GET /", http.FileServerFS(s.Frontend))
	return s.Security.Wrap(mux)
}

type statusResponse struct {
	Status  string `json:"status"`
	TTS     bool   `json:"tts"`
	STT     bool   `json:"stt"`
	App     string `json:"app"`
	Version string `json:"version"`
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	ttsOK := s.TTS != nil && s.TTS.Ready()
	sttOK := s.STT != nil && s.STT.Ready()
	st := "ready"
	if !ttsOK || !sttOK {
		st = "degraded"
	}
	writeJSON(w, http.StatusOK, statusResponse{Status: st, TTS: ttsOK, STT: sttOK, App: "OpenLST", Version: Version})
}

type ttsRequest struct {
	Text     string  `json:"text"`
	Speed    float64 `json:"speed"`
	Language string  `json:"language"`
}

func (s *Server) tts(w http.ResponseWriter, r *http.Request) {
	if s.TTS == nil {
		apiError(w, http.StatusServiceUnavailable, "Oplæsning er ikke installeret.")
		return
	}
	if !hasContentType(r, "application/json") {
		apiError(w, http.StatusUnsupportedMediaType, "Forventede JSON.")
		return
	}
	var req ttsRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxTTSBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		apiError(w, http.StatusBadRequest, "Ugyldig forespørgsel.")
		return
	}
	if req.Language != "" && req.Language != "da" {
		apiError(w, http.StatusBadRequest, "Kun dansk er understøttet.")
		return
	}
	text := strings.TrimSpace(req.Text)
	n := utf8.RuneCountInString(text)
	if n == 0 || n > maxTTSChars {
		apiError(w, http.StatusBadRequest, "Teksten er tom eller for lang.")
		return
	}

	start := time.Now()
	res, err := s.TTS.Synthesize(r.Context(), tts.Request{Text: text, Speed: req.Speed})
	if err != nil {
		if r.Context().Err() == nil {
			log.Printf("tts: fejl: %v", err)
			apiError(w, http.StatusInternalServerError, "Oplæsningen fejlede.")
		}
		return
	}
	// Log aldrig selve teksten – kun længde og tid.
	log.Printf("tts: %d tegn på %d ms", n, time.Since(start).Milliseconds())
	w.Header().Set("Content-Type", "audio/wav")
	_, _ = w.Write(res.WAV)
}

func (s *Server) stt(w http.ResponseWriter, r *http.Request) {
	if s.STT == nil {
		apiError(w, http.StatusServiceUnavailable, "Diktering er ikke installeret.")
		return
	}
	if !hasContentType(r, "audio/wav", "audio/wave", "audio/x-wav") {
		apiError(w, http.StatusUnsupportedMediaType, "Forventede WAV-lyd.")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxSTTBody))
	if err != nil {
		apiError(w, http.StatusRequestEntityTooLarge, "Optagelsen er for lang.")
		return
	}
	f, err := audio.ParseWAV(body)
	if err != nil || f.SampleRate != sttRate || f.Channels != 1 || f.BitsPerSample != 16 {
		apiError(w, http.StatusBadRequest, "Lyden skal være 16 kHz mono WAV.")
		return
	}
	dur := f.Duration()
	if dur > maxSTTAudio {
		apiError(w, http.StatusRequestEntityTooLarge, "Optagelsen er for lang.")
		return
	}
	if dur < 300*time.Millisecond {
		writeJSON(w, http.StatusOK, map[string]string{"text": "", "language": "da"})
		return
	}

	start := time.Now()
	t, err := s.STT.Transcribe(r.Context(), body, "da")
	if err != nil {
		if r.Context().Err() == nil {
			log.Printf("stt: fejl: %v", err)
			apiError(w, http.StatusInternalServerError, "Dikteringen fejlede.")
		}
		return
	}
	el := time.Since(start)
	log.Printf("stt: %.1f s lyd på %d ms (RTF %.2f)", dur.Seconds(), el.Milliseconds(), el.Seconds()/dur.Seconds())
	writeJSON(w, http.StatusOK, map[string]string{"text": t.Text, "language": t.Language})
}

func hasContentType(r *http.Request, allowed ...string) bool {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return false
	}
	for _, a := range allowed {
		if mt == a {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func apiError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// IsOpenLST tjekker om svaret fra /api/status kommer fra OpenLST.
func IsOpenLST(body []byte) bool {
	var st statusResponse
	return json.Unmarshal(body, &st) == nil && st.App == "OpenLST"
}

var ErrPortBusy = errors.New("porten bruges af et andet program")
