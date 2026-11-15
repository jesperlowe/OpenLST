// Package install installerer og afinstallerer OpenLST for den aktuelle
// bruger – uden administratorrettigheder (beslutning #011).
//
// Alt skrives under brugerens egen profil:
//
//	%LOCALAPPDATA%\Programs\OpenLST        programfiler
//	%LOCALAPPDATA%\OpenLST                 config, log, benchmark
//	Start-menu\Programs\OpenLST.lnk        genvej
//	Start-menu\Programs\Startup\OpenLST.lnk autostart (--background)
//	HKCU\...\Uninstall\OpenLST             post i "Installerede apps"
package install

const (
	AppName   = "OpenLST"
	Publisher = "WorkingOnIT"
)
