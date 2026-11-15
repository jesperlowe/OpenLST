//go:build windows

package install

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"openlst/backend/procutil"
)

const uninstallKey = `Software\Microsoft\Windows\CurrentVersion\Uninstall\OpenLST`

// Dir er installationsmappen for den aktuelle bruger.
func Dir() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", AppName)
}

func startMenuDir() string {
	return filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Start Menu\Programs`)
}

func shortcutPaths() (startMenu, startup string) {
	return filepath.Join(startMenuDir(), AppName+".lnk"),
		filepath.Join(startMenuDir(), "Startup", AppName+".lnk")
}

// StopRunning lukker kørende OpenLST-processer for denne bruger, undtagen os selv.
func StopRunning() {
	cmd := exec.Command("taskkill", "/F", "/IM", "openlst.exe", "/FI", "PID ne "+strconv.Itoa(os.Getpid()))
	procutil.Hide(cmd)
	_ = cmd.Run()
	time.Sleep(700 * time.Millisecond) // giv Windows tid til at frigive filerne
}

// Extract pakker zip-data ud i dir. Beskytter mod stier uden for dir.
func Extract(data []byte, dir string) (int64, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return 0, err
	}
	var total int64
	root := filepath.Clean(dir) + string(os.PathSeparator)
	for _, f := range zr.File {
		// Pakken har topmappen "OpenLST/"; den fjernes.
		name := strings.TrimPrefix(filepath.ToSlash(f.Name), AppName+"/")
		if name == "" {
			continue
		}
		dst := filepath.Join(dir, filepath.FromSlash(name))
		if !strings.HasPrefix(dst, root) {
			return total, fmt.Errorf("ugyldig sti i pakken: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return total, err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return total, err
		}
		n, err := writeFile(f, dst)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func writeFile(f *zip.File, dst string) (int64, error) {
	rc, err := f.Open()
	if err != nil {
		return 0, err
	}
	defer rc.Close()
	out, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(out, rc)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return n, err
}

// CreateShortcuts laver Start-menu- og autostart-genveje via Windows' egen
// WScript.Shell. Stierne gives som miljøvariabler, så citationstegn og
// specialtegn i stier ikke kan bryde scriptet.
func CreateShortcuts(dir string) error {
	startMenu, startup := shortcutPaths()
	exe := filepath.Join(dir, "openlst.exe")
	script := `$s = New-Object -ComObject WScript.Shell
function Make($path, $arguments) {
  $l = $s.CreateShortcut($path)
  $l.TargetPath = $env:OPENLST_EXE
  $l.Arguments = $arguments
  $l.WorkingDirectory = $env:OPENLST_DIR
  $l.IconLocation = $env:OPENLST_EXE + ',0'
  $l.Description = 'OpenLST – læse- og skrivestøtte'
  $l.Save()
}
Make $env:OPENLST_STARTMENU ''
Make $env:OPENLST_STARTUP '--background'`
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.Env = append(os.Environ(),
		"OPENLST_EXE="+exe, "OPENLST_DIR="+dir,
		"OPENLST_STARTMENU="+startMenu, "OPENLST_STARTUP="+startup)
	procutil.Hide(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("genveje: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Register tilføjer OpenLST til "Installerede apps" for den aktuelle bruger.
func Register(dir, version string, sizeBytes int64) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, uninstallKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	exe := filepath.Join(dir, "openlst.exe")
	for name, v := range map[string]string{
		"DisplayName":     AppName,
		"DisplayVersion":  version,
		"Publisher":       Publisher,
		"InstallLocation": dir,
		"DisplayIcon":     exe,
		"UninstallString": `"` + exe + `" --uninstall`,
	} {
		if err := k.SetStringValue(name, v); err != nil {
			return err
		}
	}
	_ = k.SetDWordValue("NoModify", 1)
	_ = k.SetDWordValue("NoRepair", 1)
	_ = k.SetDWordValue("EstimatedSize", uint32(sizeBytes/1024))
	return nil
}

// Uninstall fjerner OpenLST for den aktuelle bruger. Køres fra openlst.exe.
func Uninstall(dataDir string) error {
	if !confirm("Vil du afinstallere OpenLST?") {
		return nil
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	dir := filepath.Dir(self)
	StopRunning()

	startMenu, startup := shortcutPaths()
	_ = os.Remove(startMenu)
	_ = os.Remove(startup)
	_ = registry.DeleteKey(registry.CURRENT_USER, uninstallKey)
	_ = os.RemoveAll(dataDir) // config, log og benchmark – intet efterlades

	// Programmappen kan først slettes, når denne proces er afsluttet.
	// Sikkerhedstjek: slet kun en mappe, der ligner vores installation.
	if strings.EqualFold(filepath.Base(dir), AppName) && fileExists(filepath.Join(dir, "openlst.exe")) &&
		fileExists(filepath.Join(dir, "runtime")) {
		cmd := exec.Command(filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe"))
		cmd.Dir = os.TempDir() // må ikke stå i den mappe, der skal slettes
		procutil.Hide(cmd)
		cmd.SysProcAttr.CreationFlags |= windows.DETACHED_PROCESS
		// cmd.exe forstår ikke Go's \"-escaping, så kommandolinjen sættes rå.
		cmd.SysProcAttr.CmdLine = `cmd.exe /C "ping 127.0.0.1 -n 3 >NUL & rmdir /S /Q "` + dir + `""`
		if err := cmd.Start(); err != nil {
			return err
		}
	} else {
		return errors.New("programmappen blev ikke genkendt og er ikke slettet: " + dir)
	}
	message("OpenLST er afinstalleret.")
	return nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func confirm(text string) bool {
	t, _ := windows.UTF16PtrFromString(text)
	c, _ := windows.UTF16PtrFromString(AppName)
	r, _ := windows.MessageBox(0, t, c, windows.MB_YESNO|windows.MB_ICONQUESTION)
	return r == 6 // IDYES
}

func message(text string) {
	t, _ := windows.UTF16PtrFromString(text)
	c, _ := windows.UTF16PtrFromString(AppName)
	windows.MessageBox(0, t, c, windows.MB_OK|windows.MB_ICONINFORMATION)
}

// MissingVCRuntime returnerer de Visual C++-runtime-filer, som piper.exe og
// whisper-cli.exe kræver, men som mangler i Windows. Testbuilds bruger
// færdigbyggede binærer, der afhænger af dem; release-builds skal bygges med
// statisk runtime (/MT), så kravet forsvinder (beslutning #034).
func MissingVCRuntime() []string {
	sys := filepath.Join(os.Getenv("SystemRoot"), "System32")
	var missing []string
	for _, dll := range []string{"msvcp140.dll", "vcruntime140.dll", "vcruntime140_1.dll", "vcomp140.dll"} {
		if !fileExists(filepath.Join(sys, dll)) {
			missing = append(missing, dll)
		}
	}
	return missing
}
