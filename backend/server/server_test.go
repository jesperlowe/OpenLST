package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"openlst/backend/audio"
	"openlst/backend/stt"
	"openlst/backend/tts"
)

type fakeTTS struct{ got tts.Request }

func (f *fakeTTS) Synthesize(_ context.Context, r tts.Request) (tts.Result, error) {
	f.got = r
	return tts.Result{WAV: audio.EncodeWAV(make([]byte, 100), 22050, 1)}, nil
}
func (f *fakeTTS) Ready() bool  { return true }
func (f *fakeTTS) Close() error { return nil }

type fakeSTT struct{}

func (fakeSTT) Transcribe(context.Context, []byte, string) (stt.Transcript, error) {
	return stt.Transcript{Text: "Jeg cykler i skole i morgen.", Language: "da"}, nil
}
func (fakeSTT) Ready() bool { return true }

func newTestServer() (*Server, *fakeTTS) {
	ft := &fakeTTS{}
	return &Server{
		TTS: ft, STT: fakeSTT{},
		Frontend: fstest.MapFS{"index.html": {Data: []byte("ok")}},
		Security: NewSecurity(8765, nil),
	}, ft
}

func do(h http.Handler, method, path, ctype string, body []byte) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, bytes.NewReader(body))
	r.Host = "localhost:8765"
	r.Header.Set("Origin", "http://localhost:8765")
	if ctype != "" {
		r.Header.Set("Content-Type", ctype)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestTTS(t *testing.T) {
	s, ft := newTestServer()
	h := s.Handler()
	w := do(h, "POST", "/api/tts", "application/json", []byte(`{"text":"Hej.","speed":0.9,"language":"da"}`))
	if w.Code != 200 || w.Header().Get("Content-Type") != "audio/wav" {
		t.Fatalf("tts: %d %s", w.Code, w.Body)
	}
	if ft.got.Text != "Hej." || ft.got.Speed != 0.9 {
		t.Fatalf("forkert request: %+v", ft.got)
	}
	for _, body := range []string{`{"text":""}`, `{"text":"hej","language":"en"}`, `{"text":"hej","x":1}`, `nope`} {
		if w := do(h, "POST", "/api/tts", "application/json", []byte(body)); w.Code != 400 {
			t.Errorf("%s: %d", body, w.Code)
		}
	}
	if w := do(h, "POST", "/api/tts", "text/plain", []byte(`{"text":"hej"}`)); w.Code != 415 {
		t.Errorf("text/plain: %d", w.Code)
	}
}

func TestSTT(t *testing.T) {
	s, _ := newTestServer()
	h := s.Handler()
	oneSec := audio.EncodeWAV(make([]byte, 32000), 16000, 1)
	w := do(h, "POST", "/api/stt", "audio/wav", oneSec)
	var out map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if w.Code != 200 || out["text"] != "Jeg cykler i skole i morgen." || out["language"] != "da" {
		t.Fatalf("stt: %d %s", w.Code, w.Body)
	}
	if w := do(h, "POST", "/api/stt", "audio/wav", audio.EncodeWAV(make([]byte, 44100), 22050, 1)); w.Code != 400 {
		t.Errorf("22 kHz skal afvises: %d", w.Code)
	}
	if w := do(h, "POST", "/api/stt", "audio/wav", []byte("garbage")); w.Code != 400 {
		t.Errorf("garbage: %d", w.Code)
	}
	if w := do(h, "POST", "/api/stt", "application/octet-stream", oneSec); w.Code != 415 {
		t.Errorf("content-type: %d", w.Code)
	}
}

func TestStatus(t *testing.T) {
	s, _ := newTestServer()
	w := do(s.Handler(), "GET", "/api/status", "", nil)
	if w.Code != 200 || !IsOpenLST(w.Body.Bytes()) {
		t.Fatalf("status: %d %s", w.Code, w.Body)
	}
}
