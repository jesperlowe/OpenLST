// Package frontend indlejrer brugerfladen i openlst.exe.
package frontend

import "embed"

//go:embed index.html app.js sentences.js wav.js recorder-worklet.js styles.css
var Files embed.FS
