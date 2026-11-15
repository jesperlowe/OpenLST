//go:build windows

package main

import (
	"os/exec"

	"golang.org/x/sys/windows"
)

// showError viser en fejlbesked i en dialogboks (exe'en har ingen konsol).
func showError(msg string) {
	title, _ := windows.UTF16PtrFromString("OpenLST")
	text, _ := windows.UTF16PtrFromString(msg)
	windows.MessageBox(0, text, title, windows.MB_OK|windows.MB_ICONERROR)
}

// showFile åbner en tekstfil i Notesblok.
func showFile(path string) {
	_ = exec.Command("notepad.exe", path).Start()
}
