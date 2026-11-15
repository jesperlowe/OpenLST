package openlst_test

// Privatlivstest: koden må ikke indeholde adresser uden for denne computer.
// Fejler testen, er der tilføjet et eksternt kald, en CDN eller lignende.

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var urlRe = regexp.MustCompile(`(?i)\b(?:https?|wss?)://[^\s"'` + "`" + `)<>]+`)

func allowed(u string) bool {
	host := strings.ToLower(u)
	host = host[strings.Index(host, "://")+3:]
	return strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.0.0.1") ||
		strings.HasPrefix(host, "evil.example") // bruges kun i tests
}

func TestNoExternalURLs(t *testing.T) {
	for _, dir := range []string{"backend", "frontend", "extension"} {
		err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			switch filepath.Ext(p) {
			case ".go", ".js", ".html", ".css":
			default:
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			for _, u := range urlRe.FindAllString(string(b), -1) {
				if !allowed(u) {
					t.Errorf("%s: ekstern adresse %s", p, u)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
