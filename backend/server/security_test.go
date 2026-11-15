package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurity(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	h := NewSecurity(8765, []string{"goodext", "*", "bad/id"}).Wrap(ok)

	cases := []struct {
		name, method, path, host, origin string
		want                             int
	}{
		{"egen UI POST", "POST", "/api/tts", "localhost:8765", "http://localhost:8765", 200},
		{"egen UI via 127.0.0.1", "POST", "/api/stt", "127.0.0.1:8765", "http://127.0.0.1:8765", 200},
		{"status uden origin", "GET", "/api/status", "127.0.0.1:8765", "", 200},
		{"forside uden origin", "GET", "/", "localhost:8765", "", 200},
		{"fremmed site", "POST", "/api/tts", "localhost:8765", "https://evil.example", 403},
		{"fremmed site status", "GET", "/api/status", "localhost:8765", "https://evil.example", 403},
		{"POST uden origin", "POST", "/api/tts", "localhost:8765", "", 403},
		{"null origin", "POST", "/api/tts", "localhost:8765", "null", 403},
		{"DNS-rebinding", "GET", "/", "evil.example:8765", "", 403},
		{"forkert port i origin", "POST", "/api/tts", "localhost:8765", "http://localhost:9999", 403},
		{"kendt extension", "POST", "/api/tts", "localhost:8765", "chrome-extension://goodext", 200},
		{"ukendt extension", "POST", "/api/tts", "localhost:8765", "chrome-extension://other", 403},
		{"wildcard-ID ignoreres", "POST", "/api/tts", "localhost:8765", "chrome-extension://*", 403},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest(c.method, c.path, nil)
			r.Host = c.host
			if c.origin != "" {
				r.Header.Set("Origin", c.origin)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != c.want {
				t.Fatalf("status %d, ville have %d", w.Code, c.want)
			}
			if acao := w.Header().Get("Access-Control-Allow-Origin"); acao == "*" {
				t.Fatal("wildcard-CORS må aldrig sendes")
			}
		})
	}
}

func TestCORSOnlyForExtensions(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	h := NewSecurity(8765, []string{"goodext"}).Wrap(ok)

	r := httptest.NewRequest("OPTIONS", "/api/tts", nil)
	r.Host = "localhost:8765"
	r.Header.Set("Origin", "chrome-extension://goodext")
	r.Header.Set("Access-Control-Request-Private-Network", "true")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 204 || w.Header().Get("Access-Control-Allow-Origin") != "chrome-extension://goodext" ||
		w.Header().Get("Access-Control-Allow-Private-Network") != "true" {
		t.Fatalf("preflight: %d %v", w.Code, w.Header())
	}

	r = httptest.NewRequest("OPTIONS", "/api/tts", nil)
	r.Host = "localhost:8765"
	r.Header.Set("Origin", "https://evil.example")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 || w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("fremmed preflight skal afvises: %d", w.Code)
	}
}

func TestCSPBlocksExternal(t *testing.T) {
	w := httptest.NewRecorder()
	setCommonHeaders(w)
	csp := w.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "connect-src 'self'") || strings.Contains(csp, "http") {
		t.Fatalf("CSP skal kun tillade egen adresse: %s", csp)
	}
}
