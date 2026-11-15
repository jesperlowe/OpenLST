//go:build windows

// Command setup er OpenLST-Setup.exe: installerer OpenLST for den aktuelle
// bruger uden administratorrettigheder (beslutning #011).
//
// Programfilerne ligger som zip i payload/OpenLST.zip og bygges af
// scripts/build-windows.sh.
package main

import (
	"bufio"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"openlst/backend/install"
)

//go:embed all:payload
var payload embed.FS

var version = "dev" // sættes ved build

func main() {
	fmt.Printf("OpenLST %s – installation\n", version)
	fmt.Println("Udgiver: WorkingOnIT")
	fmt.Println("Installeres kun for dig. Kræver ikke administrator.")
	fmt.Println()
	if err := run(); err != nil {
		fmt.Println()
		fmt.Println("❌ Installationen fejlede:", err)
		pause()
		os.Exit(1)
	}
	pause()
}

func run() error {
	data, err := payload.ReadFile("payload/OpenLST.zip")
	if err != nil {
		return fmt.Errorf("installationspakken mangler (byg med scripts/build-windows.sh)")
	}
	dir := install.Dir()

	step("Lukker OpenLST, hvis det kører")
	install.StopRunning()

	step("Fjerner tidligere version")
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("kunne ikke fjerne %s: %w", dir, err)
	}

	step("Kopierer filer til " + dir)
	size, err := install.Extract(data, dir)
	if err != nil {
		return err
	}

	step("Opretter genveje i Start-menuen og autostart")
	if err := install.CreateShortcuts(dir); err != nil {
		fmt.Println("   ⚠️", err)
	}

	step("Registrerer i \"Installerede apps\"")
	if err := install.Register(dir, version, size); err != nil {
		fmt.Println("   ⚠️", err)
	}

	if missing := install.MissingVCRuntime(); len(missing) > 0 {
		fmt.Println()
		fmt.Println("   ⚠️  Microsoft Visual C++ Runtime mangler:", strings.Join(missing, ", "))
		fmt.Println("      Oplæsning og diktering virker ikke uden den. Den kræver administrator at")
		fmt.Println("      installere: bed IT om \"Microsoft Visual C++ Redistributable 2015–2022 (x64)\".")
		fmt.Println("      (Kun testversionen har dette krav – den endelige version gør ikke.)")
		fmt.Println()
	}

	step("Starter OpenLST")
	if err := exec.Command(filepath.Join(dir, "openlst.exe")).Start(); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("✅ OpenLST er installeret og åbner nu i din browser (http://localhost:8765).")
	fmt.Println("   Du finder OpenLST i Start-menuen. Det starter selv, når du logger på.")
	fmt.Println()
	fmt.Println("Browser-udvidelse (læs op og diktér på hjemmesider):")
	fmt.Println("  1. Åbn edge://extensions (eller chrome://extensions)")
	fmt.Println("  2. Slå \"Udviklertilstand\" til")
	fmt.Println("  3. Klik \"Indlæs udpakket\" og vælg mappen:")
	fmt.Println("     " + filepath.Join(dir, "extension"))
	fmt.Println()
	fmt.Println("Benchmark (til test): kør " + filepath.Join(dir, "Benchmark.cmd"))
	fmt.Println("Afinstallér: Indstillinger → Apps → Installerede apps → OpenLST")
	return nil
}

func step(s string) { fmt.Println("• " + s) }

func pause() {
	fmt.Println()
	fmt.Print("Tryk Enter for at lukke …")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}
