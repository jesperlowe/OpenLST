package procutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRelArgResolves(t *testing.T) {
	base := t.TempDir() // på macOS under /var → /private/var
	other := t.TempDir()
	target := filepath.Join(other, "model.bin")
	if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	rel := RelArg(base, target)
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	if err := os.Chdir(base); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(rel); err != nil {
		t.Fatalf("relativ sti %q virker ikke fra %s: %v", rel, base, err)
	}
}
