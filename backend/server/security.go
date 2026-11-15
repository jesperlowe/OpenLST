package server

import (
	"net/http"
	"strconv"
	"strings"
)

// Security beskytter API'et mod kald fra vilkårlige hjemmesider.
//
// Trusselsmodel: enhver side i brugerens browser kan sende requests til
// 127.0.0.1. Derfor:
//   - Host-headeren skal være vores egen (beskytter mod DNS-rebinding).
//   - /api/* kræver en Origin fra allowlisten: egen UI og kendte
//     extension-ID'er; aldrig wildcard. Browseren sætter Origin selv, og
//     hverken hjemmesider eller andre extensions kan forfalske den.
//   - CORS-headere sendes kun til extensions på allowlisten.
type Security struct {
	hosts      map[string]bool
	selfOrigin map[string]bool
	extOrigin  map[string]bool
}

func NewSecurity(port int, extensionIDs []string) *Security {
	p := strconv.Itoa(port)
	s := &Security{
		hosts: map[string]bool{
			"localhost:" + p: true,
			"127.0.0.1:" + p: true,
		},
		selfOrigin: map[string]bool{
			"http://localhost:" + p: true,
			"http://127.0.0.1:" + p: true,
		},
		extOrigin: map[string]bool{},
	}
	for _, id := range extensionIDs {
		id = strings.TrimSpace(id)
		if id == "" || strings.ContainsAny(id, "*/:") {
			continue // kun rene ID'er, aldrig mønstre
		}
		s.extOrigin["chrome-extension://"+id] = true
	}
	return s
}

func (s *Security) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.hosts[strings.ToLower(r.Host)] {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		setCommonHeaders(w)

		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		origin := r.Header.Get("Origin")
		switch {
		case s.selfOrigin[origin]:
			// Egen brugerflade.
		case s.extOrigin[origin]:
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
				w.Header().Set("Access-Control-Max-Age", "600")
				if r.Header.Get("Access-Control-Request-Private-Network") == "true" {
					w.Header().Set("Access-Control-Allow-Private-Network", "true")
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
		case origin == "" && r.Method == http.MethodGet && r.URL.Path == "/api/status":
			// Diagnostik uden browser (fx curl). Afslører kun status.
		default:
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func setCommonHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Cross-Origin-Resource-Policy", "same-origin")
	h.Set("Cache-Control", "no-store")
	// Siden må kun hente fra sig selv. Ingen CDN, ingen eksterne kald.
	h.Set("Content-Security-Policy",
		"default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; "+
			"media-src 'self' blob:; connect-src 'self'; worker-src 'self'; "+
			"frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
	h.Set("Permissions-Policy", "microphone=(self), camera=(), geolocation=()")
}
