package openlst_test

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"openlst/backend/config"
)

// Fælles JavaScript skal være identisk i webappen og extensionen.
func TestExtensionLibsInSync(t *testing.T) {
	for _, f := range []string{"sentences.js", "wav.js", "recorder-worklet.js"} {
		a, err := os.ReadFile("frontend/" + f)
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile("extension/lib/" + f)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a, b) {
			t.Errorf("extension/lib/%s afviger fra frontend/%s – kør scripts/sync-extension.sh", f, f)
		}
	}
}

// Extensionen må kun have adgang til OpenLST på localhost, og dens ID skal
// passe med det ID, serveren tillader.
func TestExtensionManifest(t *testing.T) {
	b, err := os.ReadFile("extension/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Key             string   `json:"key"`
		HostPermissions []string `json:"host_permissions"`
		Permissions     []string `json:"permissions"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, h := range m.HostPermissions {
		if !strings.HasPrefix(h, "http://127.0.0.1:8765/") && !strings.HasPrefix(h, "http://localhost:8765/") {
			t.Errorf("host_permissions må kun være OpenLST lokalt: %s", h)
		}
	}
	for _, p := range m.Permissions {
		switch p {
		case "contextMenus", "storage", "offscreen":
		default:
			t.Errorf("uventet permission: %s", p)
		}
	}
	if id := extensionID(t, m.Key); id != config.ExtensionID {
		t.Errorf("manifest-nøglen giver ID %s, men serveren tillader %s", id, config.ExtensionID)
	}
}
